package main

// KD: Customers (companies) - a customer groups contacts, and every ticket
// belongs to one customer. A ticket's customer is the one set on the ticket,
// or else its requester's customer.

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/jmoiron/sqlx"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// kdSchema creates KD's own tables. Safe to run on every start.
const kdSchema = `
CREATE TABLE IF NOT EXISTS kd_customers (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	name TEXT NOT NULL,
	domain TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	kd_sample BOOL NOT NULL DEFAULT FALSE
);
CREATE UNIQUE INDEX IF NOT EXISTS kd_customers_name_key ON kd_customers (LOWER(name));

CREATE TABLE IF NOT EXISTS kd_contact_customers (
	contact_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
	customer_id INT NOT NULL REFERENCES kd_customers(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS kd_contact_customers_customer ON kd_contact_customers (customer_id);

CREATE TABLE IF NOT EXISTS kd_conversation_customers (
	conversation_id BIGINT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
	customer_id INT NOT NULL REFERENCES kd_customers(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS kd_conversation_customers_customer ON kd_conversation_customers (customer_id);
`

// kdTicketCustomer is each ticket's effective customer: set on the ticket, else the requester's.
const kdTicketCustomer = `
	SELECT c.id AS conversation_id, COALESCE(cc.customer_id, kc.customer_id) AS customer_id
	FROM conversations c
	LEFT JOIN kd_conversation_customers cc ON cc.conversation_id = c.id
	LEFT JOIN kd_contact_customers kc ON kc.contact_id = c.contact_id
`

func kdEnsureSchema(db *sqlx.DB) error {
	_, err := db.Exec(kdSchema)
	return err
}

type kdCustomer struct {
	ID          int       `db:"id" json:"id"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	Name        string    `db:"name" json:"name"`
	Domain      string    `db:"domain" json:"domain"`
	Phone       string    `db:"phone" json:"phone"`
	Notes       string    `db:"notes" json:"notes"`
	Contacts    int       `db:"contacts" json:"contacts"`
	OpenTickets int       `db:"open_tickets" json:"open_tickets"`
	Tickets     int       `db:"tickets" json:"tickets"`
	Total       int       `db:"total" json:"-"`
}

type kdCustomerContact struct {
	ID        int    `db:"id" json:"id"`
	FirstName string `db:"first_name" json:"first_name"`
	LastName  string `db:"last_name" json:"last_name"`
	Email     string `db:"email" json:"email"`
	Phone     string `db:"phone" json:"phone"`
}

type kdCustomerTicket struct {
	UUID            string    `db:"uuid" json:"uuid"`
	ReferenceNumber string    `db:"reference_number" json:"reference_number"`
	Subject         string    `db:"subject" json:"subject"`
	Status          string    `db:"status" json:"status"`
	ContactName     string    `db:"contact_name" json:"contact_name"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

type kdCustomerReq struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Phone  string `json:"phone"`
	Notes  string `json:"notes"`
}

const kdCustomerSelect = `
	WITH t AS (` + kdTicketCustomer + `)
	SELECT k.id, k.created_at, k.updated_at, k.name, k.domain, k.phone, k.notes,
		(SELECT COUNT(*) FROM kd_contact_customers kc WHERE kc.customer_id = k.id) AS contacts,
		(SELECT COUNT(*) FROM t JOIN conversations c ON c.id = t.conversation_id
			JOIN conversation_statuses s ON s.id = c.status_id
			WHERE t.customer_id = k.id AND s.category <> 'resolved') AS open_tickets,
		(SELECT COUNT(*) FROM t WHERE t.customer_id = k.id) AS tickets,
		COUNT(*) OVER () AS total
	FROM kd_customers k
`

func kdBadRequest(r *fastglue.Request, msg string) error {
	return r.SendErrorEnvelope(fasthttp.StatusBadRequest, msg, nil, envelope.InputError)
}

func kdServerError(r *fastglue.Request, err error) error {
	app := r.Context.(*App)
	app.lo.Error("kd customers", "error", err)
	return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
}

func kdPathID(r *fastglue.Request, key string) int {
	id, _ := strconv.Atoi(r.RequestCtx.UserValue(key).(string))
	return id
}

