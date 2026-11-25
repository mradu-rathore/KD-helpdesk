package main

// KD: RMAs (returns). An RMA tracks one product line a customer sends back,
// from the request to a credit, replacement, repair or return. Every RMA hangs
// off a ticket - replies, internal notes and the owner live on the ticket - and
// one ticket can carry several RMAs (a box with different products in it).
//
// RMAs come from KD's RMA Google Form (POST /api/v1/kd/rmas/intake, called by
// the Apps Script in integrations/google-forms) or are raised by agents.

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/mail"
	"strconv"
	"strings"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/jmoiron/sqlx"
	"github.com/valyala/fasthttp"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/fastglue"
)

// kdRmaSchema creates the RMA tables. Runs after kdSchema (needs kd_customers).
const kdRmaSchema = `
CREATE TABLE IF NOT EXISTS kd_rmas (
	id SERIAL PRIMARY KEY,
	number TEXT GENERATED ALWAYS AS ('RMA-' || (1000 + id)::text) STORED,
	created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	status TEXT NOT NULL DEFAULT 'requested'
		CHECK (status IN ('requested', 'approved', 'received', 'testing', 'with_supplier', 'resolved', 'rejected')),
	outcome TEXT NOT NULL DEFAULT ''
		CHECK (outcome IN ('', 'replaced', 'repaired', 'credited', 'returned_nff')),
	source TEXT NOT NULL DEFAULT 'agent' CHECK (source IN ('agent', 'form')),
	-- Google Form response ID, so a re-sent submission never makes a second RMA
	form_response_id TEXT,
	customer_id INT REFERENCES kd_customers(id) ON DELETE SET NULL,
	contact_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
	conversation_id BIGINT REFERENCES conversations(id) ON DELETE SET NULL,
	brand TEXT NOT NULL DEFAULT '',
	product TEXT NOT NULL DEFAULT '',
	quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
	serial_numbers TEXT NOT NULL DEFAULT '', -- one per line
	fault TEXT NOT NULL DEFAULT '',
	invoice_number TEXT NOT NULL DEFAULT '',
	purchase_date DATE,
	warranty TEXT NOT NULL DEFAULT ''
		CHECK (warranty IN ('', 'doa', 'in_warranty', 'out_of_warranty')),
	test_result TEXT NOT NULL DEFAULT ''
		CHECK (test_result IN ('', 'fault_confirmed', 'no_fault_found', 'damaged')),
	supplier TEXT NOT NULL DEFAULT '',
	supplier_rma_ref TEXT NOT NULL DEFAULT '',
	credit_note_number TEXT NOT NULL DEFAULT '',
	replacement_order_number TEXT NOT NULL DEFAULT '',
	return_tracking TEXT NOT NULL DEFAULT '',
	approved_at TIMESTAMPTZ,
	received_at TIMESTAMPTZ,
	sent_to_supplier_at TIMESTAMPTZ,
	resolved_at TIMESTAMPTZ,
	kd_sample BOOL NOT NULL DEFAULT FALSE
);
CREATE UNIQUE INDEX IF NOT EXISTS kd_rmas_number_key ON kd_rmas (number);
CREATE UNIQUE INDEX IF NOT EXISTS kd_rmas_form_response_key ON kd_rmas (form_response_id) WHERE form_response_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS kd_rmas_customer ON kd_rmas (customer_id);
CREATE INDEX IF NOT EXISTS kd_rmas_conversation ON kd_rmas (conversation_id);
CREATE INDEX IF NOT EXISTS kd_rmas_status ON kd_rmas (status);

-- Who did what to an RMA, and when
CREATE TABLE IF NOT EXISTS kd_rma_events (
	id BIGSERIAL PRIMARY KEY,
	rma_id INT NOT NULL REFERENCES kd_rmas(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
	kind TEXT NOT NULL CHECK (kind IN ('created', 'status', 'update')),
	from_status TEXT NOT NULL DEFAULT '',
	to_status TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS kd_rma_events_rma ON kd_rma_events (rma_id, created_at);
`

// Allowed values and their labels (labels are used in ticket activity lines).
var (
	kdRmaStatusLabels = map[string]string{
		"requested":     "Requested",
		"approved":      "Approved",
		"received":      "Received",
		"testing":       "Testing",
		"with_supplier": "With supplier",
		"resolved":      "Resolved",
		"rejected":      "Rejected",
	}
	kdRmaOutcomeLabels = map[string]string{
		"replaced":     "Replaced",
		"repaired":     "Repaired",
		"credited":     "Credited",
		"returned_nff": "Returned – no fault found",
	}
	kdRmaWarrantyLabels = map[string]string{
		"doa":             "Dead on arrival",
		"in_warranty":     "In warranty",
		"out_of_warranty": "Out of warranty",
	}
	kdRmaTestLabels = map[string]string{
		"fault_confirmed": "Fault confirmed",
		"no_fault_found":  "No fault found",
		"damaged":         "Customer damage",
	}
	// Who KD sends a brand's faulty units back to, used as the default supplier.
	kdBrandSuppliers = map[string]string{
		"dahua":     "Dahua Technology",
		"ajax":      "Ajax Systems",
		"alta labs": "Alta Labs",
	}
	// Free email providers: their domain says nothing about the customer.
	kdFreeMailDomains = map[string]bool{
		"gmail.com": true, "googlemail.com": true, "outlook.com": true, "hotmail.com": true,
		"live.com": true, "live.ie": true, "yahoo.com": true, "yahoo.ie": true, "icloud.com": true,
		"me.com": true, "eircom.net": true, "proton.me": true, "protonmail.com": true, "aol.com": true,
	}
)

