package main

// KD: phone calls logged by hand. Most KD support starts with an installer
// ringing in, and Nuacom isn't connected in this build, so agents log each
// call: on an existing ticket, or as a new ticket in a few seconds. A call is
// a note on the ticket's conversation (so it reads in the timeline, history
// and HubSpot export) plus a row here for the call's details, call-backs and
// reporting. Tickets record where they came from in meta.kd_source.

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/valyala/fasthttp"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/fastglue"
)

const kdCallSchema = `
CREATE TABLE IF NOT EXISTS kd_calls (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	called_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
	conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	contact_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
	user_id BIGINT REFERENCES users(id) ON DELETE SET NULL, -- agent who took / made it
	direction TEXT NOT NULL DEFAULT 'incoming' CHECK (direction IN ('incoming', 'outgoing')),
	phone TEXT NOT NULL DEFAULT '',
	duration_mins INT CHECK (duration_mins IS NULL OR duration_mins >= 0),
	notes TEXT NOT NULL DEFAULT '',
	callback_needed BOOL NOT NULL DEFAULT FALSE,
	callback_done_at TIMESTAMPTZ,
	callback_done_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
	message_id BIGINT REFERENCES conversation_messages(id) ON DELETE SET NULL,
	kd_sample BOOL NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS kd_calls_conversation ON kd_calls (conversation_id);
CREATE INDEX IF NOT EXISTS kd_calls_called_at ON kd_calls (called_at);
CREATE INDEX IF NOT EXISTS kd_calls_callbacks ON kd_calls (called_at) WHERE callback_needed AND callback_done_at IS NULL;
`

// Where a ticket came from: set on tickets made in the app; otherwise an RMA
// form ticket, else the inbox's channel (email). Needs conversations c and inboxes i.
const kdTicketSource = `COALESCE(c.meta->>'kd_source',
	CASE WHEN EXISTS (SELECT 1 FROM kd_rmas sr WHERE sr.conversation_id = c.id AND sr.source = 'form') THEN 'form' END,
	i.channel::text)`

var kdSourceLabels = map[string]string{
	"phone":    "Phone",
	"email":    "Email",
	"form":     "RMA form",
	"agent":    "Logged in app",
	"livechat": "Live chat",
}

type kdCall struct {
	ID             int        `db:"id" json:"id"`
	CalledAt       time.Time  `db:"called_at" json:"called_at"`
	Direction      string     `db:"direction" json:"direction"`
	Phone          string     `db:"phone" json:"phone"`
	DurationMins   *int       `db:"duration_mins" json:"duration_mins"`
	Notes          string     `db:"notes" json:"notes"`
	CallbackNeeded bool       `db:"callback_needed" json:"callback_needed"`
	CallbackDoneAt *time.Time `db:"callback_done_at" json:"callback_done_at"`
	CallbackDoneBy *string    `db:"callback_done_by" json:"callback_done_by"`
	AgentID        *int       `db:"agent_id" json:"agent_id"`
	AgentName      *string    `db:"agent_name" json:"agent_name"`
	ContactID      *int       `db:"contact_id" json:"contact_id"`
	ContactName    string     `db:"contact_name" json:"contact_name"`
	CustomerName   *string    `db:"customer_name" json:"customer_name"`
	TicketUUID     string     `db:"ticket_uuid" json:"ticket_uuid"`
	TicketRef      string     `db:"ticket_ref" json:"ticket_ref"`
	TicketSubject  string     `db:"ticket_subject" json:"ticket_subject"`
}

const kdCallSelect = `
	WITH t AS (` + kdTicketCustomer + `)
	SELECT k.id, k.called_at, k.direction, k.phone, k.duration_mins, k.notes, k.callback_needed,
		k.callback_done_at, NULLIF(TRIM(d.first_name || ' ' || COALESCE(d.last_name, '')), '') AS callback_done_by,
		k.user_id AS agent_id, NULLIF(TRIM(a.first_name || ' ' || COALESCE(a.last_name, '')), '') AS agent_name,
		k.contact_id, COALESCE(TRIM(u.first_name || ' ' || COALESCE(u.last_name, '')), '') AS contact_name,
		cu.name AS customer_name,
		c.uuid::text AS ticket_uuid, c.reference_number AS ticket_ref, COALESCE(c.subject, '') AS ticket_subject
	FROM kd_calls k
	JOIN conversations c ON c.id = k.conversation_id
	LEFT JOIN t ON t.conversation_id = c.id
	LEFT JOIN kd_customers cu ON cu.id = t.customer_id
	LEFT JOIN users u ON u.id = k.contact_id
	LEFT JOIN users a ON a.id = k.user_id
	LEFT JOIN users d ON d.id = k.callback_done_by
`

