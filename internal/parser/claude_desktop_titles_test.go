package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeClaudeDesktopTitleFixture creates a fake
// <root>/<workspace>/<project>/local_<uuid>.json file the way the Claude
// desktop app lays out its per-session metadata.
func writeClaudeDesktopTitleFixture(
	t *testing.T, root, cliSessionID, title string,
) {
	t.Helper()
	dir := filepath.Join(root, "workspace1", "project1")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	content := `{"sessionId":"local_fixture","cliSessionId":"` + cliSessionID +
		`","title":"` + title + `","titleSource":"auto"}`
	path := filepath.Join(dir, "local_"+cliSessionID+".json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestClaudeDesktopSessionTitleLookup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_DESKTOP_SESSIONS_DIR", root)
	resetClaudeDesktopSessionTitleCacheForTest()
	t.Cleanup(resetClaudeDesktopSessionTitleCacheForTest)

	writeClaudeDesktopTitleFixture(t, root, "session-abc", "Fix login bug")

	assert.Equal(t, "Fix login bug", claudeDesktopSessionTitle("session-abc"))
	assert.Equal(t, "", claudeDesktopSessionTitle("unknown-session"))
	assert.Equal(t, "", claudeDesktopSessionTitle(""))
}

func TestClaudeParseUsesDesktopTitleWhenTranscriptHasNone(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_DESKTOP_SESSIONS_DIR", root)
	resetClaudeDesktopSessionTitleCacheForTest()
	t.Cleanup(resetClaudeDesktopSessionTitleCacheForTest)

	path := filepath.Join(t.TempDir(), "session-abc.jsonl")
	content := strings.Join([]string{
		`{"type":"user","sessionId":"session-abc","uuid":"u1","message":{"content":"hi"}}`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	writeClaudeDesktopTitleFixture(t, root, "session-abc", "Fix login bug")

	results, _, err := claudeParseWithExclusions(path, "project", "local")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Fix login bug", results[0].Session.SessionName)
}

func TestClaudeParseRenameToEmptyIsNotOverriddenByDesktopTitle(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_DESKTOP_SESSIONS_DIR", root)
	resetClaudeDesktopSessionTitleCacheForTest()
	t.Cleanup(resetClaudeDesktopSessionTitleCacheForTest)

	path := filepath.Join(t.TempDir(), "session-abc.jsonl")
	content := strings.Join([]string{
		`{"type":"user","sessionId":"session-abc","uuid":"u1","message":{"content":"hi"}}`,
		`{"type":"system","content":"<command-name>rename</command-name><command-args></command-args>","sessionId":"session-abc"}`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	writeClaudeDesktopTitleFixture(t, root, "session-abc", "Fix login bug")

	results, _, err := claudeParseWithExclusions(path, "project", "local")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "", results[0].Session.SessionName)
}
