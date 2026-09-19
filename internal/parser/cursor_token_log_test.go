package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeGlobalCursorHookLog(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENTSVIEW_DATA_DIR", dir)
	path := filepath.Join(dir, cursorHookUsageFileName)
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestEnrichCursorSessionFromTokenLogAfterAgentResponse(t *testing.T) {
	cwd := t.TempDir()
	transcript := filepath.Join(cwd, "sess.jsonl")
	require.NoError(t, os.WriteFile(transcript, []byte("x"), 0o644))

	writeGlobalCursorHookLog(t,
		`{"recorded_at":"2026-09-15T10:00:00Z","hook_event_name":"afterAgentResponse","conversation_id":"sess","input_tokens":100,"output_tokens":40,"workspace_root":"`+filepath.ToSlash(cwd)+`"}`,
		`{"recorded_at":"2026-09-15T10:01:00Z","hook_event_name":"afterAgentResponse","conversation_id":"sess","input_tokens":200,"output_tokens":60,"workspace_root":"`+filepath.ToSlash(cwd)+`"}`,
	)

	msgs := []ParsedMessage{
		{Role: RoleUser, Content: "hi"},
		{Role: RoleAssistant, Content: "one"},
		{Role: RoleUser, Content: "again"},
		{Role: RoleAssistant, Content: "two"},
	}

	require.NoError(t, enrichCursorSessionFromTokenLog(transcript, cwd, msgs))

	assert.True(t, msgs[1].HasOutputTokens)
	assert.Equal(t, 40, msgs[1].OutputTokens)
	assert.Equal(t, 100, msgs[1].ContextTokens)
	assert.Contains(t, string(msgs[1].TokenUsage), `"input_tokens":100`)

	assert.True(t, msgs[3].HasOutputTokens)
	assert.Equal(t, 60, msgs[3].OutputTokens)
	assert.Equal(t, 200, msgs[3].ContextTokens)
}

func TestEnrichCursorSessionFromTokenLogStopFallback(t *testing.T) {
	cwd := t.TempDir()
	transcript := filepath.Join(cwd, "abc.jsonl")
	require.NoError(t, os.WriteFile(transcript, []byte("x"), 0o644))

	writeGlobalCursorHookLog(t,
		`{"recorded_at":"2026-09-15T10:00:00Z","hook_event_name":"stop","conversation_id":"abc","input_tokens":500,"output_tokens":120}`,
	)

	msgs := []ParsedMessage{
		{Role: RoleUser, Content: "hi"},
		{Role: RoleAssistant, Content: "done"},
	}

	require.NoError(t, enrichCursorSessionFromTokenLog(transcript, cwd, msgs))
	assert.Equal(t, 120, msgs[1].OutputTokens)
	assert.Equal(t, 500, msgs[1].ContextTokens)
}

func TestCursorTokenLogRowMatchesSessionWorkspace(t *testing.T) {
	cwd := t.TempDir()
	row := cursorTokenLogRow{
		ConversationID: "sess",
		WorkspaceRoot:  filepath.ToSlash(cwd),
	}
	assert.True(t, cursorTokenLogRowMatchesSession(row, "sess", "", cwd))

	row.WorkspaceRoot = "/other/path"
	assert.False(t, cursorTokenLogRowMatchesSession(row, "sess", "", cwd))
}

func TestCursorSourceSetSourcesForTokenLogPath(t *testing.T) {
	projects := t.TempDir()
	projectDir := filepath.Join(projects, "encoded-workspace", "agent-transcripts")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	transcript := filepath.Join(projectDir, "deadbeef.jsonl")
	require.NoError(t, os.WriteFile(transcript, []byte(`{"role":"user"}`), 0o644))

	logPath := writeGlobalCursorHookLog(t,
		`{"conversation_id":"deadbeef","hook_event_name":"stop","input_tokens":1,"output_tokens":1}`,
	)

	set := newCursorSourceSet([]string{projects})
	sources, err := set.sourcesForTokenLogPath(logPath, logPath)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, transcript, sources[0].DisplayPath)
	assert.Equal(t, "cursor:deadbeef", CursorSessionID(sources[0].DisplayPath))
}

func TestCursorTokenLogCompositeMtime(t *testing.T) {
	writeGlobalCursorHookLog(t, `{"conversation_id":"x"}`)
	mtime, err := cursorTokenLogCompositeMtime("")
	require.NoError(t, err)
	assert.Positive(t, mtime)
}

func TestCursorTokenLogChangedPath(t *testing.T) {
	assert.True(t, cursorTokenLogChangedPath(`C:\Users\me\.agentsview\cursor-hook-usage.jsonl`))
	assert.False(t, cursorTokenLogChangedPath(`D:\work\.cursor\telemetry\token-log.jsonl`))
}
