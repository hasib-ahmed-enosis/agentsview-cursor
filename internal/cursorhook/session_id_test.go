package cursorhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalSessionID(t *testing.T) {
	assert.Equal(t, "", CanonicalSessionID(""))
	assert.Equal(t, "", CanonicalSessionID("short"))
	assert.Equal(
		t,
		"cursor:deadbeef-cafe-babe-0000-0000abcdef012345",
		CanonicalSessionID("deadbeef-cafe-babe-0000-0000abcdef012345"),
	)
}

func TestUsageRowSessionIDUsesCanonicalPrefix(t *testing.T) {
	row := UsageRow{ConversationID: "deadbeef-cafe-babe-0000-0000abcdef012345"}
	assert.Equal(t, "cursor:deadbeef-cafe-babe-0000-0000abcdef012345", row.SessionID())
}