type kdRma struct {
	ID                     int        `db:"id" json:"id"`
	Number                 string     `db:"number" json:"number"`
	CreatedAt              time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at" json:"updated_at"`
	Status                 string     `db:"status" json:"status"`
	Outcome                string     `db:"outcome" json:"outcome"`
	Source                 string     `db:"source" json:"source"`
	CustomerID             *int       `db:"customer_id" json:"customer_id"`
	CustomerName           *string    `db:"customer_name" json:"customer_name"`
	ContactID              *int       `db:"contact_id" json:"contact_id"`
	ContactName            string     `db:"contact_name" json:"contact_name"`
	ContactEmail           string     `db:"contact_email" json:"contact_email"`
	TicketUUID             *string    `db:"ticket_uuid" json:"ticket_uuid"`
	TicketRef              *string    `db:"ticket_ref" json:"ticket_ref"`
	TicketStatus           *string    `db:"ticket_status" json:"ticket_status"`
	AssigneeID             *int       `db:"assignee_id" json:"assignee_id"`
	AssigneeName           *string    `db:"assignee_name" json:"assignee_name"`
	Brand                  string     `db:"brand" json:"brand"`
	Product                string     `db:"product" json:"product"`
	Quantity               int        `db:"quantity" json:"quantity"`
	SerialNumbers          string     `db:"serial_numbers" json:"serial_numbers"`
	Fault                  string     `db:"fault" json:"fault"`
	InvoiceNumber          string     `db:"invoice_number" json:"invoice_number"`
	PurchaseDate           string     `db:"purchase_date" json:"purchase_date"`
	Warranty               string     `db:"warranty" json:"warranty"`
	TestResult             string     `db:"test_result" json:"test_result"`
	Supplier               string     `db:"supplier" json:"supplier"`
	SupplierRmaRef         string     `db:"supplier_rma_ref" json:"supplier_rma_ref"`
	CreditNoteNumber       string     `db:"credit_note_number" json:"credit_note_number"`
	ReplacementOrderNumber string     `db:"replacement_order_number" json:"replacement_order_number"`
	ReturnTracking         string     `db:"return_tracking" json:"return_tracking"`
	ApprovedAt             *time.Time `db:"approved_at" json:"approved_at"`
	ReceivedAt             *time.Time `db:"received_at" json:"received_at"`
	SentToSupplierAt       *time.Time `db:"sent_to_supplier_at" json:"sent_to_supplier_at"`
	ResolvedAt             *time.Time `db:"resolved_at" json:"resolved_at"`
	Total                  int        `db:"total" json:"-"`
}

