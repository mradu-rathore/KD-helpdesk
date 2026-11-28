package main

// KD: export for HubSpot. Four CSV files whose column names match HubSpot
// properties, so its import tool maps them on its own. Import them in this
// order: companies, contacts, tickets, RMAs (tickets and RMAs link to the
// contact by email and to the company by domain). See docs/hubspot-migration.md.

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/zerodha/fastglue"
)

// Libredesk status -> HubSpot's default support pipeline status.
var kdHubSpotTicketStatus = map[string]string{
	"Open":     "Waiting on us",
	"On-hold":  "Waiting on us",
	"Pending":  "Waiting on contact",
	"Snoozed":  "Waiting on contact",
	"Resolved": "Closed",
	"Closed":   "Closed",
}

type kdExport struct {
	header []string
	query  string
}

// What the thread looks like in HubSpot's ticket description: every message
// (not activity lines), oldest first, with internal notes marked.
const kdExportThread = `
	(SELECT string_agg(
		to_char(m.created_at AT TIME ZONE 'Europe/Dublin', 'DD Mon YYYY HH24:MI') || ' - ' ||
		COALESCE(NULLIF(TRIM(su.first_name || ' ' || COALESCE(su.last_name, '')), ''), 'Unknown') ||
		CASE WHEN m.private THEN ' (internal note)' ELSE '' END || ':' || E'\n' ||
		TRIM(COALESCE(NULLIF(m.text_content, ''), regexp_replace(m.content, '<[^>]+>', ' ', 'g'))),
		E'\n\n' ORDER BY m.created_at, m.id)
	 FROM conversation_messages m LEFT JOIN users su ON su.id = m.sender_id
	 WHERE m.conversation_id = c.id AND m.type <> 'activity')`

var kdExports = map[string]kdExport{
	"companies": {
		header: []string{"Name", "Company Domain Name", "Phone Number", "Description", "KD customer ID"},
		query:  `SELECT name, domain, phone, notes, id::text FROM kd_customers ORDER BY LOWER(name)`,
	},
	"contacts": {
		header: []string{"First Name", "Last Name", "Email", "Phone Number", "Company Name", "Company Domain Name", "KD contact ID"},
		// Contacts standing for a company itself (made by New ticket) stay out: the company covers them.
		query: `
			SELECT u.first_name, COALESCE(u.last_name, ''), COALESCE(u.email, ''),
				TRIM(COALESCE(u.phone_number_country_code, '') || ' ' || COALESCE(u.phone_number, '')),
				COALESCE(k.name, ''), COALESCE(k.domain, ''), u.id::text
			FROM users u
			LEFT JOIN kd_contact_customers kc ON kc.contact_id = u.id
			LEFT JOIN kd_customers k ON k.id = kc.customer_id
			WHERE u.type = 'contact' AND u.deleted_at IS NULL
			  AND COALESCE(u.custom_attributes->>'kd_company_contact', '') <> 'true'
			ORDER BY LOWER(u.first_name), LOWER(COALESCE(u.last_name, '')), u.id`,
	},
	"tickets": {
		header: []string{"Ticket name", "Pipeline", "Ticket status", "Priority", "Source", "Create date", "Close date",
			"Ticket owner", "Ticket description", "Email", "Company Domain Name", "KD ticket number", "KD status"},
		query: `
			WITH t AS (` + kdTicketCustomer + `)
			SELECT COALESCE(NULLIF(c.subject, ''), '(no subject)'), 'Support Pipeline', s.name,
				COALESCE(p.name, ''), ` + kdTicketSource + `, to_char(c.created_at, 'YYYY-MM-DD'),
				COALESCE(to_char(c.resolved_at, 'YYYY-MM-DD'), ''),
				COALESCE(a.email, ''), COALESCE(` + kdExportThread + `, ''),
				CASE WHEN COALESCE(cu.custom_attributes->>'kd_company_contact', '') = 'true' THEN '' ELSE COALESCE(cu.email, '') END,
				COALESCE(k.domain, ''), c.reference_number, s.name
			FROM conversations c
			JOIN inboxes i ON i.id = c.inbox_id
			JOIN conversation_statuses s ON s.id = c.status_id
			LEFT JOIN conversation_priorities p ON p.id = c.priority_id
			LEFT JOIN users a ON a.id = c.assigned_user_id
			LEFT JOIN users cu ON cu.id = c.contact_id
			LEFT JOIN t ON t.conversation_id = c.id
			LEFT JOIN kd_customers k ON k.id = t.customer_id
			ORDER BY c.created_at, c.id`,
	},
	"rmas": {
		header: []string{"Ticket name", "Pipeline", "Ticket status", "Create date", "Close date", "Ticket owner",
			"Ticket description", "Email", "Company Domain Name", "RMA number", "Brand", "Product", "Quantity",
			"Serial numbers", "Invoice number", "Purchase date", "Warranty", "Test result", "Supplier",
			"Supplier RMA number", "Outcome", "Credit note number", "Replacement order number",
			"Return tracking number", "KD ticket number"},
		query: `
			SELECT r.number || ': ' || r.quantity || ' × ' ||
					CASE WHEN r.brand = '' OR r.product ILIKE r.brand || '%' THEN r.product ELSE r.brand || ' ' || r.product END,
				'RMA', r.status, to_char(r.created_at, 'YYYY-MM-DD'), COALESCE(to_char(r.resolved_at, 'YYYY-MM-DD'), ''),
				COALESCE(a.email, ''), r.fault, COALESCE(u.email, ''), COALESCE(k.domain, ''),
				r.number, r.brand, r.product, r.quantity::text, replace(r.serial_numbers, E'\n', ', '),
				r.invoice_number, COALESCE(to_char(r.purchase_date, 'YYYY-MM-DD'), ''), r.warranty, r.test_result,
				r.supplier, r.supplier_rma_ref, r.outcome, r.credit_note_number, r.replacement_order_number,
				r.return_tracking, COALESCE(c.reference_number, '')
			FROM kd_rmas r
			LEFT JOIN conversations c ON c.id = r.conversation_id
			LEFT JOIN users a ON a.id = c.assigned_user_id
			LEFT JOIN users u ON u.id = r.contact_id
			LEFT JOIN kd_customers k ON k.id = r.customer_id
			ORDER BY r.id`,
	},
}

