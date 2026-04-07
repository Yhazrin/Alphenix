package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"strconv"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/mention"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
	"github.com/multica-ai/alphenix/server/pkg/redact"
)

type TaskService struct {
	Queries     *db.Queries
	Hub         *realtime.Hub
	Bus         *events.Bus
	prefixCache sync.Map
}

func NewTaskService(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *TaskService {
	return &TaskService{Queries: q, Hub: hub, Bus: bus}
}

// EnqueueTaskForIssue creates a queued task for an agent-assigned issue.
// No context snapshot is stored — the agent fetches all data it needs at
// runtime via the alphenix CLI.
func (s *TaskService) EnqueueTaskForIssue(ctx context.Context, issue db.Issue, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	if !issue.AssigneeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "issue has no assignee")
		return db.AgentTaskQueue{}, fmt.Errorf("issue has no assignee")
	}

	agent, err := s.Queries.GetAgent(ctx, issue.AssigneeID)
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agent.ID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "agent has no runtime")
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	// Use runtime selection policy if configured, otherwise fall back to agent default.
	runtimeID, err := s.SelectRuntime(ctx, issue.WorkspaceID, issue.AssigneeID)
	if err != nil {
		slog.Warn("runtime selection failed, falling back to agent default",
			"agent_id", util.UUIDToString(issue.AssigneeID), "error", err)
		runtimeID = agent.RuntimeID
	}

	var commentID pgtype.UUID
	if len(triggerCommentID) > 0 {
		commentID = triggerCommentID[0]
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          issue.AssigneeID,
		RuntimeID:        runtimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		TriggerCommentID: commentID,
		ChainReason:      pgtype.Text{String: "", Valid: true},
	})
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(issue.AssigneeID))
	return task, nil
}

// TryEnqueueReadySubIssues finds all sub-issues of a parent issue that have all
// dependencies satisfied and enqueues tasks for those that are assigned to an agent.
// This is called after a goal decomposition is confirmed to automatically start
// unblocked sub-tasks.
func (s *TaskService) TryEnqueueReadySubIssues(ctx context.Context, parentIssueID pgtype.UUID) {
	ready, err := s.Queries.ListReadySubIssues(ctx, parentIssueID)
	if err != nil {
		slog.Warn("TryEnqueueReadySubIssues: failed to list ready sub-issues", "parent_issue_id", util.UUIDToString(parentIssueID), "error", err)
		return
	}

	for _, issue := range ready {
		if !issue.AssigneeID.Valid {
			continue // No assignee, skip
		}
		if _, err := s.EnqueueTaskForIssue(ctx, issue); err != nil {
			slog.Warn("TryEnqueueReadySubIssues: failed to enqueue task for sub-issue", "issue_id", util.UUIDToString(issue.ID), "error", err)
		}
	}
}

// EnqueueTaskForMention creates a queued task for a mentioned agent on an issue.
// Unlike EnqueueTaskForIssue, this takes an explicit agent ID rather than
// deriving it from the issue assignee.
func (s *TaskService) EnqueueTaskForMention(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("mention task enqueue failed: agent not found", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("mention task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("mention task enqueue failed: agent has no runtime", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	// Use runtime selection policy if configured, otherwise fall back to agent default.
	runtimeID, err := s.SelectRuntime(ctx, issue.WorkspaceID, agentID)
	if err != nil {
		slog.Warn("runtime selection failed, falling back to agent default",
			"agent_id", util.UUIDToString(agentID), "error", err)
		runtimeID = agent.RuntimeID
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          agentID,
		RuntimeID:        runtimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		TriggerCommentID: triggerCommentID,
		ChainReason:      pgtype.Text{String: "", Valid: true},
	})
	if err != nil {
		slog.Error("mention task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("mention task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
	return task, nil
}

// CancelTasksForIssue cancels all active tasks for an issue.
func (s *TaskService) CancelTasksForIssue(ctx context.Context, issueID pgtype.UUID) error {
	// Fetch active tasks before cancelling so we can broadcast per-task events.
	activeTasks, err := s.Queries.ListActiveTasksByIssue(ctx, issueID)
	if err != nil {
		return fmt.Errorf("list active tasks: %w", err)
	}

	if err := s.Queries.CancelAgentTasksByIssue(ctx, issueID); err != nil {
		return fmt.Errorf("cancel tasks: %w", err)
	}

	// Broadcast cancel event for each task so frontends update immediately.
	for _, t := range activeTasks {
		t.Status = "cancelled"
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, t)
		s.ReconcileAgentStatus(ctx, t.AgentID)
	}

	return nil
}

// CancelTask cancels a single task by ID. It broadcasts a task:cancelled event
// so frontends can update immediately.
func (s *TaskService) CancelTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	existing, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if !CanTransition(TaskState(existing.Status), TaskStateCancelled) {
		return nil, fmt.Errorf("cannot transition task from %s to cancelled", existing.Status)
	}

	task, err := s.Queries.CancelAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("cancel task: %w", err)
	}

	slog.Info("task cancelled", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast cancellation as a task:failed event so frontends clear the live card
	s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)

	return &task, nil
}