type kdRmaEvent struct {
	ID         int       `db:"id" json:"id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	Kind       string    `db:"kind" json:"kind"`
	FromStatus string    `db:"from_status" json:"from_status"`
	ToStatus   string    `db:"to_status" json:"to_status"`
	Note       string    `db:"note" json:"note"`
	Actor      string    `db:"actor" json:"actor"`
}

// The RMA with its customer, contact and ticket. The owner is the ticket's assignee.
const kdRmaSelect = `
	SELECT r.id, r.number, r.created_at, r.updated_at, r.status, r.outcome, r.source,
		r.customer_id, k.name AS customer_name,
		r.contact_id, COALESCE(TRIM(u.first_name || ' ' || COALESCE(u.last_name, '')), '') AS contact_name,
		COALESCE(u.email, '') AS contact_email,
		c.uuid::text AS ticket_uuid, c.reference_number AS ticket_ref, s.name AS ticket_status,
		c.assigned_user_id AS assignee_id,
		NULLIF(TRIM(a.first_name || ' ' || COALESCE(a.last_name, '')), '') AS assignee_name,
		r.brand, r.product, r.quantity, r.serial_numbers, r.fault, r.invoice_number,
		COALESCE(to_char(r.purchase_date, 'YYYY-MM-DD'), '') AS purchase_date,
		r.warranty, r.test_result, r.supplier, r.supplier_rma_ref, r.credit_note_number,
		r.replacement_order_number, r.return_tracking,
		r.approved_at, r.received_at, r.sent_to_supplier_at, r.resolved_at,
		COUNT(*) OVER () AS total
	FROM kd_rmas r
	LEFT JOIN kd_customers k ON k.id = r.customer_id
	LEFT JOIN users u ON u.id = r.contact_id
	LEFT JOIN conversations c ON c.id = r.conversation_id
	LEFT JOIN conversation_statuses s ON s.id = c.status_id
	LEFT JOIN users a ON a.id = c.assigned_user_id
`

// kdRmaFields are the parts of an RMA agents (and the form) fill in.
type kdRmaFields struct {
	CustomerID             int    `json:"customer_id"` // 0 = none
	Brand                  string `json:"brand"`
	Product                string `json:"product"`
	Quantity               int    `json:"quantity"`
	SerialNumbers          string `json:"serial_numbers"`
	Fault                  string `json:"fault"`
	InvoiceNumber          string `json:"invoice_number"`
	PurchaseDate           string `json:"purchase_date"` // YYYY-MM-DD, or DD/MM/YYYY
	Warranty               string `json:"warranty"`
	TestResult             string `json:"test_result"`
	Supplier               string `json:"supplier"`
	SupplierRmaRef         string `json:"supplier_rma_ref"`
	CreditNoteNumber       string `json:"credit_note_number"`
	ReplacementOrderNumber string `json:"replacement_order_number"`
	ReturnTracking         string `json:"return_tracking"`
}

// clean trims and checks the fields, returning a message for the agent when one is wrong.
func (f *kdRmaFields) clean() string {
	for _, s := range []*string{&f.Brand, &f.Product, &f.InvoiceNumber, &f.Warranty, &f.TestResult,
		&f.Supplier, &f.SupplierRmaRef, &f.CreditNoteNumber, &f.ReplacementOrderNumber, &f.ReturnTracking} {
		*s = strings.Join(strings.Fields(*s), " ")
	}
	f.Fault = strings.TrimSpace(strings.ReplaceAll(f.Fault, "\r\n", "\n"))
	f.SerialNumbers = kdCleanSerials(f.SerialNumbers)
	if f.Quantity < 1 {
		f.Quantity = 1
	}
	if f.Supplier == "" {
		f.Supplier = kdBrandSuppliers[strings.ToLower(f.Brand)]
	}

	date, ok := kdParseDate(f.PurchaseDate)
	switch {
	case f.Product == "":
		return "Add the product (model or code)"
	case f.Quantity > 10000:
		return "Quantity is too large"
	case !ok:
		return "Purchase date should look like 2026-03-14 or 14/03/2026"
	case date.After(time.Now().Add(24 * time.Hour)):
		return "Purchase date is in the future"
	case f.Warranty != "" && kdRmaWarrantyLabels[f.Warranty] == "":
		return "Unknown warranty value"
	case f.TestResult != "" && kdRmaTestLabels[f.TestResult] == "":
		return "Unknown test result"
	case len(f.Fault) > 5000 || len(f.SerialNumbers) > 5000:
		return "Fault or serial numbers are too long"
	}
	for _, s := range []string{f.Brand, f.Product, f.InvoiceNumber, f.Supplier, f.SupplierRmaRef,
		f.CreditNoteNumber, f.ReplacementOrderNumber, f.ReturnTracking} {
		if len(s) > 300 {
			return "One of the fields is too long"
		}
	}
	if date.IsZero() {
		f.PurchaseDate = ""
	} else {
		f.PurchaseDate = date.Format("2006-01-02")
	}
	return ""
}

// kdCleanSerials puts one serial number per line, dropping blanks and repeats.
// Serials can be typed one per line or separated by commas or semicolons.
func kdCleanSerials(s string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ';' }) {
		part = strings.TrimSpace(part)
		if part == "" || seen[strings.ToUpper(part)] {
			continue
		}
		seen[strings.ToUpper(part)] = true
		out = append(out, part)
	}
	return strings.Join(out, "\n")
}

// kdParseDate reads an optional date in ISO (Google Forms) or Irish DD/MM/YYYY form.
func kdParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, true
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2/1/2006", "02-01-2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func kdNullInt(id int) any {
	if id <= 0 {
		return nil
	}
	return id
}

func kdGetRma(db sqlx.Queryer, id int) (kdRma, error) {
	var r kdRma
	err := sqlx.Get(db, &r, kdRmaSelect+` WHERE r.id = $1`, id)
	return r, err
}

func kdRmaNotFound(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotFound, "RMA not found", nil, envelope.NotFoundError)
}

// kdRmaFilter narrows the RMA list. Status is a status key, "open" (not
// resolved or rejected) or empty for all.
type kdRmaFilter struct {
	Q, Status, Brand, Supplier string
	CustomerID, ConversationID int
}

func (f kdRmaFilter) where(withStatus bool) (string, []any) {
	conds, args := []string{"TRUE"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.Q != "" {
		add(`(r.number ILIKE '%' || ? || '%' OR r.serial_numbers ILIKE '%' || ? || '%'
			OR r.invoice_number ILIKE '%' || ? || '%' OR r.product ILIKE '%' || ? || '%'
			OR r.brand ILIKE '%' || ? || '%' OR r.supplier_rma_ref ILIKE '%' || ? || '%'
			OR k.name ILIKE '%' || ? || '%' OR u.email ILIKE '%' || ? || '%'
			OR (u.first_name || ' ' || COALESCE(u.last_name, '')) ILIKE '%' || ? || '%'
			OR c.reference_number = ?)`, f.Q)
	}
	if withStatus && f.Status == "open" {
		conds = append(conds, "r.status NOT IN ('resolved', 'rejected')")
	} else if withStatus && f.Status != "" {
		add("r.status = ?", f.Status)
	}
	if f.CustomerID > 0 {
		add("r.customer_id = ?", f.CustomerID)
	}
	if f.ConversationID > 0 {
		add("r.conversation_id = ?", f.ConversationID)
	}
	if f.Brand != "" {
		add("LOWER(r.brand) = LOWER(?)", f.Brand)
	}
	if f.Supplier != "" {
		add("LOWER(r.supplier) = LOWER(?)", f.Supplier)
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// GET /api/v1/kd/rmas?q=&status=&customer_id=&brand=&supplier=&page=&page_size=
// Also returns counts per status (for the status tabs), with the same filters
// apart from status.
func handleKDGetRmas(r *fastglue.Request) error {
	var (
		app  = r.Context.(*App)
		args = r.RequestCtx.QueryArgs()
		f    = kdRmaFilter{
			Q:        strings.TrimSpace(string(args.Peek("q"))),
			Status:   string(args.Peek("status")),
			Brand:    strings.TrimSpace(string(args.Peek("brand"))),
			Supplier: strings.TrimSpace(string(args.Peek("supplier"))),
		}
		out = []kdRma{}
	)
	f.CustomerID, _ = strconv.Atoi(string(args.Peek("customer_id")))
	if f.Status != "" && f.Status != "open" && kdRmaStatusLabels[f.Status] == "" {
		return kdBadRequest(r, "Unknown RMA status")
	}
	page, pageSize := getPagination(r)

	where, qargs := f.where(true)
	n := len(qargs)
	qargs = append(qargs, pageSize, (page-1)*pageSize)
	if err := app.db.Select(&out, kdRmaSelect+where+fmt.Sprintf(`
		ORDER BY r.created_at DESC, r.id DESC LIMIT $%d OFFSET $%d`, n+1, n+2), qargs...); err != nil {
		return kdServerError(r, err)
	}

	counts := []kdDashCount{}
	cwhere, cargs := f.where(false)
	if err := app.db.Select(&counts, `
		SELECT r.status AS name, COUNT(*) AS count
		FROM kd_rmas r
		LEFT JOIN kd_customers k ON k.id = r.customer_id
		LEFT JOIN users u ON u.id = r.contact_id
		LEFT JOIN conversations c ON c.id = r.conversation_id`+cwhere+` GROUP BY r.status`, cargs...); err != nil {
		return kdServerError(r, err)
	}
	byStatus := map[string]int{}
	for _, c := range counts {
		byStatus[c.Name] = c.Count
	}

	total := 0
	if len(out) > 0 {
		total = out[0].Total
	}
	return r.SendEnvelope(map[string]any{
		"results":     out,
		"total":       total,
		"per_page":    pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
		"page":        page,
		"counts":      byStatus,
	})
}

// GET /api/v1/kd/rmas/{id}: the RMA and its history.
func handleKDGetRma(r *fastglue.Request) error {
	app := r.Context.(*App)
	rma, err := kdGetRma(app.db, kdPathID(r, "id"))
	if errors.Is(err, sql.ErrNoRows) {
		return kdRmaNotFound(r)
	}
	if err != nil {
		return kdServerError(r, err)
	}
	events := []kdRmaEvent{}
	if err := app.db.Select(&events, `
		SELECT e.id, e.created_at, e.kind, e.from_status, e.to_status, e.note,
			COALESCE(NULLIF(TRIM(u.first_name || ' ' || COALESCE(u.last_name, '')), ''), 'System') AS actor
		FROM kd_rma_events e LEFT JOIN users u ON u.id = e.user_id
		WHERE e.rma_id = $1 ORDER BY e.created_at, e.id`, rma.ID); err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(map[string]any{"rma": rma, "events": events})
}

// GET /api/v1/conversations/{uuid}/rmas: the ticket's RMAs.
func handleKDGetTicketRmas(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		out   = []kdRma{}
	)
	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conv, err := enforceConversationAccess(app, r.RequestCtx.UserValue("uuid").(string), user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	where, args := kdRmaFilter{ConversationID: conv.ID}.where(false)
	if err := app.db.Select(&out, kdRmaSelect+where+` ORDER BY r.id`, args...); err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(out)
}

type kdCreateRmaReq struct {
	kdRmaFields
	ConversationUUID string `json:"conversation_uuid"` // raise on an existing ticket
	ContactID        int    `json:"contact_id"`        // else: requester of the new ticket
}

// POST /api/v1/kd/rmas: an agent raises an RMA, on an existing ticket or with
// a new one (logged as if the customer had written in, like New ticket).
func handleKDCreateRma(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   kdCreateRmaReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	if msg := req.clean(); msg != "" {
		return kdBadRequest(r, msg)
	}

	var (
		conv      *cmodels.Conversation
		contactID = req.ContactID
	)
	if req.ConversationUUID != "" {
		user, err := app.user.GetAgentCachedOrLoad(auser.ID)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}
		if conv, err = enforceConversationAccess(app, req.ConversationUUID, user); err != nil {
			return sendErrorEnvelope(r, err)
		}
		contactID = conv.ContactID
		if req.CustomerID <= 0 {
			if tc, err := kdTicketCustomerFor(app.db, conv.ID); err == nil && tc.Customer != nil {
				req.CustomerID = tc.Customer.ID
			}
		}
	} else if req.CustomerID <= 0 && req.ContactID <= 0 {
		return kdBadRequest(r, "Choose a customer or a contact")
	}

	var customerName string
	if req.CustomerID > 0 {
		err := app.db.Get(&customerName, `SELECT name FROM kd_customers WHERE id = $1`, req.CustomerID)
		if errors.Is(err, sql.ErrNoRows) {
			return kdBadRequest(r, "That customer no longer exists")
		}
		if err != nil {
			return kdServerError(r, err)
		}
	}
	if conv == nil {
		var err error
		if contactID > 0 {
			var ok bool
			if err := app.db.Get(&ok, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND type = 'contact' AND deleted_at IS NULL)`, contactID); err != nil {
				return kdServerError(r, err)
			}
			if !ok {
				return kdBadRequest(r, "That contact no longer exists")
			}
		} else if contactID, err = kdCompanyContact(app, req.CustomerID, customerName); err != nil {
			return sendErrorEnvelope(r, err)
		}
	}

	convID := 0
	if conv != nil {
		convID = conv.ID
	}
	rma, err := kdInsertRma(app, req.kdRmaFields, "agent", "", contactID, convID, auser.ID, "Raised by an agent")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if conv != nil {
		kdTicketActivity(app, conv.UUID, auser, fmt.Sprintf("raised %s for %s", rma.Number, kdRmaLine(rma.Quantity, rma.Brand, rma.Product)))
	} else if err := kdRmaOpenTicket(app, &rma, contactID, auser.ID, kdRmaTicketBody(rma, "Logged by an agent.", nil)); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(rma)
}

