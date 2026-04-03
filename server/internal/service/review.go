package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// ReviewCheck defines a single automated review check.
type ReviewCheck struct {
	Name    string
	Weight  int
	Execute func(ctx context.Context, task db.AgentTaskQueue, issue db.Issue) (passed bool, feedback string)
}

// ReviewService orchestrates task review after agent completion.
type ReviewService struct {
	Queries *db.Queries
	Hub     *realtime.Hub
	Bus     *events.Bus
	Checks  []ReviewCheck
}

// NewReviewService creates a ReviewService with default automated checks.
func NewReviewService(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *ReviewService {
	s := &ReviewService{
		Queries: q,
		Hub:     hub,
		Bus:     bus,
	}
	s.Checks = s.defaultChecks()
	return s
}

// ReviewTask performs automated review on a task in in_review status.
// It evaluates all checks, computes a weighted score, and transitions the task
// to completed, queued (retry), or failed based on the result.
func (s *ReviewService) ReviewTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if task.Status != "in_review" {
		return nil, fmt.Errorf("task %s is not in review (status: %s)", util.UUIDToString(taskID), task.Status)
	}

	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return nil, fmt.Errorf("load issue: %w", err)
	}

	totalWeight := 0
	earnedScore := 0
	var feedbackParts []string

	for _, check := range s.Checks {
		totalWeight += check.Weight
		passed, feedback := check.Execute(ctx, task, issue)
		if passed {
			earnedScore += check.Weight
		}
		if feedback != "" {
			feedbackParts = append(feedbackParts, fmt.Sprintf("[%s] %s", check.Name, feedback))
		}
	}

	score := 0
	if totalWeight > 0 {
		score = earnedScore * 100 / totalWeight
	}
	verdict := "pass"
	if score < 60 {
		verdict = "fail"
	}

	feedback := ""
	if len(feedbackParts) > 0 {
		feedback = joinFeedback(feedbackParts)
	}

	review, err := s.Queries.CreateTaskReview(ctx, db.CreateTaskReviewParams{
		TaskID:       taskID,
		ReviewerType: "automated",
		Verdict:      verdict,
		Score:        int32(score),
		Feedback:     feedback,
	})
	if err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}

	slog.Info("task reviewed",
		"task_id", util.UUIDToString(taskID),
		"verdict", verdict,
		"score", score,
		"review_id", util.UUIDToString(review.ID),
	)

	var updatedTask db.AgentTaskQueue
	switch verdict {
	case "pass":
		updatedTask, err = s.Queries.CompleteTaskReview(ctx, db.CompleteTaskReviewParams{
			ID:        taskID,
			Result:    task.Result,
			SessionID: task.SessionID,
			WorkDir:   task.WorkDir,
		})
		if err != nil {
			return nil, fmt.Errorf("complete review: %w", err)
		}
	default:
		// fail — retry if under limit, otherwise fail permanently
		updatedTask, err = s.Queries.RetryTaskReview(ctx, taskID)
		if err != nil {
			// Retry failed (likely review_count >= max_reviews), so fail permanently
			updatedTask, err = s.Queries.FailTaskReview(ctx, db.FailTaskReviewParams{
				ID:    taskID,
				Error: feedback,
			})
			if err != nil {
				return nil, fmt.Errorf("fail review: %w", err)
			}
			slog.Warn("task review failed permanently", "task_id", util.UUIDToString(taskID), "score", score)
		} else {
			slog.Info("task review retrying", "task_id", util.UUIDToString(taskID), "review_count", updatedTask.ReviewCount)
		}
	}

	s.broadcastReviewEvent(ctx, updatedTask, review)
	return &updatedTask, nil
}