// ClaimTask atomically claims the next queued task for an agent,
// respecting max_concurrent_tasks.
func (s *TaskService) ClaimTask(ctx context.Context, agentID pgtype.UUID) (*db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("count running tasks: %w", err)
	}
	if running >= int64(agent.MaxConcurrentTasks) {
		slog.Debug("task claim: no capacity", "agent_id", util.UUIDToString(agentID), "running", running, "max", agent.MaxConcurrentTasks)
		return nil, nil // No capacity
	}

	task, err := s.Queries.ClaimAgentTask(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("task claim: no tasks available", "agent_id", util.UUIDToString(agentID))
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("claim task: %w", err)
	}

	slog.Info("task claimed", "task_id", util.UUIDToString(task.ID), "agent_id", util.UUIDToString(agentID))

	// Update agent status to working
	s.updateAgentStatus(ctx, agentID, "working")

	// Broadcast task:dispatch
	s.broadcastTaskDispatch(ctx, task)

	// Broadcast agent:started
	s.broadcastAgentEvent(ctx, protocol.EventAgentStarted, task)

	return &task, nil
}

// ClaimTaskForRuntime claims the next runnable task for a runtime while
// still respecting each agent's max_concurrent_tasks limit.
func (s *TaskService) ClaimTaskForRuntime(ctx context.Context, runtimeID pgtype.UUID) (*db.AgentTaskQueue, error) {
	tasks, err := s.Queries.ListPendingTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("list pending tasks: %w", err)
	}

	triedAgents := map[string]struct{}{}
	for _, candidate := range tasks {
		agentKey := util.UUIDToString(candidate.AgentID)
		if _, seen := triedAgents[agentKey]; seen {
			continue
		}
		triedAgents[agentKey] = struct{}{}

		task, err := s.ClaimTask(ctx, candidate.AgentID)
		if err != nil {
			return nil, err
		}
		if task != nil && task.RuntimeID == runtimeID {
			return task, nil
		}
	}

	return nil, nil
}

// StartTask transitions a dispatched task to running.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) StartTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	existing, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if !CanTransition(TaskState(existing.Status), TaskStateRunning) {
		return nil, fmt.Errorf("cannot transition task from %s to running", existing.Status)
	}

	task, err := s.Queries.StartAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}

	slog.Info("task started", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	return &task, nil
}

// CompleteTask marks a task as completed.
// If the agent has review enabled (max_reviews > 0), the task transitions to
// in_review instead of directly completing. The ReviewService handles the rest.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) CompleteTask(ctx context.Context, taskID pgtype.UUID, result []byte, sessionID, workDir string, reviewService *ReviewService) (*db.AgentTaskQueue, error) {
	// Load the task to check review config.
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if !CanTransition(TaskState(task.Status), TaskStateInReview) && !CanTransition(TaskState(task.Status), TaskStateCompleted) {
		return nil, fmt.Errorf("cannot transition task from %s to in_review or completed", task.Status)
	}

	// If review is enabled, transition to in_review instead of completing.
	if task.MaxReviews > 0 && reviewService != nil {
		inReview, err := s.Queries.SetTaskInReview(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("set task in review: %w", err)
		}
		slog.Info("task moved to review", "task_id", util.UUIDToString(taskID))

		// Post agent output as a comment even during review.
		if !task.TriggerCommentID.Valid {
			var payload protocol.TaskCompletedPayload
			if err := json.Unmarshal(result, &payload); err == nil {
				if payload.Output != "" {
					s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(payload.Output), "comment", task.TriggerCommentID)
				}
			}
		}

		// Broadcast in_review status
		s.broadcastTaskEvent(ctx, protocol.EventTaskInReview, inReview)

		// Trigger async review
		go func() {
			if _, reviewErr := reviewService.ReviewTask(context.Background(), taskID); reviewErr != nil {
				slog.Error("automated review failed, marking task as failed", "task_id", util.UUIDToString(taskID), "error", reviewErr)
				if _, failErr := s.FailTask(context.Background(), taskID, fmt.Sprintf("automated review failed: %v", reviewErr)); failErr != nil {
					slog.Error("failed to mark task as failed after review error", "task_id", util.UUIDToString(taskID), "error", failErr)
				}
			}
		}()

		return &inReview, nil
	}

	// No review — complete directly (original behavior).
	completed, err := s.Queries.CompleteAgentTask(ctx, db.CompleteAgentTaskParams{
		ID:        taskID,
		Result:    result,
		SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
		WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("complete task: %w", err)
	}

	slog.Info("task completed", "task_id", util.UUIDToString(completed.ID), "issue_id", util.UUIDToString(completed.IssueID))

	// Check if any dependent tasks are now unblocked.
	s.checkAndLogReadyDependents(ctx, completed.ID)

	// Post agent output as a comment, but only for assignment-triggered tasks.
	// Comment-triggered tasks: the agent replies via CLI with --parent, so
	// posting here would create a duplicate.
	if !completed.TriggerCommentID.Valid {
		var payload protocol.TaskCompletedPayload
		if err := json.Unmarshal(result, &payload); err == nil {
			if payload.Output != "" {
				s.createAgentComment(ctx, completed.IssueID, completed.AgentID, redact.Text(payload.Output), "comment", completed.TriggerCommentID)
			}
		}
	}

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, completed.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, completed)
	s.broadcastAgentEvent(ctx, protocol.EventAgentCompleted, completed)

	return &completed, nil
}

