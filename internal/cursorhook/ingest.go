package cursorhook

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"go.kenn.io/agentsview/internal/db"
)

const archiveMetadataIngestBytesKey = "cursor_hook_usage_ingest_bytes"

// Ingest appends new hook telemetry rows from the global JSONL store into the
// archive cursor_usage_events table. It is safe to call after every sync pass.
func Ingest(ctx context.Context, database *db.DB, dataDir string) (int, error) {
	if database == nil {
		return 0, errors.New("database is required")
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

	offset, err := database.ArchiveMetadataInt(ctx, archiveMetadataIngestBytesKey)
	if err != nil {
		return 0, err
	}
	if offset > info.Size() {
		offset = 0
	}
	if offset == info.Size() {
		return 0, nil
	}

	rows, newSize, err := readUsageRowsFromOffset(path, offset)
	if err != nil {
		return 0, fmt.Errorf("read cursor hook usage log: %w", err)
	}

	events := make([]db.CursorUsageEvent, 0, len(rows))
	for _, row := range rows {
		ev, ok := row.CursorUsageEvent()
		if !ok {
			continue
		}
		events = append(events, ev)
	}
	if len(events) > 0 {
		if err := database.InsertCursorUsageEvents(events); err != nil {
			return 0, err
		}
	}
	if err := database.SetArchiveMetadata(
		ctx, archiveMetadataIngestBytesKey, strconv.FormatInt(newSize, 10),
	); err != nil {
		return 0, err
	}
	return len(events), nil
}
