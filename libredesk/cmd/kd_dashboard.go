package main

// KD: one-call dashboard - ticket totals, per-agent workload and a daily trend.

import (
	"strconv"
	"time"

	"github.com/zerodha/fastglue"
)

type kdDashTotals struct {
	Open             int      `db:"open" json:"open"`
	Unassigned       int      `db:"unassigned" json:"unassigned"`
	Waiting          int      `db:"waiting" json:"waiting"`
	Created          int      `db:"created" json:"created"`
	Solved           int      `db:"solved" json:"solved"`
	AvgFirstResponse *float64 `db:"avg_first_response" json:"avg_first_response_secs"`
	AvgResolution    *float64 `db:"avg_resolution" json:"avg_resolution_secs"`
}

type kdDashAgent struct {
	ID               int      `db:"id" json:"id"`
	Name             string   `db:"name" json:"name"`
	AvatarURL        *string  `db:"avatar_url" json:"avatar_url"`
	Open             int      `db:"open" json:"open"`
	Waiting          int      `db:"waiting" json:"waiting"`
	Solved           int      `db:"solved" json:"solved"`
	AvgFirstResponse *float64 `db:"avg_first_response" json:"avg_first_response_secs"`
}

type kdDashDay struct {
	Day     time.Time `db:"day" json:"day"`
	Created int       `db:"created" json:"created"`
	Solved  int       `db:"solved" json:"solved"`
}

type kdDashCount struct {
	Name  string `db:"name" json:"name"`
	Count int    `db:"count" json:"count"`
}

// GET /api/v1/kd/dashboard?days=30
func handleKDDashboard(r *fastglue.Request) error {
	app := r.Context.(*App)
	days, _ := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("days")))
	if days < 1 || days > 365 {
		days = 30
	}

	var totals kdDashTotals
	if err := app.db.Get(&totals, `
		SELECT
			COUNT(*) FILTER (WHERE s.category <> 'resolved') AS open,
			COUNT(*) FILTER (WHERE s.category <> 'resolved' AND c.assigned_user_id IS NULL) AS unassigned,
			COUNT(*) FILTER (WHERE s.category = 'waiting') AS waiting,
			COUNT(*) FILTER (WHERE c.created_at >= NOW() - make_interval(days => $1)) AS created,
			COUNT(*) FILTER (WHERE c.resolved_at >= NOW() - make_interval(days => $1)) AS solved,
			AVG(EXTRACT(EPOCH FROM c.first_reply_at - c.created_at))
				FILTER (WHERE c.first_reply_at IS NOT NULL AND c.created_at >= NOW() - make_interval(days => $1)) AS avg_first_response,
			AVG(EXTRACT(EPOCH FROM c.resolved_at - c.created_at))
				FILTER (WHERE c.resolved_at >= NOW() - make_interval(days => $1)) AS avg_resolution
		FROM conversations c JOIN conversation_statuses s ON s.id = c.status_id`, days); err != nil {
		return kdServerError(r, err)
	}

	agents := []kdDashAgent{}
	if err := app.db.Select(&agents, `
		SELECT u.id, TRIM(u.first_name || ' ' || COALESCE(u.last_name, '')) AS name, u.avatar_url,
			COUNT(c.id) FILTER (WHERE s.category = 'open') AS open,
			COUNT(c.id) FILTER (WHERE s.category = 'waiting') AS waiting,
			COUNT(c.id) FILTER (WHERE c.resolved_at >= NOW() - make_interval(days => $1)) AS solved,
			AVG(EXTRACT(EPOCH FROM c.first_reply_at - c.created_at))
				FILTER (WHERE c.first_reply_at IS NOT NULL AND c.created_at >= NOW() - make_interval(days => $1)) AS avg_first_response
		FROM users u
		LEFT JOIN conversations c ON c.assigned_user_id = u.id
		LEFT JOIN conversation_statuses s ON s.id = c.status_id
		WHERE u.type = 'agent' AND u.deleted_at IS NULL AND u.enabled
			-- people on the support team, not integration logins like "RMA Form"
			AND (EXISTS (SELECT 1 FROM team_members tm WHERE tm.user_id = u.id)
				OR EXISTS (SELECT 1 FROM conversations ac WHERE ac.assigned_user_id = u.id))
		GROUP BY u.id
		ORDER BY open DESC, waiting DESC, LOWER(u.first_name)`, days); err != nil {
		return kdServerError(r, err)
	}

	daily := []kdDashDay{}
	if err := app.db.Select(&daily, `
		WITH d AS (
			SELECT generate_series(date_trunc('day', NOW()) - make_interval(days => $1 - 1), date_trunc('day', NOW()), '1 day') AS day
		)
		SELECT d.day,
			(SELECT COUNT(*) FROM conversations c WHERE c.created_at >= d.day AND c.created_at < d.day + INTERVAL '1 day') AS created,
			(SELECT COUNT(*) FROM conversations c WHERE c.resolved_at >= d.day AND c.resolved_at < d.day + INTERVAL '1 day') AS solved
		FROM d ORDER BY d.day`, days); err != nil {
		return kdServerError(r, err)
	}

	byStatus := []kdDashCount{}
	if err := app.db.Select(&byStatus, `
		SELECT s.name, COUNT(c.id) AS count FROM conversation_statuses s
		LEFT JOIN conversations c ON c.status_id = s.id
		GROUP BY s.id ORDER BY s.id`); err != nil {
		return kdServerError(r, err)
	}

	byPriority := []kdDashCount{}
	if err := app.db.Select(&byPriority, `
		SELECT COALESCE(p.name, 'None') AS name, COUNT(*) AS count
		FROM conversations c JOIN conversation_statuses s ON s.id = c.status_id
		LEFT JOIN conversation_priorities p ON p.id = c.priority_id
		WHERE s.category <> 'resolved'
		GROUP BY p.id, p.name ORDER BY p.id NULLS LAST`); err != nil {
		return kdServerError(r, err)
	}

	byCustomer := []kdDashCount{}
	if err := app.db.Select(&byCustomer, `
		WITH t AS (`+kdTicketCustomer+`)
		SELECT k.name, COUNT(*) AS count
		FROM t JOIN kd_customers k ON k.id = t.customer_id
		JOIN conversations c ON c.id = t.conversation_id
		JOIN conversation_statuses s ON s.id = c.status_id
		WHERE s.category <> 'resolved'
		GROUP BY k.id, k.name ORDER BY count DESC, LOWER(k.name) LIMIT 8`); err != nil {
		return kdServerError(r, err)
	}

	rmas, err := kdDashboardRmas(app, days)
	if err != nil {
		return kdServerError(r, err)
	}

	return r.SendEnvelope(map[string]any{
		"days":        days,
		"totals":      totals,
		"agents":      agents,
		"daily":       daily,
		"by_status":   byStatus,
		"by_priority": byPriority,
		"by_customer": byCustomer,
		"rmas":        rmas,
	})
}

