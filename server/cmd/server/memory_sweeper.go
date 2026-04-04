package main

import (
	"context"
	"log/slog"
	"time"

	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

const (
	// memorySweepInterval is how often we clean up expired agent memories.
	memorySweepInterval = 6 * time.Hour
)

// runMemorySweeper periodically removes expired agent memory entries.
// Memories with a non-null expires_at in the past are deleted to keep
// the workspace context lean and relevant.
func runMemorySweeper(ctx context.Context, queries *db.Queries) {
	ticker := time.NewTicker(memorySweepInterval)
	defer ticker.Stop()

	// Run once on startup to clean any stale entries.
	if err := queries.DeleteExpiredMemory(ctx); err != nil {
		slog.Warn("memory sweeper: initial cleanup failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := queries.DeleteExpiredMemory(ctx); err != nil {
				slog.Warn("memory sweeper: cleanup failed", "error", err)
				continue
			}
			slog.Debug("memory sweeper: expired memories cleaned")
		}
	}
}
