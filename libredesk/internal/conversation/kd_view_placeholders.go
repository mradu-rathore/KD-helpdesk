package conversation

import (
	"strconv"
	"strings"
)

// KD: shared views can use the value "@me" (e.g. assigned_user_id equals "@me")
// to mean "the agent looking at the view". Zendesk's "Your unsolved tickets"
// view needs this, and Libredesk filters otherwise only take fixed IDs.
func expandViewPlaceholders(filters string, userID int) string {
	return strings.ReplaceAll(filters, `"@me"`, strconv.Quote(strconv.Itoa(userID)))
}