func (req *kdCustomerReq) clean() {
	req.Name = strings.TrimSpace(req.Name)
	req.Domain = strings.ToLower(strings.TrimSpace(req.Domain))
	req.Phone = strings.TrimSpace(req.Phone)
	req.Notes = strings.TrimSpace(req.Notes)
}

func kdIsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "kd_customers_name_key")
}

// GET /api/v1/customers?q=&page=&page_size=
func handleKDGetCustomers(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		q   = strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("q")))
		out = []kdCustomer{}
	)
	page, pageSize := getPagination(r)
	err := app.db.Select(&out, kdCustomerSelect+`
		WHERE $1 = '' OR k.name ILIKE '%' || $1 || '%' OR k.domain ILIKE '%' || $1 || '%'
		ORDER BY LOWER(k.name) LIMIT $2 OFFSET $3`, q, pageSize, (page-1)*pageSize)
	if err != nil {
		return kdServerError(r, err)
	}
	total := 0
	if len(out) > 0 {
		total = out[0].Total
	}
	return r.SendEnvelope(envelope.PageResults{
		Results:    out,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

func kdGetCustomer(db *sqlx.DB, id int) (kdCustomer, error) {
	var c kdCustomer
	err := db.Get(&c, kdCustomerSelect+` WHERE k.id = $1`, id)
	return c, err
}

// GET /api/v1/customers/{id}: the customer with its contacts and tickets.
func handleKDGetCustomer(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		id  = kdPathID(r, "id")
	)
	c, err := kdGetCustomer(app.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Customer not found", nil, envelope.NotFoundError)
	}
	if err != nil {
		return kdServerError(r, err)
	}
	contacts := []kdCustomerContact{}
	if err := app.db.Select(&contacts, `
		SELECT u.id, u.first_name, COALESCE(u.last_name, '') AS last_name, COALESCE(u.email, '') AS email,
			TRIM(COALESCE(u.phone_number_country_code, '') || ' ' || COALESCE(u.phone_number, '')) AS phone
		FROM kd_contact_customers kc JOIN users u ON u.id = kc.contact_id
		WHERE kc.customer_id = $1 AND u.deleted_at IS NULL
		ORDER BY LOWER(u.first_name), LOWER(COALESCE(u.last_name, ''))`, id); err != nil {
		return kdServerError(r, err)
	}
	tickets := []kdCustomerTicket{}
	if err := app.db.Select(&tickets, `
		WITH t AS (`+kdTicketCustomer+`)
		SELECT c.uuid, c.reference_number, COALESCE(c.subject, '') AS subject, s.name AS status,
			TRIM(u.first_name || ' ' || COALESCE(u.last_name, '')) AS contact_name,
			COALESCE(c.last_message_at, c.updated_at) AS updated_at
		FROM t JOIN conversations c ON c.id = t.conversation_id
		JOIN conversation_statuses s ON s.id = c.status_id
		JOIN users u ON u.id = c.contact_id
		WHERE t.customer_id = $1
		ORDER BY COALESCE(c.last_message_at, c.updated_at) DESC LIMIT 200`, id); err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(map[string]any{"customer": c, "contacts": contacts, "tickets": tickets})
}

// POST /api/v1/customers
func handleKDCreateCustomer(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req kdCustomerReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	req.clean()
	if req.Name == "" {
		return kdBadRequest(r, "Customer name is required")
	}
	var id int
	err := app.db.Get(&id, `INSERT INTO kd_customers (name, domain, phone, notes) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.Name, req.Domain, req.Phone, req.Notes)
	if kdIsUniqueViolation(err) {
		return kdBadRequest(r, "A customer with this name already exists")
	}
	if err != nil {
		return kdServerError(r, err)
	}
	c, err := kdGetCustomer(app.db, id)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(c)
}

// PUT /api/v1/customers/{id}
func handleKDUpdateCustomer(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		id  = kdPathID(r, "id")
		req kdCustomerReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	req.clean()
	if req.Name == "" {
		return kdBadRequest(r, "Customer name is required")
	}
	res, err := app.db.Exec(`UPDATE kd_customers SET name = $2, domain = $3, phone = $4, notes = $5, updated_at = NOW() WHERE id = $1`,
		id, req.Name, req.Domain, req.Phone, req.Notes)
	if kdIsUniqueViolation(err) {
		return kdBadRequest(r, "A customer with this name already exists")
	}
	if err != nil {
		return kdServerError(r, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Customer not found", nil, envelope.NotFoundError)
	}
	c, err := kdGetCustomer(app.db, id)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(c)
}

// DELETE /api/v1/customers/{id}: unlinks its contacts and tickets, which are kept.
func handleKDDeleteCustomer(r *fastglue.Request) error {
	app := r.Context.(*App)
	if _, err := app.db.Exec(`DELETE FROM kd_customers WHERE id = $1`, kdPathID(r, "id")); err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(true)
}

type kdLinkContactReq struct {
	ContactID int `json:"contact_id"`
}

// POST /api/v1/customers/{id}/contacts: links a contact (moving it from any other customer).
func handleKDLinkContact(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		id  = kdPathID(r, "id")
		req kdLinkContactReq
	)
	if err := r.Decode(&req, "json"); err != nil || req.ContactID <= 0 {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	_, err := app.db.Exec(`
		INSERT INTO kd_contact_customers (contact_id, customer_id)
		SELECT u.id, $2 FROM users u WHERE u.id = $1 AND u.type IN ('contact', 'visitor')
		ON CONFLICT (contact_id) DO UPDATE SET customer_id = EXCLUDED.customer_id`, req.ContactID, id)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(true)
}

// DELETE /api/v1/customers/{id}/contacts/{contact_id}
func handleKDUnlinkContact(r *fastglue.Request) error {
	app := r.Context.(*App)
	if _, err := app.db.Exec(`DELETE FROM kd_contact_customers WHERE customer_id = $1 AND contact_id = $2`,
		kdPathID(r, "id"), kdPathID(r, "contact_id")); err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(true)
}

type kdTicketCustomerResp struct {
	Customer *kdCustomer `json:"customer"`
	// "ticket" when set on the ticket, "requester" when it comes from the requester.
	Source string `json:"source"`
}

func kdTicketCustomerFor(db *sqlx.DB, conversationID int) (kdTicketCustomerResp, error) {
	var row struct {
		Explicit sql.NullInt64 `db:"explicit"`
		FromReq  sql.NullInt64 `db:"from_requester"`
	}
	err := db.Get(&row, `
		SELECT cc.customer_id AS explicit, kc.customer_id AS from_requester
		FROM conversations c
		LEFT JOIN kd_conversation_customers cc ON cc.conversation_id = c.id
		LEFT JOIN kd_contact_customers kc ON kc.contact_id = c.contact_id
		WHERE c.id = $1`, conversationID)
	if err != nil {
		return kdTicketCustomerResp{}, err
	}
	resp := kdTicketCustomerResp{}
	id := 0
	switch {
	case row.Explicit.Valid:
		id, resp.Source = int(row.Explicit.Int64), "ticket"
	case row.FromReq.Valid:
		id, resp.Source = int(row.FromReq.Int64), "requester"
	default:
		return resp, nil
	}
	c, err := kdGetCustomer(db, id)
	if err != nil {
		return resp, err
	}
	resp.Customer = &c
	return resp, nil
}

// GET /api/v1/conversations/{uuid}/customer
func handleKDGetTicketCustomer(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	resp, err := kdTicketCustomerFor(app.db, conv.ID)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(resp)
}

type kdSetTicketCustomerReq struct {
	// 0 clears the ticket's own customer (it then follows the requester's).
	CustomerID int `json:"customer_id"`
}

// PUT /api/v1/conversations/{uuid}/customer. Also links the requester to the
// customer when they don't have one yet, so their next tickets get it too.
func handleKDSetTicketCustomer(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   kdSetTicketCustomerReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	tx, err := app.db.Beginx()
	if err != nil {
		return kdServerError(r, err)
	}
	defer tx.Rollback()
	if req.CustomerID <= 0 {
		_, err = tx.Exec(`DELETE FROM kd_conversation_customers WHERE conversation_id = $1`, conv.ID)
	} else {
		_, err = tx.Exec(`
			INSERT INTO kd_conversation_customers (conversation_id, customer_id) VALUES ($1, $2)
			ON CONFLICT (conversation_id) DO UPDATE SET customer_id = EXCLUDED.customer_id`, conv.ID, req.CustomerID)
		if err == nil {
			_, err = tx.Exec(`INSERT INTO kd_contact_customers (contact_id, customer_id) VALUES ($1, $2)
				ON CONFLICT (contact_id) DO NOTHING`, conv.ContactID, req.CustomerID)
		}
	}
	if err != nil {
		return kdServerError(r, err)
	}
	if err := tx.Commit(); err != nil {
		return kdServerError(r, err)
	}
	resp, err := kdTicketCustomerFor(app.db, conv.ID)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(resp)
}

// GET /api/v1/contacts/{id}/customer: the contact's customer, or null.
func handleKDGetContactCustomer(r *fastglue.Request) error {
	app := r.Context.(*App)
	var id int
	err := app.db.Get(&id, `SELECT customer_id FROM kd_contact_customers WHERE contact_id = $1`, kdPathID(r, "id"))
	if errors.Is(err, sql.ErrNoRows) {
		return r.SendEnvelope(nil)
	}
	if err != nil {
		return kdServerError(r, err)
	}
	c, err := kdGetCustomer(app.db, id)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(c)
}

type kdContactRow struct {
	ID           int       `db:"id" json:"id"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	FirstName    string    `db:"first_name" json:"first_name"`
	LastName     string    `db:"last_name" json:"last_name"`
	Email        string    `db:"email" json:"email"`
	Phone        string    `db:"phone" json:"phone"`
	AvatarURL    *string   `db:"avatar_url" json:"avatar_url"`
	CustomerID   *int      `db:"customer_id" json:"customer_id"`
	CustomerName *string   `db:"customer_name" json:"customer_name"`
	Tickets      int       `db:"tickets" json:"tickets"`
	OpenTickets  int       `db:"open_tickets" json:"open_tickets"`
	Total        int       `db:"total" json:"-"`
}

// GET /api/v1/kd/contacts?q=&page=&page_size=: contacts with their customer and
// ticket counts; q matches name, email, phone or customer name.
func handleKDGetContacts(r *fastglue.Request) error {
	var (
		app           = r.Context.(*App)
		q             = strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("q")))
		customerID, _ = strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("customer_id")))
		out           = []kdContactRow{}
	)
	page, pageSize := getPagination(r)
	err := app.db.Select(&out, `
		SELECT u.id, u.created_at, u.first_name, COALESCE(u.last_name, '') AS last_name,
			COALESCE(u.email, '') AS email,
			TRIM(COALESCE(u.phone_number_country_code, '') || ' ' || COALESCE(u.phone_number, '')) AS phone,
			u.avatar_url, k.id AS customer_id, k.name AS customer_name,
			(SELECT COUNT(*) FROM conversations c WHERE c.contact_id = u.id) AS tickets,
			(SELECT COUNT(*) FROM conversations c JOIN conversation_statuses s ON s.id = c.status_id
				WHERE c.contact_id = u.id AND s.category <> 'resolved') AS open_tickets,
			COUNT(*) OVER () AS total
		FROM users u
		LEFT JOIN kd_contact_customers kc ON kc.contact_id = u.id
		LEFT JOIN kd_customers k ON k.id = kc.customer_id
		WHERE u.type = 'contact' AND u.deleted_at IS NULL
		AND ($1 = '' OR (u.first_name || ' ' || COALESCE(u.last_name, '')) ILIKE '%' || $1 || '%'
			OR u.email ILIKE '%' || $1 || '%' OR u.phone_number ILIKE '%' || $1 || '%'
			OR k.name ILIKE '%' || $1 || '%')
		AND ($4 = 0 OR kc.customer_id = $4)
		ORDER BY LOWER(u.first_name), LOWER(COALESCE(u.last_name, '')), u.id
		LIMIT $2 OFFSET $3`, q, pageSize, (page-1)*pageSize, customerID)
	if err != nil {
		return kdServerError(r, err)
	}
	total := 0
	if len(out) > 0 {
		total = out[0].Total
	}
	return r.SendEnvelope(envelope.PageResults{
		Results:    out,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}
