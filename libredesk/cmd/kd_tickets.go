package main

// KD: agent-created tickets. Unlike Libredesk's "new conversation" (an
// outgoing email), this logs a ticket straight into the helpdesk: a subject,
// a description, a customer and/or contact, status, priority and owner.
// Nothing is emailed. The description is stored as the requester's first
// message, as if they had written in.

import (
	"database/sql"
	"errors"
	"html"
	"strings"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/zerodha/fastglue"
)

type kdCreateTicketReq struct {
	Subject    string `json:"subject"`
	Body       string `json:"body"` // plain text
	CustomerID int    `json:"customer_id"`
	ContactID  int    `json:"contact_id"`
	StatusID   int    `json:"status_id"`
	PriorityID int    `json:"priority_id"`
	TeamID     int    `json:"team_id"`
	AgentID    int    `json:"agent_id"`
}

// POST /api/v1/kd/tickets
func handleKDCreateTicket(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   kdCreateTicketReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return kdBadRequest(r, app.i18n.T("errors.parsingRequest"))
	}
	req.Subject = strings.TrimSpace(req.Subject)
	req.Body = strings.TrimSpace(req.Body)
	switch {
	case req.Subject == "":
		return kdBadRequest(r, "Add a subject")
	case len(req.Subject) > 300:
		return kdBadRequest(r, "Subject is too long")
	case req.Body == "":
		return kdBadRequest(r, "Add a description")
	case req.CustomerID <= 0 && req.ContactID <= 0:
		return kdBadRequest(r, "Choose a customer or a contact")
	}

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Customer, if given, must exist.
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

	inboxID, err := kdDefaultInbox(app)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Requester: the chosen contact, or the customer's own company contact.
	contactID := req.ContactID
	if contactID > 0 {
		var ok bool
		if err := app.db.Get(&ok, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND type = 'contact' AND deleted_at IS NULL)`, contactID); err != nil {
			return kdServerError(r, err)
		}
		if !ok {
			return kdBadRequest(r, "That contact no longer exists")
		}
	} else {
		if contactID, err = kdCompanyContact(app, req.CustomerID, customerName); err != nil {
			return sendErrorEnvelope(r, err)
		}
	}

	_, uuid, err := app.conversation.CreateConversation(contactID, inboxID, "", time.Now(), req.Subject, false, nil, nil, 0, 0)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := app.conversation.CreateContactMessage(nil, contactID, uuid, kdTextToHTML(req.Body), cmodels.ContentTypeHTML, true, "", auser.ID); err != nil {
		app.lo.Error("kd tickets: error adding description", "error", err)
		_ = app.conversation.DeleteConversation(uuid)
		return sendErrorEnvelope(r, err)
	}

	// Customer link: on the ticket, and on the contact if they have none yet.
	if req.CustomerID > 0 {
		if _, err := app.db.Exec(`
			WITH t AS (
				INSERT INTO kd_conversation_customers (conversation_id, customer_id)
				SELECT id, $2 FROM conversations WHERE uuid = $1
				ON CONFLICT (conversation_id) DO UPDATE SET customer_id = EXCLUDED.customer_id
			)
			INSERT INTO kd_contact_customers (contact_id, customer_id) VALUES ($3, $2)
			ON CONFLICT (contact_id) DO NOTHING`, uuid, req.CustomerID, contactID); err != nil {
			app.lo.Error("kd tickets: error linking customer", "error", err)
		}
	}

	// Owner, status and priority. Team assignment clears the agent, so it goes first.
	if req.TeamID > 0 {
		if err := app.conversation.UpdateConversationTeamAssignee(uuid, req.TeamID, user); err != nil {
			app.lo.Error("kd tickets: error assigning team", "error", err)
		}
	}
	if req.AgentID > 0 {
		if err := app.conversation.UpdateConversationUserAssignee(uuid, req.AgentID, user); err != nil {
			app.lo.Error("kd tickets: error assigning agent", "error", err)
		}
	}
	if req.PriorityID > 0 {
		if err := app.conversation.UpdateConversationPriority(uuid, req.PriorityID, "", user); err != nil {
			app.lo.Error("kd tickets: error setting priority", "error", err)
		}
	}
	if req.StatusID > 0 {
		if err := app.conversation.UpdateConversationStatus(uuid, req.StatusID, "", "", user); err != nil {
			app.lo.Error("kd tickets: error setting status", "error", err)
		}
	}

	conv, err := app.conversation.GetConversation(0, uuid, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(conv)
}

// kdCompanyContact returns the contact that stands for a customer itself, for
// tickets raised against the company rather than a person. Made on first use.
func kdCompanyContact(app *App, customerID int, name string) (int, error) {
	var id int
	err := app.db.Get(&id, `
		SELECT u.id FROM kd_contact_customers kc JOIN users u ON u.id = kc.contact_id
		WHERE kc.customer_id = $1 AND u.deleted_at IS NULL
		  AND u.custom_attributes->>'kd_company_contact' = 'true'
		ORDER BY u.id LIMIT 1`, customerID)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		app.lo.Error("kd tickets: error finding company contact", "error", err)
		return 0, kdErr(app)
	}

	contact := umodels.User{FirstName: name}
	if err := app.user.CreateContact(&contact); err != nil {
		return 0, err
	}
	if _, err := app.db.Exec(`
		WITH u AS (
			UPDATE users SET custom_attributes = COALESCE(custom_attributes, '{}'::jsonb) || '{"kd_company_contact": true}'::jsonb
			WHERE id = $1
		)
		INSERT INTO kd_contact_customers (contact_id, customer_id) VALUES ($1, $2)
		ON CONFLICT (contact_id) DO UPDATE SET customer_id = EXCLUDED.customer_id`, contact.ID, customerID); err != nil {
		app.lo.Error("kd tickets: error linking company contact", "error", err)
		return 0, kdErr(app)
	}
	return contact.ID, nil
}

// kdDefaultInbox picks the inbox agent-created tickets live in: an email
// inbox, preferring enabled and real ones over sample data. A disabled inbox
// is fine - nothing is sent when a ticket is logged.
func kdDefaultInbox(app *App) (int, error) {
	var id int
	err := app.db.Get(&id, `
		SELECT id FROM inboxes WHERE deleted_at IS NULL
		ORDER BY (channel <> 'email'), (NOT enabled), (name ILIKE '%sample%'), id LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, kdInputErr("Add an inbox first (Admin → Inboxes)")
	}
	if err != nil {
		app.lo.Error("kd tickets: error finding inbox", "error", err)
		return 0, kdErr(app)
	}
	return id, nil
}

// kdTextToHTML turns a plain-text description into safe HTML paragraphs.
func kdTextToHTML(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	for _, para := range strings.Split(s, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(html.EscapeString(para), "\n", "<br>"))
		b.WriteString("</p>")
	}
	return b.String()
}

func kdErr(app *App) error {
	return envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil)
}

func kdInputErr(msg string) error {
	return envelope.NewError(envelope.InputError, msg, nil)
}
