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

func TestIngestAppendsRows(t *testing.T) {
	dataDir := t.TempDir()
	logPath := UsageLogPath(dataDir)
	line := `{"recorded_at":"2026-09-15T12:00:00Z","hook_event_name":"afterAgentResponse","conversation_id":"sess","generation_id":"g1","model":"m1","input_tokens":10,"output_tokens":5}` + "\n"
	require.NoError(t, os.WriteFile(logPath, []byte(line), 0o644))

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	ctx := context.Background()

	count, err := Ingest(ctx, d, dataDir)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	count, err = Ingest(ctx, d, dataDir)
	require.NoError(t, err)
	assert.Zero(t, count)

	events, err := d.GetCursorUsageEvents(ctx, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "hook", events[0].Kind)
	assert.Equal(t, 10, events[0].InputTokens)
}

func TestIngestSkipsBOMPrefixedFirstLine(t *testing.T) {
	dataDir := t.TempDir()
	logPath := UsageLogPath(dataDir)
	line := `{"recorded_at":"2026-09-15T12:00:00Z","hook_event_name":"afterAgentResponse","conversation_id":"sess","generation_id":"g1","model":"m1","input_tokens":10,"output_tokens":5}` + "\n"
	require.NoError(t, os.WriteFile(logPath, append([]byte{0xef, 0xbb, 0xbf}, []byte(line)...), 0o644))

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	count, err := Ingest(context.Background(), d, dataDir)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
