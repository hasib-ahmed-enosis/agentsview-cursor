package cursorhook

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.kenn.io/agentsview/internal/db"
)

func TestStartUsageLogWatcherIngestsOnWrite(t *testing.T) {
	dataDir := t.TempDir()
	logPath := UsageLogPath(dataDir)
	require.NoError(t, os.WriteFile(logPath, nil, 0o644))

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	stop := StartUsageLogWatcher(ctx, dataDir, d, 20*time.Millisecond)
	t.Cleanup(stop)

	line := `{"recorded_at":"2026-09-15T12:00:00Z","hook_event_name":"afterAgentResponse",` +
		`"conversation_id":"deadbeef-cafe-babe-0000-0000abcdef012345","generation_id":"g1",` +
		`"model":"m1","input_tokens":10,"output_tokens":5}` + "\n"
	require.NoError(t, os.WriteFile(logPath, []byte(line), 0o644))

	require.Eventually(t, func() bool {
		events, err := d.GetCursorUsageEvents(ctx, 0)
		return err == nil && len(events) == 1
	}, 2*time.Second, 10*time.Millisecond, "watcher should ingest the appended row")
}

func TestStartUsageLogWatcherNoopWithoutDatabase(t *testing.T) {
	stop := StartUsageLogWatcher(context.Background(), t.TempDir(), nil, time.Second)
	assert.NotPanics(t, stop)
}

func TestStartUsageLogWatcherNoopWithNonPositiveDebounce(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	stop := StartUsageLogWatcher(context.Background(), t.TempDir(), d, 0)
	assert.NotPanics(t, stop)
}