type kdDashRmaTotals struct {
	Open          int      `db:"open" json:"open"`
	Requested     int      `db:"requested" json:"requested"`
	Approved      int      `db:"approved" json:"approved"`
	WithSupplier  int      `db:"with_supplier" json:"with_supplier"`
	Created       int      `db:"created" json:"created"`
	Resolved      int      `db:"resolved" json:"resolved"`
	NoFaultFound  int      `db:"no_fault_found" json:"no_fault_found"`
	Tested        int      `db:"tested" json:"tested"`
	AvgTurnaround *float64 `db:"avg_turnaround" json:"avg_turnaround_secs"`
}

// kdDashboardRmas: open RMAs by step, and the period's volume by product,
// supplier and outcome. Volume counts RMAs opened in the period; outcomes count
// RMAs closed in it.
func kdDashboardRmas(app *App, days int) (map[string]any, error) {
	var totals kdDashRmaTotals
	if err := app.db.Get(&totals, `
		SELECT
			COUNT(*) FILTER (WHERE status NOT IN ('resolved', 'rejected')) AS open,
			COUNT(*) FILTER (WHERE status = 'requested') AS requested,
			COUNT(*) FILTER (WHERE status = 'approved') AS approved,
			COUNT(*) FILTER (WHERE status = 'with_supplier') AS with_supplier,
			COUNT(*) FILTER (WHERE created_at >= NOW() - make_interval(days => $1)) AS created,
			COUNT(*) FILTER (WHERE status = 'resolved' AND resolved_at >= NOW() - make_interval(days => $1)) AS resolved,
			COUNT(*) FILTER (WHERE test_result = 'no_fault_found' AND created_at >= NOW() - make_interval(days => $1)) AS no_fault_found,
			COUNT(*) FILTER (WHERE test_result <> '' AND created_at >= NOW() - make_interval(days => $1)) AS tested,
			AVG(EXTRACT(EPOCH FROM resolved_at - created_at))
				FILTER (WHERE status IN ('resolved', 'rejected') AND resolved_at >= NOW() - make_interval(days => $1)) AS avg_turnaround
		FROM kd_rmas`, days); err != nil {
		return nil, err
	}

	byStatus := []kdDashCount{}
	if err := app.db.Select(&byStatus, `
		SELECT status AS name, COUNT(*) AS count FROM kd_rmas
		WHERE status NOT IN ('resolved', 'rejected')
		GROUP BY status
		ORDER BY array_position(ARRAY['requested', 'approved', 'received', 'with_supplier'], status)`); err != nil {
		return nil, err
	}

	byProduct := []kdDashCount{}
	if err := app.db.Select(&byProduct, `
		SELECT CASE WHEN brand = '' OR product ILIKE brand || '%' THEN product ELSE brand || ' ' || product END AS name,
			SUM(quantity) AS count
		FROM kd_rmas WHERE created_at >= NOW() - make_interval(days => $1)
		GROUP BY 1 ORDER BY count DESC, name LIMIT 8`, days); err != nil {
		return nil, err
	}

	bySupplier := []kdDashCount{}
	if err := app.db.Select(&bySupplier, `
		SELECT COALESCE(NULLIF(supplier, ''), 'Not set') AS name, SUM(quantity) AS count
		FROM kd_rmas WHERE created_at >= NOW() - make_interval(days => $1)
		GROUP BY 1 ORDER BY count DESC, name LIMIT 8`, days); err != nil {
		return nil, err
	}

	byOutcome := []kdDashCount{}
	if err := app.db.Select(&byOutcome, `
		SELECT CASE WHEN status = 'rejected' THEN 'rejected' ELSE outcome END AS name, COUNT(*) AS count
		FROM kd_rmas
		WHERE status IN ('resolved', 'rejected') AND resolved_at >= NOW() - make_interval(days => $1)
		GROUP BY 1 ORDER BY count DESC`, days); err != nil {
		return nil, err
	}

	return map[string]any{
		"totals":      totals,
		"by_status":   byStatus,
		"by_product":  byProduct,
		"by_supplier": bySupplier,
		"by_outcome":  byOutcome,
	}, nil
}