// FailTask marks a task as failed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) FailTask(ctx context.Context, taskID pgtype.UUID, errMsg string) (*db.AgentTaskQueue, error) {
	existing, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if !CanTransition(TaskState(existing.Status), TaskStateFailed) {
		return nil, fmt.Errorf("cannot transition task from %s to failed", existing.Status)
	}

	task, err := s.Queries.FailAgentTask(ctx, db.FailAgentTaskParams{
		ID:    taskID,
		Error: pgtype.Text{String: errMsg, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("fail task: %w", err)
	}

	slog.Warn("task failed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID), "error", errMsg)

	if errMsg != "" {
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(errMsg), "system", task.TriggerCommentID)
	}
	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task)
	s.broadcastAgentEvent(ctx, protocol.EventAgentFailed, task)

	return &task, nil
}

// ReportProgress broadcasts a progress update via the event bus.
func (s *TaskService) ReportProgress(ctx context.Context, taskID string, workspaceID string, summary string, step, total int) {
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskProgress,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskProgressPayload{
			TaskID:  taskID,
			Summary: summary,
			Step:    step,
			Total:   total,
		},
	})
}

// ReconcileAgentStatus checks running task count and sets agent status accordingly.
func (s *TaskService) ReconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return
	}
	newStatus := "idle"
	if running > 0 {
		newStatus = "working"
	}
	slog.Debug("agent status reconciled", "agent_id", util.UUIDToString(agentID), "status", newStatus, "running_tasks", running)
	s.updateAgentStatus(ctx, agentID, newStatus)
}

func (s *TaskService) updateAgentStatus(ctx context.Context, agentID pgtype.UUID, status string) {
	agent, err := s.Queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
		ID:     agentID,
		Status: status,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentStatus,
		WorkspaceID: util.UUIDToString(agent.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"agent": agentToMap(agent)},
	})
}

// LoadAgentSkills loads an agent's skills with their files for task execution.
func (s *TaskService) LoadAgentSkills(ctx context.Context, agentID pgtype.UUID) []AgentSkillData {
	skills, err := s.Queries.ListAgentSkills(ctx, agentID)
	if err != nil || len(skills) == 0 {
		return nil
	}

	result := make([]AgentSkillData, 0, len(skills))
	for _, sk := range skills {
		data := AgentSkillData{Name: sk.Name, Content: sk.Content}
		files, _ := s.Queries.ListSkillFiles(ctx, sk.ID)
		for _, f := range files {
			data.Files = append(data.Files, AgentSkillFileData{Path: f.Path, Content: f.Content})
		}
		result = append(result, data)
	}
	return result
}

// AgentSkillData represents a skill for task execution responses.
type AgentSkillData struct {
	Name    string               `json:"name"`
	Content string               `json:"content"`
	Files   []AgentSkillFileData `json:"files,omitempty"`
}