// SubmitManualReview records a human review and transitions the task accordingly.
func (s *ReviewService) SubmitManualReview(ctx context.Context, taskID pgtype.UUID, reviewerID pgtype.UUID, verdict, feedback string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if task.Status != "in_review" {
		return nil, fmt.Errorf("task %s is not in review (status: %s)", util.UUIDToString(taskID), task.Status)
	}

	score := int32(100)
	if verdict == "fail" || verdict == "retry" {
		score = 0
	}

	review, err := s.Queries.CreateTaskReview(ctx, db.CreateTaskReviewParams{
		TaskID:       taskID,
		ReviewerType: "member",
		ReviewerID:   reviewerID,
		Verdict:      verdict,
		Score:        score,
		Feedback:     feedback,
	})
	if err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}

	var updatedTask db.AgentTaskQueue
	switch verdict {
	case "pass":
		updatedTask, err = s.Queries.CompleteTaskReview(ctx, db.CompleteTaskReviewParams{
			ID:        taskID,
			Result:    task.Result,
			SessionID: task.SessionID,
			WorkDir:   task.WorkDir,
		})
	case "retry":
		updatedTask, err = s.Queries.RetryTaskReview(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("retry review: %w", err)
		}
	default:
		updatedTask, err = s.Queries.FailTaskReview(ctx, db.FailTaskReviewParams{
			ID:    taskID,
			Error: feedback,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("apply review verdict: %w", err)
	}

	slog.Info("manual review submitted",
		"task_id", util.UUIDToString(taskID),
		"reviewer_id", util.UUIDToString(reviewerID),
		"verdict", verdict,
	)

	s.broadcastReviewEvent(ctx, updatedTask, review)
	return &updatedTask, nil
}

func (s *ReviewService) broadcastReviewEvent(ctx context.Context, task db.AgentTaskQueue, review db.TaskReview) {
	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return
	}

	eventType := protocol.EventTaskReviewed
	if task.Status == "queued" {
		eventType = protocol.EventTaskInReview
	}

	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":   util.UUIDToString(task.ID),
			"agent_id":  util.UUIDToString(task.AgentID),
			"issue_id":  util.UUIDToString(task.IssueID),
			"status":    task.Status,
			"verdict":   review.Verdict,
			"score":     review.Score,
			"feedback":  review.Feedback,
			"review_id": util.UUIDToString(review.ID),
		},
	})
}

func (s *ReviewService) defaultChecks() []ReviewCheck {
	return []ReviewCheck{
		{
			Name:   "output_nonempty",
			Weight: 20,
			Execute: func(ctx context.Context, task db.AgentTaskQueue, issue db.Issue) (bool, string) {
				if len(task.Result) == 0 {
					return false, "agent result is empty"
				}
				return true, ""
			},
		},
		{
			Name:   "comment_exists",
			Weight: 20,
			Execute: func(ctx context.Context, task db.AgentTaskQueue, issue db.Issue) (bool, string) {
				comments, err := s.Queries.ListComments(ctx, db.ListCommentsParams{
					IssueID:     task.IssueID,
					WorkspaceID: issue.WorkspaceID,
				})
				if err != nil {
					return false, "could not list comments"
				}
				for _, c := range comments {
					if c.AuthorType == "agent" && c.AuthorID == task.AgentID {
						return true, ""
					}
				}
				return false, "agent did not leave a comment"
			},
		},
		{
			Name:   "issue_status_changed",
			Weight: 30,
			Execute: func(ctx context.Context, task db.AgentTaskQueue, issue db.Issue) (bool, string) {
				switch issue.Status {
				case "done", "completed", "cancelled", "wont_fix":
					return true, ""
				default:
					return false, fmt.Sprintf("issue status is still %q", issue.Status)
				}
			},
		},
		{
			Name:   "result_parseable",
			Weight: 30,
			Execute: func(ctx context.Context, task db.AgentTaskQueue, issue db.Issue) (bool, string) {
				if len(task.Result) == 0 {
					return false, "result is empty"
				}
				var payload map[string]any
				if err := json.Unmarshal(task.Result, &payload); err != nil {
					return false, "result is not valid JSON"
				}
				return true, ""
			},
		},
	}
}

func joinFeedback(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "; "
		}
		result += p
	}
	return result
}
