package cursorhook

import (
	"context"
	"fmt"
	"os"

	"go.kenn.io/agentsview/internal/db"
)

// BackfillSessionIDs sets session_id on existing hook rows by re-reading the JSONL.
func BackfillSessionIDs(
	ctx context.Context, database *db.DB, dataDir string,
) (int, error) {
	if database == nil {
		return 0, fmt.Errorf("database is required")
	}
	path := UsageLogPath(dataDir)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat cursor hook usage log: %w", err)
	}
	if info.Size() == 0 {
		return 0, nil
	}
	rows, _, err := readUsageRowsFromOffset(path, 0)
	if err != nil {
		return 0, fmt.Errorf("read cursor hook usage log: %w", err)
	}
	updated := 0
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return updated, err
		}
		ev, ok := row.CursorUsageEvent()
		if !ok {
			continue
		}
		sessionID := CanonicalSessionID(row.ConversationID)
		if sessionID == "" {
			continue
		}
		n, err := database.UpdateCursorUsageEventSessionID(
			ctx, ev.DedupKey, sessionID,
		)
		if err != nil {
			return updated, err
		}
		updated += n
	}
	return updated, nil
}