// AgentSkillFileData represents a supporting file within a skill.
type AgentSkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ChainTask creates a follow-up task for a target agent, linking it to the source task.
// The source task's result is included as context so the target agent has full background.
func (s *TaskService) ChainTask(ctx context.Context, sourceTaskID, targetAgentID pgtype.UUID, chainReason string) (*db.AgentTaskQueue, error) {
	sourceTask, err := s.Queries.GetAgentTask(ctx, sourceTaskID)
	if err != nil {
		return nil, fmt.Errorf("load source task: %w", err)
	}

	agent, err := s.Queries.GetAgent(ctx, targetAgentID)
	if err != nil {
		return nil, fmt.Errorf("load target agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		return nil, fmt.Errorf("target agent is archived")
	}
	if !agent.RuntimeID.Valid {
		return nil, fmt.Errorf("target agent has no runtime")
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:           targetAgentID,
		RuntimeID:         agent.RuntimeID,
		IssueID:           sourceTask.IssueID,
		Priority:          sourceTask.Priority,
		ChainSourceTaskID: sourceTaskID,
		ChainReason:       pgtype.Text{String: chainReason, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create chained task: %w", err)
	}

	slog.Info("task chained",
		"source_task_id", util.UUIDToString(sourceTaskID),
		"chained_task_id", util.UUIDToString(task.ID),
		"target_agent_id", util.UUIDToString(targetAgentID),
		"chain_reason", chainReason,
	)

	s.broadcastTaskEvent(ctx, protocol.EventTaskChained, task)
	return &task, nil
}

func priorityToInt(p string) int32 {
	switch p {
	case "urgent":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func (s *TaskService) broadcastTaskDispatch(ctx context.Context, task db.AgentTaskQueue) {
	var payload map[string]any
	if task.Context != nil {
		if err := json.Unmarshal(task.Context, &payload); err != nil {
			slog.Warn("broadcastTaskDispatch: failed to unmarshal task context", "task_id", util.UUIDToString(task.ID), "error", err)
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["task_id"] = util.UUIDToString(task.ID)
	payload["runtime_id"] = util.UUIDToString(task.RuntimeID)

	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDispatch,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

func (s *TaskService) broadcastTaskEvent(ctx context.Context, eventType string, task db.AgentTaskQueue) {
	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":  util.UUIDToString(task.ID),
			"agent_id": util.UUIDToString(task.AgentID),
			"issue_id": util.UUIDToString(task.IssueID),
			"status":   task.Status,
		},
	})
}

// broadcastAgentEvent publishes an agent lifecycle event (agent:started, agent:completed, agent:failed).
// These complement the task:* events and allow frontends to react at the agent level.
func (s *TaskService) broadcastAgentEvent(ctx context.Context, eventType string, task db.AgentTaskQueue) {
	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":  util.UUIDToString(task.ID),
			"agent_id": util.UUIDToString(task.AgentID),
			"issue_id": util.UUIDToString(task.IssueID),
		},
	})
}

// checkAndLogReadyDependents checks if any tasks depending on the completed task
// are now fully unblocked (all dependencies satisfied). Logs and publishes events
// for tasks that become ready.
func (s *TaskService) checkAndLogReadyDependents(ctx context.Context, completedTaskID pgtype.UUID) {
	dependents, err := s.Queries.GetTaskDependents(ctx, completedTaskID)
	if err != nil {
		slog.Warn("failed to check dependents", "task_id", util.UUIDToString(completedTaskID), "error", err)
		return
	}

	for _, dep := range dependents {
		// Check if ALL dependencies for this dependent task are now satisfied.
		deps, err := s.Queries.GetTaskDependencies(ctx, dep.TaskID)
		if err != nil {
			continue
		}
		allSatisfied := true
		for _, d := range deps {
			if depTask, err := s.Queries.GetAgentTask(ctx, d.DependsOnTaskID); err != nil || (depTask.Status != "completed" && depTask.Status != "failed" && depTask.Status != "cancelled") {
				allSatisfied = false
				break
			}
		}
		if !allSatisfied {
			continue
		}

		// All dependencies satisfied — log and publish event.
		readyTask, err := s.Queries.GetAgentTask(ctx, dep.TaskID)
		if err != nil {
			continue
		}
		slog.Info("dependent task unblocked",
			"task_id", util.UUIDToString(readyTask.ID),
			"issue_id", util.UUIDToString(readyTask.IssueID),
			"unblocked_by", util.UUIDToString(completedTaskID),
		)

		workspaceID := ""
		if issue, err := s.Queries.GetIssue(ctx, readyTask.IssueID); err == nil {
			workspaceID = util.UUIDToString(issue.WorkspaceID)
		}
		if workspaceID != "" {
			s.Bus.Publish(events.Event{
				Type:        "task:dependencies_satisfied",
				WorkspaceID: workspaceID,
				ActorType:   "system",
				ActorID:     "",
				Payload: map[string]any{
					"task_id":        util.UUIDToString(readyTask.ID),
					"unblocked_by":   util.UUIDToString(completedTaskID),
				},
			})
		}
	}
}

func (s *TaskService) broadcastIssueUpdated(issue db.Issue) {
	prefix := s.getIssuePrefix(context.Background(), issue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"issue": issueToMap(issue, prefix)},
	})
}

