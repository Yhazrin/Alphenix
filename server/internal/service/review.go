package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// ReviewService orchestrates task review after agent completion.
type ReviewService struct {
	Queries     *db.Queries
	Hub         *realtime.Hub
	Bus         *events.Bus
	TaskService *TaskService
}

// NewReviewService creates a ReviewService.
func NewReviewService(q *db.Queries, hub *realtime.Hub, bus *events.Bus, taskService *TaskService) *ReviewService {
	return &ReviewService{
		Queries:     q,
		Hub:         hub,
		Bus:         bus,
		TaskService: taskService,
	}
}

// ReviewTask performs automated review on a task (auto-approve path).
func (s *ReviewService) ReviewTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if task.Status != "in_review" {
		return nil, fmt.Errorf("task is not in review (status: %s)", task.Status)
	}

	// Auto-approve: complete the task.
	slog.Info("auto-approving task", "task_id", util.UUIDToString(taskID), "issue_id", util.UUIDToString(task.IssueID))

	// Broadcast reviewed event
	s.broadcastReviewEvent(ctx, protocol.EventTaskReviewed, task, "approved", "auto-approved")

	completed, err := s.Queries.CompleteTaskReview(ctx, db.CompleteTaskReviewParams{
		ID:        taskID,
		Result:    task.Result,
		SessionID: task.SessionID,
		WorkDir:   task.WorkDir,
	})
	if err != nil {
		return nil, fmt.Errorf("complete task after review: %w", err)
	}

	slog.Info("task completed after review", "task_id", util.UUIDToString(taskID))

	// Check dependent tasks
	s.TaskService.checkAndLogReadyDependents(ctx, completed.ID)

	// Reconcile agent status
	s.TaskService.ReconcileAgentStatus(ctx, completed.AgentID)

	// Broadcast completion
	s.TaskService.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, completed)

	return &completed, nil
}

// SubmitManualReview handles manual review submission.
func (s *ReviewService) SubmitManualReview(ctx context.Context, taskID, reviewerID pgtype.UUID, verdict, feedback string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if task.Status != "in_review" {
		return nil, fmt.Errorf("task is not in review (status: %s)", task.Status)
	}

	switch verdict {
	case "approved":
		slog.Info("task approved by reviewer", "task_id", util.UUIDToString(taskID), "reviewer_id", util.UUIDToString(reviewerID))

		s.broadcastReviewEvent(ctx, protocol.EventTaskReviewed, task, "approved", feedback)

		completed, err := s.Queries.CompleteTaskReview(ctx, db.CompleteTaskReviewParams{
			ID:        taskID,
			Result:    task.Result,
			SessionID: task.SessionID,
			WorkDir:   task.WorkDir,
		})
		if err != nil {
			return nil, fmt.Errorf("complete task after approval: %w", err)
		}

		if feedback != "" {
			s.TaskService.createAgentComment(ctx, task.IssueID, task.AgentID, feedback, "comment", task.TriggerCommentID)
		}

		s.TaskService.checkAndLogReadyDependents(ctx, completed.ID)
		s.TaskService.ReconcileAgentStatus(ctx, completed.AgentID)
		s.TaskService.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, completed)

		return &completed, nil

	case "rejected":
		slog.Info("task rejected by reviewer", "task_id", util.UUIDToString(taskID), "reviewer_id", util.UUIDToString(reviewerID))

		s.broadcastReviewEvent(ctx, protocol.EventTaskReviewed, task, "rejected", feedback)

		failed, err := s.TaskService.FailTask(ctx, taskID, feedback)
		if err != nil {
			return nil, fmt.Errorf("fail task after rejection: %w", err)
		}

		return failed, nil

	default:
		return nil, fmt.Errorf("invalid verdict: %s (must be approved or rejected)", verdict)
	}
}

// RetryTask resets an in_review task back to queued for re-execution.
func (s *ReviewService) RetryTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	retried, err := s.Queries.RetryTaskReview(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("retry task: %w", err)
	}

	slog.Info("task retried", "task_id", util.UUIDToString(taskID))

	s.broadcastReviewEvent(ctx, protocol.EventTaskReviewed, retried, "retry", "")

	return &retried, nil
}

func (s *ReviewService) broadcastReviewEvent(ctx context.Context, eventType string, task db.AgentTaskQueue, verdict, feedback string) {
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: util.UUIDToString(getIssueWorkspaceID(ctx, s.Queries, task.IssueID)),
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":  util.UUIDToString(task.ID),
			"agent_id": util.UUIDToString(task.AgentID),
			"issue_id": util.UUIDToString(task.IssueID),
			"verdict":  verdict,
			"feedback": feedback,
		},
	})
}

func getIssueWorkspaceID(ctx context.Context, q *db.Queries, issueID pgtype.UUID) pgtype.UUID {
	issue, err := q.GetIssue(ctx, issueID)
	if err != nil {
		return pgtype.UUID{}
	}
	return issue.WorkspaceID
}