// kdPhoneDigits is the last 9 digits of a number, enough to match Irish
// numbers however they're written (087 123 4501, +353 87 123 4501).
func kdPhoneDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if len(d) > 9 {
		d = d[len(d)-9:]
	}
	return d
}

type kdCallerTicket struct {
	UUID            string `db:"uuid" json:"uuid"`
	ReferenceNumber string `db:"reference_number" json:"reference_number"`
	Subject         string `db:"subject" json:"subject"`
	Status          string `db:"status" json:"status"`
	OwnerID         int    `db:"owner_id" json:"-"`
}

type kdCaller struct {
	Kind         string           `json:"kind"` // contact | customer
	ID           int              `db:"id" json:"id"`
	Name         string           `db:"name" json:"name"`
	Email        string           `db:"email" json:"email"`
	Phone        string           `db:"phone" json:"phone"`
	CustomerID   *int             `db:"customer_id" json:"customer_id"`
	CustomerName *string          `db:"customer_name" json:"customer_name"`
	OpenTickets  []kdCallerTicket `json:"open_tickets"`
}

// GET /api/v1/kd/calls/callers?q=: contacts (and customers' main lines)
// matching a phone number, name or email, each with their open tickets.
func handleKDFindCallers(r *fastglue.Request) error {
	var (
		app    = r.Context.(*App)
		q      = strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("q")))
		digits = kdPhoneDigits(q)
	)
	if len([]rune(q)) < 2 {
		return r.SendEnvelope(map[string]any{"contacts": []kdCaller{}, "customers": []kdCaller{}})
	}
	if len(digits) < 4 {
		digits = "" // too short to mean a number
	}

	contacts := []kdCaller{}
	if err := app.db.Select(&contacts, `
		SELECT u.id, TRIM(u.first_name || ' ' || COALESCE(u.last_name, '')) AS name, COALESCE(u.email, '') AS email,
			TRIM(COALESCE(u.phone_number_country_code, '') || ' ' || COALESCE(u.phone_number, '')) AS phone,
			k.id AS customer_id, k.name AS customer_name
		FROM users u
		LEFT JOIN kd_contact_customers kc ON kc.contact_id = u.id
		LEFT JOIN kd_customers k ON k.id = kc.customer_id
		WHERE u.type = 'contact' AND u.deleted_at IS NULL
		  AND COALESCE(u.custom_attributes->>'kd_company_contact', '') <> 'true'
		  AND (($2 <> '' AND regexp_replace(COALESCE(u.phone_number_country_code, '') || COALESCE(u.phone_number, ''), '\D', '', 'g') LIKE '%' || $2 || '%')
		    OR (u.first_name || ' ' || COALESCE(u.last_name, '')) ILIKE '%' || $1 || '%'
		    OR u.email ILIKE '%' || $1 || '%')
		ORDER BY LOWER(u.first_name), u.id LIMIT 8`, q, digits); err != nil {
		return kdServerError(r, err)
	}

	customers := []kdCaller{}
	if err := app.db.Select(&customers, `
		SELECT k.id, k.name, '' AS email, k.phone, k.id AS customer_id, k.name AS customer_name
		FROM kd_customers k
		WHERE ($2 <> '' AND regexp_replace(k.phone, '\D', '', 'g') LIKE '%' || $2 || '%')
		   OR k.name ILIKE '%' || $1 || '%'
		ORDER BY LOWER(k.name) LIMIT 5`, q, digits); err != nil {
		return kdServerError(r, err)
	}

	// Open tickets: the contact's own, or anything for the customer.
	for i := range contacts {
		contacts[i].Kind = "contact"
		if err := app.db.Select(&contacts[i].OpenTickets, `
			SELECT c.uuid::text AS uuid, c.reference_number, COALESCE(c.subject, '') AS subject, s.name AS status, 0 AS owner_id
			FROM conversations c JOIN conversation_statuses s ON s.id = c.status_id
			WHERE c.contact_id = $1 AND s.category <> 'resolved'
			ORDER BY COALESCE(c.last_message_at, c.updated_at) DESC LIMIT 5`, contacts[i].ID); err != nil {
			return kdServerError(r, err)
		}
	}
	for i := range customers {
		customers[i].Kind = "customer"
		if err := app.db.Select(&customers[i].OpenTickets, `
			WITH t AS (`+kdTicketCustomer+`)
			SELECT c.uuid::text AS uuid, c.reference_number, COALESCE(c.subject, '') AS subject, s.name AS status, 0 AS owner_id
			FROM t JOIN conversations c ON c.id = t.conversation_id
			JOIN conversation_statuses s ON s.id = c.status_id
			WHERE t.customer_id = $1 AND s.category <> 'resolved'
			ORDER BY COALESCE(c.last_message_at, c.updated_at) DESC LIMIT 5`, customers[i].ID); err != nil {
			return kdServerError(r, err)
		}
	}
	return r.SendEnvelope(map[string]any{"contacts": contacts, "customers": customers})
}

