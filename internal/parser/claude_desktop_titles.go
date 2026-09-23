package parser

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

// claudeDesktopSessionTitleCacheTTL bounds how stale a cached title lookup
// can be. The Claude desktop app (Code tab) generates titles asynchronously
// shortly after a session starts, so a short TTL lets a title appear on the
// next sync without rescanning the directory tree on every session.
const claudeDesktopSessionTitleCacheTTL = 30 * time.Second

// claudeDesktopSessionsDefaultDirs returns platform-specific default
// directories that hold Claude Desktop's per-session metadata, including
// the auto-generated title shown in its own sidebar. The Claude desktop app
// is an Electron app, so its user-data directory follows the standard
// Electron convention per platform (mirroring coworkDefaultDirs, which
// resolves the sibling local-agent-mode-sessions directory under the same
// user-data root):
//
//	macOS:   ~/Library/Application Support/Claude/claude-code-sessions
//	Linux:   ~/.config/Claude/claude-code-sessions
//	Windows: %LOCALAPPDATA%\Packages\Claude_pzs8sxrjxfjjc\LocalCache\Roaming\Claude\claude-code-sessions
//	         or %APPDATA%\Claude\claude-code-sessions on non-MSIX/older installs
func claudeDesktopSessionsDefaultDirs() []string {
	return []string{
		// macOS
		"Library/Application Support/Claude/claude-code-sessions",
		// Linux
		".config/Claude/claude-code-sessions",
		// Windows MSIX package install
		"AppData/Local/Packages/Claude_pzs8sxrjxfjjc/" +
			"LocalCache/Roaming/Claude/claude-code-sessions",
		// Windows non-MSIX / older installs
		"AppData/Roaming/Claude/claude-code-sessions",
	}
}

// claudeDesktopSessionsRoots resolves the candidate roots to scan for
// desktop-app session metadata. CLAUDE_DESKTOP_SESSIONS_DIR overrides
// discovery entirely, primarily for tests.
func claudeDesktopSessionsRoots() []string {
	if override := strings.TrimSpace(
		os.Getenv("CLAUDE_DESKTOP_SESSIONS_DIR"),
	); override != "" {
		return []string{override}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	dirs := claudeDesktopSessionsDefaultDirs()
	roots := make([]string, 0, len(dirs))
	for _, rel := range dirs {
		roots = append(roots, filepath.Join(home, filepath.FromSlash(rel)))
	}
	return roots
}

var claudeDesktopTitleCache = struct {
	mu        sync.Mutex
	titles    map[string]string
	scannedAt time.Time
}{}

// claudeDesktopSessionTitle returns the desktop app's auto-generated (or
// user-renamed) title for a Claude Code CLI session, keyed by the session
// ID that also names its transcript file
// (~/.claude/projects/<project>/<sessionID>.jsonl). The CLI transcript
// itself carries no title unless the user ran /rename or the CLI emitted an
// ai-title event; the desktop app generates one independently and stores it
// under its own per-session metadata file
// (<userData>/claude-code-sessions/<workspace>/<project>/local_*.json,
// keyed by "cliSessionId"). Returns "" when no such title is found, e.g.
// the session was not started from the desktop app, or the desktop app is
// not installed on this machine.
func claudeDesktopSessionTitle(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	return claudeDesktopSessionTitles()[sessionID]
}

// resetClaudeDesktopSessionTitleCacheForTest clears the cache so a test
// that points CLAUDE_DESKTOP_SESSIONS_DIR at a fixture is not served a
// stale (or empty) scan from an earlier lookup in the same process.
func resetClaudeDesktopSessionTitleCacheForTest() {
	claudeDesktopTitleCache.mu.Lock()
	defer claudeDesktopTitleCache.mu.Unlock()
	claudeDesktopTitleCache.titles = nil
}

func claudeDesktopSessionTitles() map[string]string {
	claudeDesktopTitleCache.mu.Lock()
	defer claudeDesktopTitleCache.mu.Unlock()
	if claudeDesktopTitleCache.titles != nil &&
		time.Since(claudeDesktopTitleCache.scannedAt) < claudeDesktopSessionTitleCacheTTL {
		return claudeDesktopTitleCache.titles
	}
	titles := make(map[string]string)
	for _, root := range claudeDesktopSessionsRoots() {
		scanClaudeDesktopSessionTitles(root, titles)
	}
	claudeDesktopTitleCache.titles = titles
	claudeDesktopTitleCache.scannedAt = time.Now()
	return titles
}

// scanClaudeDesktopSessionTitles walks <root>/<workspace>/<project>/local_*.json
// (skipping the backlog/ and deleted_*/ subdirectories that live alongside
// those files) and records each session's title, keyed by its cliSessionId.
func scanClaudeDesktopSessionTitles(root string, out map[string]string) {
	workspaces, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, workspace := range workspaces {
		if !workspace.IsDir() {
			continue
		}
		workspaceDir := filepath.Join(root, workspace.Name())
		projects, err := os.ReadDir(workspaceDir)
		if err != nil {
			continue
		}
		for _, project := range projects {
			if !project.IsDir() {
				continue
			}
			scanClaudeDesktopProjectTitles(
				filepath.Join(workspaceDir, project.Name()), out,
			)
		}
	}
}

func scanClaudeDesktopProjectTitles(projectDir string, out map[string]string) {
	files, err := os.ReadDir(projectDir)
	if err != nil {
		return
	}
	for _, file := range files {
		name := file.Name()
		if file.IsDir() ||
			!strings.HasPrefix(name, "local_") ||
			!strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(projectDir, name))
		if err != nil {
			continue
		}
		cliSessionID := strings.TrimSpace(gjson.GetBytes(data, "cliSessionId").Str)
		title := strings.TrimSpace(gjson.GetBytes(data, "title").Str)
		if cliSessionID == "" || title == "" {
			continue
		}
		out[cliSessionID] = title
	}
}
