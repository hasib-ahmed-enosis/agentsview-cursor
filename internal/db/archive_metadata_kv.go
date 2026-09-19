package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ArchiveMetadataInt reads a decimal integer from archive_metadata.
func (db *DB) ArchiveMetadataInt(ctx context.Context, key string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return 0, errors.New("archive metadata key is empty")
	}
	var raw string
	err := db.getReader().QueryRowContext(ctx,
		`SELECT value FROM archive_metadata WHERE key = ?`, key,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading archive metadata %q: %w", key, err)
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid archive metadata %q value %q: %w", key, raw, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("invalid archive metadata %q value %q: negative", key, raw)
	}
	return value, nil
}

// SetArchiveMetadata upserts one archive_metadata key.
func (db *DB) SetArchiveMetadata(ctx context.Context, key, value string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := db.requireWritable(); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("archive metadata key is empty")
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, err := db.getWriter().ExecContext(ctx, `
		INSERT INTO archive_metadata (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		key, SanitizeUTF8(value),
	); err != nil {
		return fmt.Errorf("writing archive metadata %q: %w", key, err)
	}
	return nil
}