// kdInsertRma stores a new RMA with its "created" event, and links the contact
// to the customer if they have none yet.
func kdInsertRma(app *App, f kdRmaFields, source, formResponseID string, contactID, conversationID, actorID int, note string) (kdRma, error) {
	tx, err := app.db.Beginx()
	if err != nil {
		app.lo.Error("kd rmas: begin", "error", err)
		return kdRma{}, kdErr(app)
	}
	defer tx.Rollback()

	var id int
	if err := tx.Get(&id, `
		INSERT INTO kd_rmas (source, form_response_id, customer_id, contact_id, conversation_id,
			brand, product, quantity, serial_numbers, fault, invoice_number, purchase_date, warranty,
			test_result, supplier, supplier_rma_ref, credit_note_number, replacement_order_number, return_tracking)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, '')::date, $13,
			$14, $15, $16, $17, $18, $19)
		RETURNING id`,
		source, formResponseID, kdNullInt(f.CustomerID), kdNullInt(contactID), kdNullInt(conversationID),
		f.Brand, f.Product, f.Quantity, f.SerialNumbers, f.Fault, f.InvoiceNumber, f.PurchaseDate, f.Warranty,
		f.TestResult, f.Supplier, f.SupplierRmaRef, f.CreditNoteNumber, f.ReplacementOrderNumber, f.ReturnTracking); err != nil {
		app.lo.Error("kd rmas: insert", "error", err)
		return kdRma{}, kdErr(app)
	}
	if _, err := tx.Exec(`INSERT INTO kd_rma_events (rma_id, user_id, kind, to_status, note) VALUES ($1, $2, 'created', 'requested', $3)`,
		id, kdNullInt(actorID), note); err != nil {
		app.lo.Error("kd rmas: insert event", "error", err)
		return kdRma{}, kdErr(app)
	}
	if f.CustomerID > 0 && contactID > 0 {
		if _, err := tx.Exec(`INSERT INTO kd_contact_customers (contact_id, customer_id) VALUES ($1, $2)
			ON CONFLICT (contact_id) DO NOTHING`, contactID, f.CustomerID); err != nil {
			app.lo.Error("kd rmas: link contact", "error", err)
			return kdRma{}, kdErr(app)
		}
	}
	if err := tx.Commit(); err != nil {
		app.lo.Error("kd rmas: commit", "error", err)
		return kdRma{}, kdErr(app)
	}
	rma, err := kdGetRma(app.db, id)
	if err != nil {
		app.lo.Error("kd rmas: reload", "error", err)
		return kdRma{}, kdErr(app)
	}
	return rma, nil
}

