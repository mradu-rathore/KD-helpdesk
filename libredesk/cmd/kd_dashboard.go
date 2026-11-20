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

	return r.SendEnvelope(map[string]any{
		"days":        days,
		"totals":      totals,
		"agents":      agents,
		"daily":       daily,
		"by_status":   byStatus,
		"by_priority": byPriority,
		"by_customer": byCustomer,
	})
}
