package cursorhook

import (
	"context"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"go.kenn.io/agentsview/internal/db"
)

// StartUsageLogWatcher ingests hook telemetry when the global JSONL file changes.
// It returns a stop function that must be called on shutdown.
func StartUsageLogWatcher(
	ctx context.Context,
	dataDir string,
	database *db.DB,
	debounce time.Duration,
) func() {
	if database == nil || debounce <= 0 {
		return func() {}
	}
	logPath := UsageLogPath(dataDir)
	watchDir := filepath.Dir(logPath)
	baseName := filepath.Base(logPath)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("cursor hook usage watch: %v", err)
		return func() {}
	}

	if err := watcher.Add(watchDir); err != nil {
		log.Printf("cursor hook usage watch: %v", err)
		_ = watcher.Close()
		return func() {}
	}

	stop := make(chan struct{})
	var stopOnce sync.Once
	closeWatcher := func() {
		stopOnce.Do(func() {
			close(stop)
			_ = watcher.Close()
		})
	}

	var (
		mu            sync.Mutex
		pending       bool
		debounceTimer *time.Timer
	)

	schedule := func() {
		mu.Lock()
		defer mu.Unlock()
		pending = true
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(debounce, func() {
			mu.Lock()
			if !pending {
				mu.Unlock()
				return
			}
			pending = false
			mu.Unlock()
			runIngest(ctx, dataDir, database)
		})
	}

	go func() {
		for {
			select {
			case <-stop:
				mu.Lock()
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				mu.Unlock()
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Base(event.Name) != baseName {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				schedule()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("cursor hook usage watch: %v", err)
			}
		}
	}()

	return closeWatcher
}

func runIngest(ctx context.Context, dataDir string, database *db.DB) {
	if ctx.Err() != nil {
		return
	}
	count, err := Ingest(ctx, database, dataDir)
	if err != nil && ctx.Err() == nil {
		log.Printf("cursor hook usage ingest: %v", err)
		return
	}
	if count > 0 && ctx.Err() == nil {
		log.Printf("ingested %d cursor hook usage event(s)", count)
	}
}
