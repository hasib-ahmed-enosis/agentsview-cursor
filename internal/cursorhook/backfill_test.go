package cursorhook

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.kenn.io/agentsview/internal/db"
)

func TestBackfillSessionIDsSetsSessionIDOnExistingRows(t *testing.T) {
	dataDir := t.TempDir()
	logPath := UsageLogPath(dataDir)
	conversationID := "deadbeef-cafe-babe-0000-0000abcdef012345"
	line := `{"recorded_at":"2026-09-15T12:00:00Z","hook_event_name":"afterAgentResponse",` +
		`"conversation_id":"` + conversationID + `","generation_id":"g1","model":"m1",` +
		`"input_tokens":10,"output_tokens":5}` + "\n"
	require.NoError(t, os.WriteFile(logPath, []byte(line), 0o644))

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	ctx := context.Background()

	// Simulate a pre-migration row that was inserted without a session_id.
	row := UsageRow{
		RecordedAt:     "2026-09-15T12:00:00Z",
		HookEventName:  "afterAgentResponse",
		ConversationID: conversationID,
		GenerationID:   "g1",
		Model:          "m1",
	}
	in := int64(10)
	out := int64(5)
	row.InputTokens = &in
	row.OutputTokens = &out
	ev, ok := row.CursorUsageEvent()
	require.True(t, ok)
	ev.SessionID = ""
	require.NoError(t, d.InsertCursorUsageEvents([]db.CursorUsageEvent{ev}))

	events, err := d.GetCursorUsageEvents(ctx, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Empty(t, events[0].SessionID, "row should start without a session_id")

	updated, err := BackfillSessionIDs(ctx, d, dataDir)
	require.NoError(t, err)
	assert.Equal(t, 1, updated)

	events, err = d.GetCursorUsageEvents(ctx, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "cursor:"+conversationID, events[0].SessionID)

	// Running again is a no-op: the row already has a session_id.
	updated, err = BackfillSessionIDs(ctx, d, dataDir)
	require.NoError(t, err)
	assert.Zero(t, updated)
}

func TestBackfillSessionIDsNoLogFile(t *testing.T) {
	dataDir := t.TempDir()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	updated, err := BackfillSessionIDs(context.Background(), d, dataDir)
	require.NoError(t, err)
	assert.Zero(t, updated)
}

func TestBackfillSessionIDsRequiresDatabase(t *testing.T) {
	_, err := BackfillSessionIDs(context.Background(), nil, t.TempDir())
	assert.Error(t, err)
}