type kdLogCallReq struct {
	ConversationUUID string `json:"conversation_uuid"` // log on this ticket; else a new one
	ContactID        int    `json:"contact_id"`
	CustomerID       int    `json:"customer_id"`
	// A caller who isn't a contact yet
	NewCallerName string `json:"new_caller_name"`
	Phone         string `json:"phone"`
	Direction     string `json:"direction"`
	DurationMins  *int   `json:"duration_mins"`
	Notes         string `json:"notes"`
	Callback      bool   `json:"callback_needed"`
	Subject       string `json:"subject"` // new ticket only; defaults from the notes
}

// POST /api/v1/kd/calls: log a call on a ticket, or as a new ticket.
func handleKDLogCall(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   kdLogCallReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	req.Notes = strings.TrimSpace(strings.ReplaceAll(req.Notes, "\r\n", "\n"))
	req.Phone = strings.Join(strings.Fields(req.Phone), " ")
	req.NewCallerName = strings.Join(strings.Fields(req.NewCallerName), " ")
	req.Subject = strings.Join(strings.Fields(req.Subject), " ")
	if req.Direction == "" {
		req.Direction = "incoming"
	}
	switch {
	case req.Direction != "incoming" && req.Direction != "outgoing":
		return kdBadRequest(r, "Direction is incoming or outgoing")
	case req.Notes == "":
		return kdBadRequest(r, "Add what the call was about")
	case len(req.Notes) > 10000:
		return kdBadRequest(r, "Notes are too long")
	case len(req.Subject) > 300 || len(req.Phone) > 50 || len(req.NewCallerName) > 200:
		return kdBadRequest(r, "One of the fields is too long")
	case req.DurationMins != nil && (*req.DurationMins < 0 || *req.DurationMins > 600):
		return kdBadRequest(r, "Call length should be in minutes")
	}

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	var (
		uuid      string
		contactID = req.ContactID
		newTicket = req.ConversationUUID == ""
	)
	if !newTicket {
		conv, err := enforceConversationAccess(app, req.ConversationUUID, user)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}
		uuid, contactID = conv.UUID, conv.ContactID
	} else {
		if contactID, req.CustomerID, err = kdCallRequester(app, req); err != nil {
			return sendErrorEnvelope(r, err)
		}
		subject := req.Subject
		if subject == "" {
			subject = "Phone call: " + kdFirstLine(req.Notes, 80)
		}
		body := kdCallHTML(req.Direction, req.Phone, req.DurationMins, req.Notes, req.Callback,
			"Logged by "+strings.TrimSpace(auser.FirstName+" "+auser.LastName))
		if uuid, err = kdLogTicketFrom(app, "phone", contactID, req.CustomerID, subject, body, auser.ID); err != nil {
			return sendErrorEnvelope(r, err)
		}
		// Like New ticket and emailed tickets: the Support group with High
		// priority (when they exist). Team first - assigning a team clears the
		// agent - then whoever took the call owns the follow-up.
		var teamID, priorityID int
		if err := app.db.Get(&teamID, `SELECT id FROM teams WHERE LOWER(name) = 'support' ORDER BY id LIMIT 1`); err == nil {
			if err := app.conversation.UpdateConversationTeamAssignee(uuid, teamID, user); err != nil {
				app.lo.Error("kd calls: error setting group", "error", err)
			}
		}
		if err := app.conversation.UpdateConversationUserAssignee(uuid, auser.ID, user); err != nil {
			app.lo.Error("kd calls: error assigning ticket", "error", err)
		}
		if err := app.db.Get(&priorityID, `SELECT id FROM conversation_priorities WHERE LOWER(name) = 'high' ORDER BY id LIMIT 1`); err == nil {
			if err := app.conversation.UpdateConversationPriority(uuid, priorityID, "", user); err != nil {
				app.lo.Error("kd calls: error setting priority", "error", err)
			}
		}
	}

	// The call itself: on a new ticket its description already says it all;
	// on an existing ticket it's a note in the conversation.
	var messageID any
	if newTicket {
		var id int
		if err := app.db.Get(&id, `SELECT m.id FROM conversation_messages m JOIN conversations c ON c.id = m.conversation_id
			WHERE c.uuid = $1 ORDER BY m.id LIMIT 1`, uuid); err == nil {
			messageID = id
		}
	} else {
		msg, err := app.conversation.SendPrivateNote(nil, auser.ID, uuid,
			kdCallHTML(req.Direction, req.Phone, req.DurationMins, req.Notes, req.Callback, ""), nil)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}
		messageID = msg.ID
	}

	var callID int
	if err := app.db.Get(&callID, `
		INSERT INTO kd_calls (conversation_id, contact_id, user_id, direction, phone, duration_mins, notes, callback_needed, message_id)
		SELECT c.id, $2, $3, $4, $5, $6, $7, $8, $9 FROM conversations c WHERE c.uuid = $1
		RETURNING id`, uuid, kdNullInt(contactID), auser.ID, req.Direction, req.Phone, req.DurationMins,
		req.Notes, req.Callback, messageID); err != nil {
		return kdServerError(r, err)
	}
	call, err := kdGetCall(app, callID)
	if err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(map[string]any{"call": call, "new_ticket": newTicket})
}