// kdRmaOpenTicket logs the ticket an RMA lives on and links them. If the ticket
// can't be created the RMA is removed again, so there are no RMAs without one.
func kdRmaOpenTicket(app *App, rma *kdRma, contactID, actorID int, body string) error {
	customerID := 0
	if rma.CustomerID != nil {
		customerID = *rma.CustomerID
	}
	subject := rma.Number + ": " + kdRmaLine(rma.Quantity, rma.Brand, rma.Product)
	uuid, err := kdLogTicket(app, contactID, customerID, subject, body, actorID)
	if err == nil {
		if _, uerr := app.db.Exec(`UPDATE kd_rmas SET conversation_id = (SELECT id FROM conversations WHERE uuid = $2) WHERE id = $1`, rma.ID, uuid); uerr != nil {
			app.lo.Error("kd rmas: error linking ticket", "error", uerr)
			err = kdErr(app)
		}
	}
	if err != nil {
		app.lo.Error("kd rmas: error opening ticket", "rma", rma.Number, "error", err)
		if _, derr := app.db.Exec(`DELETE FROM kd_rmas WHERE id = $1`, rma.ID); derr != nil {
			app.lo.Error("kd rmas: error removing RMA without ticket", "error", derr)
		}
		return err
	}
	if fresh, err := kdGetRma(app.db, rma.ID); err == nil {
		*rma = fresh
	}
	return nil
}

// kdRmaLine is the short product line, e.g. "3 × Ajax FireProtect 2".
func kdRmaLine(qty int, brand, product string) string {
	name := product
	if brand != "" && !strings.HasPrefix(strings.ToLower(product), strings.ToLower(brand)) {
		name = brand + " " + product
	}
	return fmt.Sprintf("%d × %s", qty, name)
}

