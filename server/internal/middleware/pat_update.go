package middleware

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// patUpdateTask is a best-effort request to update a PAT's last_used_at.
type patUpdateTask struct {
	patID pgtype.UUID
}

// patUpdateCh is a buffered channel that acts as a bounded worker pool
// for PAT last-used-at updates. Replaces the previous fire-and-forget
// goroutine pattern that could spawn unbounded goroutines under load.
var patUpdateCh = make(chan patUpdateTask, 256)

// StartPATUpdatePool launches n workers that process PAT last-used updates.
// Must be called once at server startup (e.g. in main.go).
func StartPATUpdatePool(queries *db.Queries, n int) {
	for i := 0; i < n; i++ {
		go func() {
			for task := range patUpdateCh {
				if err := queries.UpdatePersonalAccessTokenLastUsed(context.Background(), task.patID); err != nil {
					slog.Debug("PAT last_used_at update failed", "error", err)
				}
			}
		}()
	}
}

// enqueuePATUpdate sends a best-effort last_used_at update to the worker pool.
// Drops silently if the channel is full (non-blocking) to avoid slowing requests.
func enqueuePATUpdate(patID pgtype.UUID) {
	select {
	case patUpdateCh <- patUpdateTask{patID: patID}:
	default:
		// Channel full — drop the update. Best-effort, non-critical.
	}
}
