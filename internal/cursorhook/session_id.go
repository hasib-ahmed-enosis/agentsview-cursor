package cursorhook

import "strings"

const sessionIDPrefix = "cursor:"

// CanonicalSessionID maps a hook conversation UUID to the AgentsView session id.
func CanonicalSessionID(conversationID string) string {
	id := strings.TrimSpace(conversationID)
	if id == "" || !isHookSessionID(id) {
		return ""
	}
	return sessionIDPrefix + id
}
