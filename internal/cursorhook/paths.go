package cursorhook

import (
	"path/filepath"
)

const hookUsageFileName = "cursor-hook-usage.jsonl"

// UsageLogPath returns the durable hook telemetry path under the AgentsView
// data directory (~/.agentsview/cursor-hook-usage.jsonl by default).
func UsageLogPath(dataDir string) string {
	return filepath.Join(dataDir, hookUsageFileName)
}
