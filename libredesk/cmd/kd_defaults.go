package main

// KD: one-time defaults added on startup. Each runs once (recorded in
// kd_defaults_applied), so an admin who edits or deletes the result in the
// UI keeps their change.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/zerodha/logf"
)

const kdDefaultsSchema = `
CREATE TABLE IF NOT EXISTS kd_defaults_applied (
	key TEXT PRIMARY KEY,
	applied_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);`

// kdEnsureDefaults returns true so it can run inside main's var block,
// before the automation engine loads its rules.
func kdEnsureDefaults(db *sqlx.DB, lo *logf.Logger) bool {
	if _, err := db.Exec(kdDefaultsSchema); err != nil {
		lo.Error("kd defaults: error creating table", "error", err)
		return false
	}
	kdApplyOnce(db, lo, "new_ticket_rule_support_high_email_tag", kdNewTicketRule)
	return true
}

// kdApplyOnce runs fn unless key was already applied. fn returns done=false
// to try again on the next start (e.g. a team it needs doesn't exist yet).
func kdApplyOnce(db *sqlx.DB, lo *logf.Logger, key string, fn func(*sqlx.DB) (bool, error)) {
	var exists bool
	if err := db.Get(&exists, `SELECT EXISTS (SELECT 1 FROM kd_defaults_applied WHERE key = $1)`, key); err != nil {
		lo.Error("kd defaults: error checking", "key", key, "error", err)
		return
	}
	if exists {
		return
	}
	done, err := fn(db)
	if err != nil {
		lo.Error("kd defaults: error applying", "key", key, "error", err)
		return
	}
	if !done {
		return
	}
	if _, err := db.Exec(`INSERT INTO kd_defaults_applied (key) VALUES ($1) ON CONFLICT DO NOTHING`, key); err != nil {
		lo.Error("kd defaults: error recording", "key", key, "error", err)
		return
	}
	lo.Info("kd defaults: applied", "key", key)
}

// kdNewTicketRule adds an automation rule: a new ticket with no group goes to
// the Support group with High priority and the "Email Support" tag. It covers
// tickets that arrive by email; tickets logged with the New ticket form take
// the form's values and skip new-ticket rules.
func kdNewTicketRule(db *sqlx.DB) (bool, error) {
	var teamID, priorityID int
	err := db.Get(&teamID, `SELECT id FROM teams WHERE LOWER(name) = 'support' ORDER BY id LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = db.Get(&priorityID, `SELECT id FROM conversation_priorities WHERE LOWER(name) = 'high' ORDER BY id LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := db.Exec(`INSERT INTO tags (name) VALUES ('Email Support') ON CONFLICT (name) DO NOTHING`); err != nil {
		return false, err
	}

	type ruleDetail struct {
		Field     string `json:"field"`
		FieldType string `json:"field_type"`
		Operator  string `json:"operator"`
		Value     string `json:"value"`
	}
	type group struct {
		LogicalOp string       `json:"logical_op"`
		Rules     []ruleDetail `json:"rules"`
	}
	type action struct {
		Type  string   `json:"type"`
		Value []string `json:"value"`
	}
	rules, err := json.Marshal([]map[string]any{{
		"group_operator": "AND",
		"groups": []group{
			{LogicalOp: "AND", Rules: []ruleDetail{{Field: "assigned_team", FieldType: "conversation", Operator: "not set", Value: ""}}},
			{LogicalOp: "AND", Rules: []ruleDetail{}},
		},
		"actions": []action{
			{Type: "assign_team", Value: []string{strconv.Itoa(teamID)}},
			{Type: "set_priority", Value: []string{strconv.Itoa(priorityID)}},
			{Type: "add_tags", Value: []string{"Email Support"}},
		},
	}})
	if err != nil {
		return false, err
	}
	_, err = db.Exec(`
		INSERT INTO automation_rules (name, description, type, rules, events, enabled, weight)
		VALUES ('New email tickets: Support, High, Email Support tag',
			'New tickets without a group go to Support with High priority and the Email Support tag.',
			'new_conversation', $1, '{}', TRUE, 0)`, string(rules))
	return err == nil, err
}