// GET /api/v1/kd/export/hubspot/{kind}: kind is companies, contacts, tickets or rmas.
func handleKDExportHubSpot(r *fastglue.Request) error {
	app := r.Context.(*App)
	kind := r.RequestCtx.UserValue("kind").(string)
	exp, ok := kdExports[kind]
	if !ok {
		return kdBadRequest(r, "Export companies, contacts, tickets or rmas")
	}

	rows, err := app.db.Query(exp.query)
	if err != nil {
		return kdServerError(r, err)
	}
	defer rows.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write(exp.header)
	record := make([]sql.NullString, len(exp.header))
	dest := make([]any, len(record))
	for i := range record {
		dest[i] = &record[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return kdServerError(r, err)
		}
		out := make([]string, len(record))
		for i, v := range record {
			out[i] = v.String
		}
		kdHubSpotValues(kind, exp.header, out)
		_ = w.Write(out)
	}
	if err := rows.Err(); err != nil {
		return kdServerError(r, err)
	}
	w.Flush()

	r.RequestCtx.SetContentType("text/csv; charset=utf-8")
	r.RequestCtx.Response.Header.Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="kd-hubspot-%s-%s.csv"`, kind, time.Now().Format("2006-01-02")))
	r.RequestCtx.Response.Header.Set("Cache-Control", "no-store")
	r.RequestCtx.SetBody(buf.Bytes())
	return nil
}

// kdHubSpotValues turns KD's values into the words HubSpot (and people) expect.
func kdHubSpotValues(kind string, header, row []string) {
	col := func(name string) int {
		for i, h := range header {
			if h == name {
				return i
			}
		}
		return -1
	}
	switch kind {
	case "tickets":
		// HubSpot's ticket Source options are Phone, Email, Form and Chat.
		if i := col("Source"); i >= 0 {
			row[i] = map[string]string{"phone": "Phone", "email": "Email", "form": "Form", "livechat": "Chat"}[row[i]]
		}
		if i := col("Ticket status"); i >= 0 {
			if s, ok := kdHubSpotTicketStatus[row[i]]; ok {
				row[i] = s
			} else {
				row[i] = "Waiting on us"
			}
		}
	case "rmas":
		for name, labels := range map[string]map[string]string{
			"Ticket status": kdRmaStatusLabels,
			"Warranty":      kdRmaWarrantyLabels,
			"Test result":   kdRmaTestLabels,
			"Outcome":       kdRmaOutcomeLabels,
		} {
			if i := col(name); i >= 0 && row[i] != "" {
				row[i] = labels[row[i]]
			}
		}
		if i := col("Ticket description"); i >= 0 {
			row[i] = strings.TrimSpace(row[i])
		}
	}
}