func (s *TaskService) getIssuePrefix(ctx context.Context, workspaceID pgtype.UUID) string {
	key := util.UUIDToString(workspaceID)
	if v, ok := s.prefixCache.Load(key); ok {
		return v.(string)
	}
	ws, err := s.Queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return ""
	}
	prefix := ws.IssuePrefix
	s.prefixCache.Store(key, prefix)
	return prefix
}

func (s *TaskService) createAgentComment(ctx context.Context, issueID, agentID pgtype.UUID, content, commentType string, parentID pgtype.UUID) {
	if content == "" {
		return
	}
	// Look up issue to get workspace ID for mention expansion and broadcasting.
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return
	}
	// Expand bare issue identifiers (e.g. MUL-117) into mention links.
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)
	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:    issueID,
		AuthorType: "agent",
		AuthorID:   agentID,
		Content:    content,
		Type:       commentType,
		ParentID:   parentID,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(agentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
}

func issueToMap(issue db.Issue, issuePrefix string) map[string]any {
	return map[string]any{
		"id":              util.UUIDToString(issue.ID),
		"workspace_id":    util.UUIDToString(issue.WorkspaceID),
		"number":          issue.Number,
		"identifier":      issuePrefix + "-" + strconv.Itoa(int(issue.Number)),
		"title":           issue.Title,
		"description":     util.TextToPtr(issue.Description),
		"status":          issue.Status,
		"priority":        issue.Priority,
		"assignee_type":   util.TextToPtr(issue.AssigneeType),
		"assignee_id":     util.UUIDToPtr(issue.AssigneeID),
		"creator_type":    issue.CreatorType,
		"creator_id":      util.UUIDToString(issue.CreatorID),
		"parent_issue_id": util.UUIDToPtr(issue.ParentIssueID),
		"position":        issue.Position,
		"due_date":        util.TimestampToPtr(issue.DueDate),
		"created_at":      util.TimestampToString(issue.CreatedAt),
		"updated_at":      util.TimestampToString(issue.UpdatedAt),
	}
}

// agentToMap builds a simple map for broadcasting agent status updates.
func agentToMap(a db.Agent) map[string]any {
	var rc any
	if a.RuntimeConfig != nil {
		if err := json.Unmarshal(a.RuntimeConfig, &rc); err != nil {
			slog.Warn("failed to unmarshal agent runtime_config", "error", err)
		}
	}
	var tools any
	if a.Tools != nil {
		if err := json.Unmarshal(a.Tools, &tools); err != nil {
			slog.Warn("failed to unmarshal agent tools", "error", err)
		}
	}
	var triggers any
	if a.Triggers != nil {
		if err := json.Unmarshal(a.Triggers, &triggers); err != nil {
			slog.Warn("failed to unmarshal agent triggers", "error", err)
		}
	}
	return map[string]any{
		"id":                   util.UUIDToString(a.ID),
		"workspace_id":         util.UUIDToString(a.WorkspaceID),
		"runtime_id":           util.UUIDToString(a.RuntimeID),
		"name":                 a.Name,
		"description":          a.Description,
		"avatar_url":           util.TextToPtr(a.AvatarUrl),
		"runtime_mode":         a.RuntimeMode,
		"runtime_config":       rc,
		"visibility":           a.Visibility,
		"status":               a.Status,
		"max_concurrent_tasks": a.MaxConcurrentTasks,
		"owner_id":             util.UUIDToPtr(a.OwnerID),
		"skills":               []any{},
		"tools":                tools,
		"triggers":             triggers,
		"created_at":           util.TimestampToString(a.CreatedAt),
		"updated_at":           util.TimestampToString(a.UpdatedAt),
		"archived_at":          util.TimestampToPtr(a.ArchivedAt),
		"archived_by":          util.UUIDToPtr(a.ArchivedBy),
	}
}
