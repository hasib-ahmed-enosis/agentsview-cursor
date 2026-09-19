package cursorhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageRowCursorUsageEventAfterAgentResponse(t *testing.T) {
	in := int64(100)
	out := int64(40)
	row := UsageRow{
		RecordedAt:     "2026-09-15T10:00:00Z",
		HookEventName:  "afterAgentResponse",
		ConversationID: "deadbeef-cafe-babe-0000-0000abcdef012345",
		GenerationID:   "gen-1",
		Model:          "claude-4-sonnet",
		InputTokens:    &in,
		OutputTokens:   &out,
	}
	ev, ok := row.CursorUsageEvent()
	require.True(t, ok)
	assert.Equal(t, hookUsageEventKind, ev.Kind)
	assert.Equal(t, 100, ev.InputTokens)
	assert.Equal(t, 40, ev.OutputTokens)
	assert.NotEmpty(t, ev.DedupKey)
	assert.Equal(
		t,
		"cursor:deadbeef-cafe-babe-0000-0000abcdef012345",
		ev.SessionID,
	)
}

func TestUsageRowCursorUsageEventSkipsNonBillable(t *testing.T) {
	row := UsageRow{
		HookEventName: "subagentStop",
		Model:         "gpt-4",
	}
	_, ok := row.CursorUsageEvent()
	assert.False(t, ok)
}

func TestUsageRowCursorUsageEventDedupStableWithGeneration(t *testing.T) {
	in := int64(1)
	row := UsageRow{
		RecordedAt:     "2026-09-15T10:00:00Z",
		HookEventName:  "stop",
		GenerationID:   "gen-stable",
		ConversationID: "sess",
		Model:          "model-a",
		InputTokens:    &in,
	}
	ev1, ok := row.CursorUsageEvent()
	require.True(t, ok)
	ev2, ok := row.CursorUsageEvent()
	require.True(t, ok)
	assert.Equal(t, ev1.DedupKey, ev2.DedupKey)
}