// kdCallRequester picks the new ticket's requester: the chosen contact, a new
// contact for a first-time caller, or the customer's company contact.
// Returns the contact and the customer the ticket goes under.
func kdCallRequester(app *App, req kdLogCallReq) (int, int, error) {
	customerID := max(req.CustomerID, 0)
	var customerName string
	if customerID > 0 {
		err := app.db.Get(&customerName, `SELECT name FROM kd_customers WHERE id = $1`, customerID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, kdInputErr("That customer no longer exists")
		}
		if err != nil {
			app.lo.Error("kd calls: customer", "error", err)
			return 0, 0, kdErr(app)
		}
	}

	switch {
	case req.ContactID > 0:
		var linked sql.NullInt64
		err := app.db.Get(&linked, `SELECT kc.customer_id FROM users u LEFT JOIN kd_contact_customers kc ON kc.contact_id = u.id
			WHERE u.id = $1 AND u.type = 'contact' AND u.deleted_at IS NULL`, req.ContactID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, kdInputErr("That contact no longer exists")
		}
		if err != nil {
			app.lo.Error("kd calls: contact", "error", err)
			return 0, 0, kdErr(app)
		}
		if customerID == 0 && linked.Valid {
			customerID = int(linked.Int64)
		}
		return req.ContactID, customerID, nil

	case req.NewCallerName != "":
		first, last, _ := strings.Cut(req.NewCallerName, " ")
		contact := umodels.User{FirstName: first, LastName: last}
		if req.Phone != "" {
			contact.PhoneNumber = null.StringFrom(req.Phone)
		}
		if err := app.user.CreateContact(&contact); err != nil {
			return 0, 0, err
		}
		if customerID > 0 {
			if _, err := app.db.Exec(`INSERT INTO kd_contact_customers (contact_id, customer_id) VALUES ($1, $2)
				ON CONFLICT (contact_id) DO NOTHING`, contact.ID, customerID); err != nil {
				app.lo.Error("kd calls: link new caller", "error", err)
			}
		}
		return contact.ID, customerID, nil

	case customerID > 0:
		id, err := kdCompanyContact(app, customerID, customerName)
		return id, customerID, err
	}
	return 0, 0, kdInputErr("Choose who called: a contact, a customer, or type a new caller's name")
}