// kdRmaTicketBody is the ticket's first message: the RMA details, plus any
// form answers that have no RMA field of their own.
func kdRmaTicketBody(rma kdRma, intro string, extra [][2]string) string {
	var b strings.Builder
	row := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			b.WriteString("<strong>" + html.EscapeString(label) + ":</strong> " +
				strings.ReplaceAll(html.EscapeString(value), "\n", "<br>") + "<br>")
		}
	}
	b.WriteString("<p>" + html.EscapeString(intro) + " RMA number: <strong>" + html.EscapeString(rma.Number) + "</strong></p><p>")
	row("Product", kdRmaLine(rma.Quantity, rma.Brand, rma.Product))
	row("Serial numbers", strings.ReplaceAll(rma.SerialNumbers, "\n", ", "))
	row("Invoice / order", rma.InvoiceNumber)
	if t, ok := kdParseDate(rma.PurchaseDate); ok && !t.IsZero() {
		row("Purchased", t.Format("2 Jan 2006"))
	}
	b.WriteString("</p>")
	if rma.Fault != "" {
		b.WriteString("<p><strong>Fault:</strong><br>" + strings.ReplaceAll(html.EscapeString(rma.Fault), "\n", "<br>") + "</p>")
	}
	if len(extra) > 0 {
		b.WriteString("<p>")
		for _, kv := range extra {
			row(kv[0], kv[1])
		}
		b.WriteString("</p>")
	}
	return b.String()
}

// kdTicketActivity adds a private activity line ("Aoife Byrne raised RMA-1001 ...")
// to the ticket's timeline. Failures are logged, not returned.
func kdTicketActivity(app *App, conversationUUID string, actor amodels.User, text string) {
	name := strings.TrimSpace(actor.FirstName + " " + actor.LastName)
	if name == "" {
		name = "System"
	}
	msg := cmodels.Message{
		Type:             cmodels.MessageActivity,
		Status:           cmodels.MessageStatusSent,
		Content:          name + " " + text,
		ContentType:      cmodels.ContentTypeText,
		ConversationUUID: conversationUUID,
		Private:          true,
		SenderID:         actor.ID,
		SenderType:       cmodels.SenderTypeAgent,
		Meta:             []byte(`{"activity_type": "kd_rma"}`),
	}
	if err := app.conversation.InsertMessage(&msg); err != nil {
		app.lo.Error("kd rmas: error adding ticket activity", "error", err)
	}
}

