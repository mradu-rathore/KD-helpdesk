package automation

// KD: the default "new email tickets" rule (seeded by cmd/kd_defaults.go)
// parses and fires only for tickets with no group.

import (
	"encoding/json"
	"testing"

	"github.com/abhinavxd/libredesk/internal/automation/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/stretchr/testify/mock"
	"github.com/volatiletech/null/v9"
)

const kdNewTicketRuleJSON = `[{"groups": [{"rules": [{"field": "assigned_team", "value": "", "operator": "not set", "field_type": "conversation"}], "logical_op": "AND"}, {"rules": [], "logical_op": "AND"}], "actions": [{"type": "assign_team", "value": ["4"]}, {"type": "set_priority", "value": ["3"]}, {"type": "add_tags", "value": ["Email Support"]}], "group_operator": "AND"}]`

func TestKDNewTicketRule(t *testing.T) {
	var rules []models.Rule
	if err := json.Unmarshal([]byte(kdNewTicketRuleJSON), &rules); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		teamID    null.Int
		wantCalls int
	}{
		{"email ticket with no group", null.Int{}, 3},
		{"ticket already in a group", null.IntFrom(2), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := new(mockConversationStore)
			engine := createTestEngine(store)
			conversation := createTestConversation(func(c *cmodels.Conversation) { c.AssignedTeamID = tt.teamID })
			if tt.wantCalls > 0 {
				store.On("ApplyAction", mock.Anything, conversation, umodels.User{}).Return(nil).Times(tt.wantCalls)
			}
			engine.evalConversationRules(rules, conversation, map[string]string{})
			if store.callCount != tt.wantCalls {
				t.Fatalf("got %d actions, want %d", store.callCount, tt.wantCalls)
			}
		})
	}
}