// kdCallHTML is how a call reads in the conversation.
func kdCallHTML(direction, phone string, mins *int, notes string, callback bool, footer string) string {
	head := "📞 Incoming call"
	if direction == "outgoing" {
		head = "📞 Outgoing call"
	}
	parts := []string{"<strong>" + head + "</strong>"}
	if mins != nil && *mins > 0 {
		parts = append(parts, strconv.Itoa(*mins)+" min")
	}
	if phone != "" {
		parts = append(parts, html.EscapeString(phone))
	}
	out := "<p>" + strings.Join(parts, " · ") + "</p><p>" +
		strings.ReplaceAll(html.EscapeString(notes), "\n", "<br>") + "</p>"
	if callback {
		out += "<p><strong>Call back needed</strong></p>"
	}
	if footer != "" {
		out += "<p><em>" + html.EscapeString(footer) + "</em></p>"
	}
	return out
}

func kdFirstLine(s string, n int) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	r := []rune(strings.TrimSpace(line))
	if len(r) > n {
		return strings.TrimSpace(string(r[:n])) + "…"
	}
	return string(r)
}

func kdGetCall(app *App, id int) (kdCall, error) {
	var c kdCall
	err := app.db.Get(&c, kdCallSelect+` WHERE k.id = $1`, id)
	return c, err
}

// GET /api/v1/kd/calls?callback=open&limit=: recent calls; callback=open
// gives the call-backs still to make, oldest first.
func handleKDGetCalls(r *fastglue.Request) error {
	var (
		app      = r.Context.(*App)
		args     = r.RequestCtx.QueryArgs()
		open     = string(args.Peek("callback")) == "open"
		limit, _ = strconv.Atoi(string(args.Peek("limit")))
		out      = []kdCall{}
	)
	if limit < 1 || limit > 200 {
		limit = 50
	}
	where, order := "", "k.called_at DESC"
	if open {
		where, order = " WHERE k.callback_needed AND k.callback_done_at IS NULL", "k.called_at"
	}
	if err := app.db.Select(&out, kdCallSelect+where+fmt.Sprintf(" ORDER BY %s LIMIT %d", order, limit)); err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(out)
}

// GET /api/v1/conversations/{uuid}/calls: the ticket's calls, newest first.
func handleKDGetTicketCalls(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		out   = []kdCall{}
	)
	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conv, err := enforceConversationAccess(app, r.RequestCtx.UserValue("uuid").(string), user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := app.db.Select(&out, kdCallSelect+` WHERE k.conversation_id = $1 ORDER BY k.called_at DESC`, conv.ID); err != nil {
		return kdServerError(r, err)
	}
	return r.SendEnvelope(out)
}

// POST /api/v1/kd/calls/{id}/callback-done: the call-back was made.
func handleKDCallbackDone(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		id    = kdPathID(r, "id")
	)
	res, err := app.db.Exec(`UPDATE kd_calls SET callback_done_at = NOW(), callback_done_by = $2
		WHERE id = $1 AND callback_needed AND callback_done_at IS NULL`, id, auser.ID)
	if err != nil {
		return kdServerError(r, err)
	}
	call, err := kdGetCall(app, id)
	if errors.Is(err, sql.ErrNoRows) {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Call not found", nil, envelope.NotFoundError)
	}
	if err != nil {
		return kdServerError(r, err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		who := call.ContactName
		if who == "" {
			who = "the caller"
		}
		kdTicketActivity(app, call.TicketUUID, auser, "called back "+who)
	}
	return r.SendEnvelope(call)
}