// PUT /api/v1/kd/rmas/{id}: edit the RMA's details (not its status).
func handleKDUpdateRma(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		id    = kdPathID(r, "id")
		req   kdRmaFields
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	// Supplier is left as typed when editing (cleared on purpose stays cleared).
	supplier := strings.TrimSpace(req.Supplier)
	if msg := req.clean(); msg != "" {
		return kdBadRequest(r, msg)
	}
	req.Supplier = strings.Join(strings.Fields(supplier), " ")

	cur, err := kdGetRma(app.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		return kdRmaNotFound(r)
	}
	if err != nil {
		return kdServerError(r, err)
	}
	if req.CustomerID > 0 {
		var ok bool
		if err := app.db.Get(&ok, `SELECT EXISTS (SELECT 1 FROM kd_customers WHERE id = $1)`, req.CustomerID); err != nil {
			return kdServerError(r, err)
		}
		if !ok {
			return kdBadRequest(r, "That customer no longer exists")
		}
	}

	// Record which fields changed, in words, for the history.
	curCustomer := 0
	if cur.CustomerID != nil {
		curCustomer = *cur.CustomerID
	}
	changed := []string{}
	for _, c := range []struct {
		label    string
		old, new any
	}{
		{"customer", curCustomer, max(req.CustomerID, 0)},
		{"brand", cur.Brand, req.Brand},
		{"product", cur.Product, req.Product},
		{"quantity", cur.Quantity, req.Quantity},
		{"serial numbers", cur.SerialNumbers, req.SerialNumbers},
		{"fault", cur.Fault, req.Fault},
		{"invoice", cur.InvoiceNumber, req.InvoiceNumber},
		{"purchase date", cur.PurchaseDate, req.PurchaseDate},
		{"warranty", cur.Warranty, req.Warranty},
		{"test result", cur.TestResult, req.TestResult},
		{"supplier", cur.Supplier, req.Supplier},
		{"supplier RMA ref", cur.SupplierRmaRef, req.SupplierRmaRef},
		{"credit note", cur.CreditNoteNumber, req.CreditNoteNumber},
		{"replacement order", cur.ReplacementOrderNumber, req.ReplacementOrderNumber},
		{"return tracking", cur.ReturnTracking, req.ReturnTracking},
	} {
		if c.old != c.new {
			changed = append(changed, c.label)
		}
	}
	if len(changed) == 0 {
		return r.SendEnvelope(cur)
	}

	tx, err := app.db.Beginx()
	if err != nil {
		return kdServerError(r, err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		UPDATE kd_rmas SET customer_id = $2, brand = $3, product = $4, quantity = $5, serial_numbers = $6,
			fault = $7, invoice_number = $8, purchase_date = NULLIF($9, '')::date, warranty = $10,
			test_result = $11, supplier = $12, supplier_rma_ref = $13, credit_note_number = $14,
			replacement_order_number = $15, return_tracking = $16, updated_at = NOW()
		WHERE id = $1`,
		id, kdNullInt(req.CustomerID), req.Brand, req.Product, req.Quantity, req.SerialNumbers, req.Fault,
		req.InvoiceNumber, req.PurchaseDate, req.Warranty, req.TestResult, req.Supplier, req.SupplierRmaRef,
		req.CreditNoteNumber, req.ReplacementOrderNumber, req.ReturnTracking); err != nil {
		return kdServerError(r, err)
	}
	if _, err := tx.Exec(`INSERT INTO kd_rma_events (rma_id, user_id, kind, note) VALUES ($1, $2, 'update', $3)`,
		id, auser.ID, "Updated "+strings.Join(changed, ", ")); err != nil {
		return kdServerError(r, err)
	}
	if req.CustomerID > 0 && cur.ContactID != nil {
		if _, err := tx.Exec(`INSERT INTO kd_contact_customers (contact_id, customer_id) VALUES ($1, $2)
			ON CONFLICT (contact_id) DO NOTHING`, *cur.ContactID, req.CustomerID); err != nil {
			return kdServerError(r, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return kdServerError(r, err)
	}
	rma, err := kdGetRma(app.db, id)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(rma)
}

type kdRmaStatusReq struct {
	Status  string `json:"status"`
	Outcome string `json:"outcome"` // required when resolving
	Note    string `json:"note"`    // e.g. why it was rejected
}

// POST /api/v1/kd/rmas/{id}/status: move the RMA to another step. Any step can
// be chosen (agents fix mistakes by moving back); the dates of each step are
// kept for turnaround reporting, and reopening a closed RMA clears its end date.
func handleKDSetRmaStatus(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		id    = kdPathID(r, "id")
		req   kdRmaStatusReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	req.Note = strings.TrimSpace(req.Note)
	switch {
	case kdRmaStatusLabels[req.Status] == "":
		return kdBadRequest(r, "Unknown RMA status")
	case req.Status == "resolved" && kdRmaOutcomeLabels[req.Outcome] == "":
		return kdBadRequest(r, "Choose the outcome: replaced, repaired, credited or returned")
	case len(req.Note) > 2000:
		return kdBadRequest(r, "Note is too long")
	}
	if req.Status != "resolved" {
		req.Outcome = ""
	}

	tx, err := app.db.Beginx()
	if err != nil {
		return kdServerError(r, err)
	}
	defer tx.Rollback()
	var cur struct {
		Status  string `db:"status"`
		Outcome string `db:"outcome"`
	}
	err = tx.Get(&cur, `SELECT status, outcome FROM kd_rmas WHERE id = $1 FOR UPDATE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return kdRmaNotFound(r)
	}
	if err != nil {
		return kdServerError(r, err)
	}
	if cur.Status == req.Status && cur.Outcome == req.Outcome && req.Note == "" {
		rma, err := kdGetRma(app.db, id)
		if err != nil {
			return kdServerError(r, err)
		}
		return r.SendEnvelope(rma)
	}
	if _, err := tx.Exec(`
		UPDATE kd_rmas SET status = $2, outcome = $3, updated_at = NOW(),
			approved_at = CASE WHEN $2 IN ('approved', 'received', 'testing', 'with_supplier', 'resolved')
				THEN COALESCE(approved_at, NOW()) ELSE approved_at END,
			received_at = CASE WHEN $2 IN ('received', 'testing', 'with_supplier')
				THEN COALESCE(received_at, NOW()) ELSE received_at END,
			sent_to_supplier_at = CASE WHEN $2 = 'with_supplier'
				THEN COALESCE(sent_to_supplier_at, NOW()) ELSE sent_to_supplier_at END,
			resolved_at = CASE WHEN $2 IN ('resolved', 'rejected') THEN COALESCE(resolved_at, NOW()) ELSE NULL END
		WHERE id = $1`, id, req.Status, req.Outcome); err != nil {
		return kdServerError(r, err)
	}
	note := req.Note
	if req.Outcome != "" {
		note = strings.TrimSpace(kdRmaOutcomeLabels[req.Outcome] + ". " + note)
	}
	if _, err := tx.Exec(`INSERT INTO kd_rma_events (rma_id, user_id, kind, from_status, to_status, note)
		VALUES ($1, $2, 'status', $3, $4, $5)`, id, auser.ID, cur.Status, req.Status, note); err != nil {
		return kdServerError(r, err)
	}
	if err := tx.Commit(); err != nil {
		return kdServerError(r, err)
	}

	rma, err := kdGetRma(app.db, id)
	if err != nil {
		return kdServerError(r, err)
	}
	if rma.TicketUUID != nil {
		text := "moved " + rma.Number + " to " + kdRmaStatusLabels[rma.Status]
		if rma.Outcome != "" {
			text += " (" + kdRmaOutcomeLabels[rma.Outcome] + ")"
		}
		if req.Note != "" {
			short := []rune(req.Note)
			if len(short) > 160 {
				short = append(short[:160], '…')
			}
			text += ": " + string(short)
		}
		kdTicketActivity(app, *rma.TicketUUID, auser, text)
	}
	return r.SendEnvelope(rma)
}

// kdRmaIntakeReq is one RMA Google Form submission, as sent by the Apps Script.
// Everything is a string because that's what forms give us.
type kdRmaIntakeReq struct {
	ResponseID    string `json:"response_id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Company       string `json:"company"`
	Phone         string `json:"phone"`
	InvoiceNumber string `json:"invoice_number"`
	PurchaseDate  string `json:"purchase_date"`
	Brand         string `json:"brand"`
	Product       string `json:"product"`
	Quantity      string `json:"quantity"`
	SerialNumbers string `json:"serial_numbers"`
	Fault         string `json:"fault"`
	// Answers to questions that have no RMA field, kept on the ticket: [question, answer] pairs.
	Other [][2]string `json:"other"`
}

// POST /api/v1/kd/rmas/intake: a Google Form submission becomes an RMA and a
// ticket. Called with the API key of the form's own agent ("RMA Form"). Sending
// the same response again returns the RMA made the first time.
func handleKDRmaIntake(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   kdRmaIntakeReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	req.ResponseID = strings.TrimSpace(req.ResponseID)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.Join(strings.Fields(req.Name), " ")
	req.Company = strings.Join(strings.Fields(req.Company), " ")
	req.Phone = strings.Join(strings.Fields(req.Phone), " ")
	if req.ResponseID == "" || len(req.ResponseID) > 200 {
		return kdBadRequest(r, "response_id is required")
	}
	if addr, err := mail.ParseAddress(req.Email); err != nil || addr.Address != req.Email {
		return kdBadRequest(r, "A valid email address is required")
	}

	// Already received? Return the RMA made the first time.
	var existing int
	err := app.db.Get(&existing, `SELECT id FROM kd_rmas WHERE form_response_id = $1`, req.ResponseID)
	if err == nil {
		rma, err := kdGetRma(app.db, existing)
		if err != nil {
			return kdServerError(r, err)
		}
		return r.SendEnvelope(map[string]any{"rma": rma, "duplicate": true})
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return kdServerError(r, err)
	}

	f := kdRmaFields{
		Brand:         req.Brand,
		Product:       req.Product,
		SerialNumbers: req.SerialNumbers,
		Fault:         req.Fault,
		InvoiceNumber: req.InvoiceNumber,
		PurchaseDate:  req.PurchaseDate,
	}
	f.Quantity, _ = strconv.Atoi(strings.TrimSpace(req.Quantity))
	extra := [][2]string{}
	// A badly typed date shouldn't lose the request: keep it on the ticket instead.
	if _, ok := kdParseDate(f.PurchaseDate); !ok {
		extra = append(extra, [2]string{"Purchase date (as typed)", f.PurchaseDate})
		f.PurchaseDate = ""
	}
	if msg := f.clean(); msg != "" {
		return kdBadRequest(r, msg)
	}

	contactID, err := kdIntakeContact(app, req)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if f.CustomerID, err = kdMatchCustomer(app.db, contactID, req.Email, req.Company); err != nil {
		return kdServerError(r, err)
	}

	note := "From the RMA form"
	if f.CustomerID == 0 {
		note += " - no matching customer, link one"
	}
	rma, err := kdInsertRma(app, f, "form", req.ResponseID, contactID, 0, auser.ID, note)
	if err != nil {
		// Two deliveries of the same response at once: the other one won.
		if existingErr := app.db.Get(&existing, `SELECT id FROM kd_rmas WHERE form_response_id = $1`, req.ResponseID); existingErr == nil {
			if rma, err := kdGetRma(app.db, existing); err == nil {
				return r.SendEnvelope(map[string]any{"rma": rma, "duplicate": true})
			}
		}
		return sendErrorEnvelope(r, err)
	}

	for _, kv := range [][2]string{{"Name", req.Name}, {"Company (as entered)", req.Company}, {"Phone", req.Phone}} {
		if kv[1] != "" {
			extra = append(extra, kv)
		}
	}
	for _, kv := range req.Other {
		if strings.TrimSpace(kv[1]) != "" && len(extra) < 50 {
			extra = append(extra, kv)
		}
	}
	if err := kdRmaOpenTicket(app, &rma, contactID, auser.ID, kdRmaTicketBody(rma, "RMA request sent through the RMA form.", extra)); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(map[string]any{"rma": rma, "duplicate": false})
}

// kdIntakeContact finds the form sender's contact by email, or adds them.
func kdIntakeContact(app *App, req kdRmaIntakeReq) (int, error) {
	if c, err := app.user.GetContactByEmail(req.Email); err == nil {
		return c.ID, nil
	} else if envErr, ok := err.(envelope.Error); !ok || envErr.ErrorType != envelope.NotFoundError {
		return 0, err
	}
	first, last, _ := strings.Cut(req.Name, " ")
	if first == "" {
		first, _, _ = strings.Cut(req.Email, "@")
	}
	contact := umodels.User{
		Email:       null.StringFrom(req.Email),
		FirstName:   first,
		LastName:    last,
		PhoneNumber: null.NewString(req.Phone, req.Phone != ""),
	}
	if err := app.user.CreateContact(&contact); err != nil {
		// Created by a parallel request in the meantime?
		if c, gerr := app.user.GetContactByEmail(req.Email); gerr == nil {
			return c.ID, nil
		}
		return 0, err
	}
	return contact.ID, nil
}

// kdMatchCustomer finds the customer for a form submission: the contact's own
// customer, else the one whose domain matches their email's, else one named
// exactly like the company they typed. 0 when nothing matches (an agent links it).
func kdMatchCustomer(db *sqlx.DB, contactID int, email, company string) (int, error) {
	_, domain, _ := strings.Cut(strings.ToLower(email), "@")
	if kdFreeMailDomains[domain] {
		domain = ""
	}
	var id int
	err := db.Get(&id, `
		SELECT id FROM (
			SELECT customer_id AS id, 1 AS rank FROM kd_contact_customers WHERE contact_id = $1
			UNION ALL
			SELECT id, 2 FROM kd_customers WHERE $2 <> '' AND domain <> '' AND LOWER(domain) = $2
			UNION ALL
			SELECT id, 3 FROM kd_customers WHERE $3 <> '' AND LOWER(name) = LOWER($3)
		) m ORDER BY rank, id LIMIT 1`, contactID, domain, company)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}
