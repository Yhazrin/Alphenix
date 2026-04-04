package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multicode/server/internal/events"
	"github.com/multica-ai/multicode/server/internal/realtime"
	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

// ReviewService orchestrates task review after agent completion.
type ReviewService struct {
	Queries *db.Queries
	Hub     *realtime.Hub
	Bus     *events.Bus
}

// NewReviewService creates a ReviewService.
func NewReviewService(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *ReviewService {
	return &ReviewService{
		Queries: q,
		Hub:     hub,
		Bus:     bus,
	}
}

// ReviewTask performs automated review on a task.
func (s *ReviewService) ReviewTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	return nil, fmt.Errorf("review not implemented")
}

// SubmitManualReview handles manual review submission.
func (s *ReviewService) SubmitManualReview(ctx context.Context, taskID, reviewerID pgtype.UUID, verdict, feedback string) (*db.AgentTaskQueue, error) {
	return nil, fmt.Errorf("review not implemented")
}
