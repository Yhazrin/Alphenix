package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/util"
	"github.com/multica-ai/alphenix/server/pkg/agent"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// Run phases — the lifecycle a run progresses through.
const (
	PhasePending    = "pending"
	PhasePlanning   = "planning"
	PhaseExecuting  = "executing"
	PhaseReviewing  = "reviewing"
	PhaseCompleted  = "completed"
	PhaseFailed     = "failed"
	PhaseCancelled  = "cancelled"
)

// allowedRunTransitions defines valid run phase transitions.
var allowedRunTransitions = map[string][]string{
	PhasePending:   {PhasePlanning, PhaseExecuting, PhaseFailed, PhaseCancelled},
	PhasePlanning:  {PhaseExecuting, PhaseFailed, PhaseCancelled},
	PhaseExecuting: {PhaseReviewing, PhaseCompleted, PhaseFailed, PhaseCancelled},
	PhaseReviewing: {PhaseCompleted, PhaseFailed, PhaseCancelled},
	PhaseCompleted: {},
	PhaseFailed:    {},
	PhaseCancelled: {},
}

// CanRunTransition returns true if transitioning from oldPhase to newPhase is allowed.
func CanRunTransition(oldPhase, newPhase string) bool {
	allowed, ok := allowedRunTransitions[oldPhase]
	if !ok {
		return false
	}
	for _, p := range allowed {
		if p == newPhase {
			return true
		}
	}
	return false
}

// RunQuerier abstracts the database operations used by RunOrchestrator,
// enabling test doubles (stubQueries) to be injected.
type RunQuerier interface {
	CreateRun(ctx context.Context, arg db.CreateRunParams) (db.Run, error)
	StartRun(ctx context.Context, id pgtype.UUID) (db.Run, error)
	GetRun(ctx context.Context, id pgtype.UUID) (db.Run, error)
	UpdateRunPhase(ctx context.Context, arg db.UpdateRunPhaseParams) (db.Run, error)
	CompleteRun(ctx context.Context, id pgtype.UUID) (db.Run, error)
	FailRun(ctx context.Context, id pgtype.UUID) (db.Run, error)
	CancelRun(ctx context.Context, id pgtype.UUID) (db.Run, error)
	UpdateRunTokens(ctx context.Context, arg db.UpdateRunTokensParams) (db.Run, error)
	GetNextStepSeq(ctx context.Context, id pgtype.UUID) (int32, error)
	CreateRunStep(ctx context.Context, arg db.CreateRunStepParams) (db.RunStep, error)
	CompleteRunStep(ctx context.Context, arg db.CompleteRunStepParams) (db.RunStep, error)
	GetNextTodoSeq(ctx context.Context, id pgtype.UUID) (int32, error)
	CreateRunTodo(ctx context.Context, arg db.CreateRunTodoParams) (db.RunTodo, error)
	UpdateRunTodo(ctx context.Context, arg db.UpdateRunTodoParams) (db.RunTodo, error)
	CreateRunHandoff(ctx context.Context, arg db.CreateRunHandoffParams) (db.RunHandoff, error)
	CreateRunContinuation(ctx context.Context, arg db.CreateRunContinuationParams) (db.RunContinuation, error)
	CreateRunArtifact(ctx context.Context, arg db.CreateRunArtifactParams) (db.RunArtifact, error)
	ListRunsByWorkspace(ctx context.Context, arg db.ListRunsByWorkspaceParams) ([]db.Run, error)
	ListRunsByIssue(ctx context.Context, id pgtype.UUID) ([]db.Run, error)
	ListRunSteps(ctx context.Context, id pgtype.UUID) ([]db.RunStep, error)
	ListRunTodos(ctx context.Context, id pgtype.UUID) ([]db.RunTodo, error)
	ListRunArtifacts(ctx context.Context, id pgtype.UUID) ([]db.RunArtifact, error)
	CreateRunEvent(ctx context.Context, arg db.CreateRunEventParams) (db.RunEvent, error)
	ListRunEvents(ctx context.Context, arg db.ListRunEventsParams) ([]db.RunEvent, error)
	ListRunEventsAll(ctx context.Context, arg db.ListRunEventsAllParams) ([]db.RunEvent, error)
}

// RunOrchestrator manages the lifecycle of agent runs: creation, phase
// transitions, step/todo recording, compaction, handoffs, and completion.
type RunOrchestrator struct {
	Queries   RunQuerier
	Compactor *Compactor
	Hub       *realtime.Hub
	Bus       *events.Bus

	// taskLocks protects GetOrCreateRun from concurrent creation for the same task.
	// Keys are task ID strings; values are *sync.Mutex.
	taskLocks sync.Map
}

// NewRunOrchestrator creates a new RunOrchestrator.
func NewRunOrchestrator(queries RunQuerier, compactor *Compactor, hub *realtime.Hub, bus *events.Bus) *RunOrchestrator {
	return &RunOrchestrator{
		Queries:   queries,
		Compactor: compactor,
		Hub:       hub,
		Bus:       bus,
	}
}

// CreateRunRequest contains the parameters for creating a new run.
type CreateRunRequest struct {
	WorkspaceID    string
	IssueID        string
	AgentID        string
	TaskID         string
	ParentRunID    string
	TeamID         string
	SystemPrompt   string
	ModelName      string
	PermissionMode string
}

// CreateRun creates a new run in the pending phase and broadcasts run:created.
func (o *RunOrchestrator) CreateRun(ctx context.Context, req CreateRunRequest) (db.Run, error) {
	params := db.CreateRunParams{
		WorkspaceID:    util.ParseUUID(req.WorkspaceID),
		IssueID:        util.ParseUUID(req.IssueID),
		AgentID:        util.ParseUUID(req.AgentID),
		Phase:          PhasePending,
		Status:         "pending",
		SystemPrompt:   req.SystemPrompt,
		ModelName:      req.ModelName,
		PermissionMode: req.PermissionMode,
		TaskID:         parseNullUUID(req.TaskID),
		ParentRunID:    parseNullUUID(req.ParentRunID),
		TeamID:         parseNullUUID(req.TeamID),
	}

	run, err := o.Queries.CreateRun(ctx, params)
	if err != nil {
		return db.Run{}, fmt.Errorf("create run: %w", err)
	}

	wsID := util.UUIDToString(run.WorkspaceID)
	payload := map[string]any{
		"run_id": util.UUIDToString(run.ID),
		"phase":  run.Phase,
		"status": run.Status,
	}
	if run.AgentID.Valid {
		payload["agent_id"] = util.UUIDToString(run.AgentID)
	}
	if run.IssueID.Valid {
		payload["issue_id"] = util.UUIDToString(run.IssueID)
	}
	if run.TaskID.Valid {
		payload["task_id"] = util.UUIDToString(run.TaskID)
	}
	o.BroadcastRunEvent(ctx, util.UUIDToString(run.ID), wsID, protocol.EventRunCreated, payload)
	slog.Info("run created",
		"run_id", util.UUIDToString(run.ID),
		"issue_id", req.IssueID,
		"agent_id", req.AgentID,
	)

	return run, nil
}

// GetOrCreateRun returns the existing run for a task, or creates a new one atomically.
// It uses per-task locking to prevent duplicate runs from concurrent callers.
func (o *RunOrchestrator) GetOrCreateRun(ctx context.Context, req CreateRunRequest) (db.Run, error) {
	if req.TaskID == "" {
		return o.CreateRun(ctx, req)
	}

	// Per-task lock to prevent concurrent creation for the same task.
	lock, _ := o.taskLocks.LoadOrStore(req.TaskID, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Fast path: check if a run already exists for this task.
	// RunQuerier doesn't expose GetRunByTask, so we try via the concrete Queries
	// field if available, or fall back to creating.
	type taskRunQuerier interface {
		GetRunByTask(ctx context.Context, taskID pgtype.UUID) (db.Run, error)
	}
	if qr, ok := o.Queries.(taskRunQuerier); ok {
		if existing, err := qr.GetRunByTask(ctx, util.ParseUUID(req.TaskID)); err == nil {
			return existing, nil
		}
	}

	// No existing run — create one.
	return o.CreateRun(ctx, req)
}

// StartRun transitions a run from pending to active/executing and broadcasts run:started.
func (o *RunOrchestrator) StartRun(ctx context.Context, runID string) (db.Run, error) {
	run, err := o.Queries.StartRun(ctx, util.ParseUUID(runID))
	if err != nil {
		return db.Run{}, fmt.Errorf("start run: %w", err)
	}

	o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), protocol.EventRunStarted, map[string]any{
		"run_id":     runID,
		"started_at": run.StartedAt.Time.Format(time.RFC3339Nano),
	})
	slog.Info("run started", "run_id", runID)

	return run, nil
}

// AdvancePhase moves the run to a new phase and broadcasts run:phase_changed.
// Returns an error if the transition is not allowed by the run phase state machine.
func (o *RunOrchestrator) AdvancePhase(ctx context.Context, runID string, newPhase string) (db.Run, error) {
	current, readErr := o.Queries.GetRun(ctx, util.ParseUUID(runID))
	if readErr != nil {
		return db.Run{}, fmt.Errorf("get run for phase: %w", readErr)
	}
	oldPhase := current.Phase

	if !CanRunTransition(oldPhase, newPhase) {
		return db.Run{}, fmt.Errorf("cannot transition run from %s to %s", oldPhase, newPhase)
	}

	run, err := o.Queries.UpdateRunPhase(ctx, db.UpdateRunPhaseParams{
		ID:    util.ParseUUID(runID),
		Phase: newPhase,
	})
	if err != nil {
		return db.Run{}, fmt.Errorf("advance phase: %w", err)
	}

	o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), protocol.EventRunPhaseChanged, map[string]any{
		"run_id":     runID,
		"old_phase":  oldPhase,
		"new_phase":  newPhase,
	})
	slog.Info("run phase changed",
		"run_id", runID,
		"old_phase", oldPhase,
		"new_phase", newPhase,
	)

	return run, nil
}

// RecordStep records a step within a run.
// stepType: "thinking", "text", "tool_use", "tool_result", "error".
// callID: correlates tool_use ↔ tool_result pairs (empty for thinking/text/error).
// If toolOutput is empty, the step is started (in-progress). If toolOutput is
// provided, the step is completed immediately.
func (o *RunOrchestrator) RecordStep(ctx context.Context, runID string, stepType string, toolName string, callID string, toolInput []byte, toolOutput string, isError bool) (db.RunStep, error) {
	seq, err := o.Queries.GetNextStepSeq(ctx, util.ParseUUID(runID))
	if err != nil {
		return db.RunStep{}, fmt.Errorf("get next step seq: %w", err)
	}

	var outputVal pgtype.Text
	if toolOutput != "" {
		outputVal = util.StrToText(toolOutput)
	}

	var callIDVal pgtype.Text
	if callID != "" {
		callIDVal = util.StrToText(callID)
	}

	step, err := o.Queries.CreateRunStep(ctx, db.CreateRunStepParams{
		RunID:      util.ParseUUID(runID),
		Seq:        seq,
		ToolName:   toolName,
		ToolInput:  toolInput,
		ToolOutput: outputVal,
		IsError:    isError,
		StartedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		StepType:   stepType,
		CallID:     callIDVal,
	})
	if err != nil {
		return db.RunStep{}, fmt.Errorf("create run step: %w", err)
	}

	eventType := protocol.EventRunStepStarted
	if toolOutput != "" {
		eventType = protocol.EventRunStepCompleted
	}

	// Look up workspace ID for event routing.
	run, runErr := o.Queries.GetRun(ctx, util.ParseUUID(runID))
	if runErr != nil {
		slog.Warn("record step: failed to get run for broadcast", "run_id", runID, "error", runErr)
		return step, nil
	}

	payload := map[string]any{
		"run_id":    runID,
		"step_id":   util.UUIDToString(step.ID),
		"seq":       seq,
		"step_type": stepType,
	}
	if toolName != "" {
		payload["tool_name"] = toolName
	}
	if callID != "" {
		payload["call_id"] = callID
	}
	o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), eventType, payload)

	return step, nil
}

// CompleteStep marks an existing step as completed with output.
func (o *RunOrchestrator) CompleteStep(ctx context.Context, stepID string, toolOutput string, isError bool) (db.RunStep, error) {
	step, err := o.Queries.CompleteRunStep(ctx, db.CompleteRunStepParams{
		ID:         util.ParseUUID(stepID),
		ToolOutput: util.StrToText(toolOutput),
		IsError:    isError,
	})
	if err != nil {
		return db.RunStep{}, fmt.Errorf("complete step: %w", err)
	}

	// Look up workspace ID for event routing.
	run, runErr := o.Queries.GetRun(ctx, step.RunID)
	if runErr != nil {
		slog.Warn("complete step: failed to get run for broadcast", "run_id", util.UUIDToString(step.RunID), "error", runErr)
		return step, nil
	}

	payload := map[string]any{
		"run_id":    util.UUIDToString(step.RunID),
		"step_id":   stepID,
		"step_type": step.StepType,
		"is_error":  isError,
	}
	if step.ToolName != "" {
		payload["tool_name"] = step.ToolName
	}
	if step.CallID.Valid {
		payload["call_id"] = step.CallID.String
	}
	o.BroadcastRunEvent(ctx, util.UUIDToString(step.RunID), util.UUIDToString(run.WorkspaceID), protocol.EventRunStepCompleted, payload)

	return step, nil
}

// CreateTodo creates a new todo item for a run and broadcasts run:todo_created.
func (o *RunOrchestrator) CreateTodo(ctx context.Context, runID string, title string, description string) (db.RunTodo, error) {
	seq, err := o.Queries.GetNextTodoSeq(ctx, util.ParseUUID(runID))
	if err != nil {
		return db.RunTodo{}, fmt.Errorf("get next todo seq: %w", err)
	}

	todo, err := o.Queries.CreateRunTodo(ctx, db.CreateRunTodoParams{
		RunID:       util.ParseUUID(runID),
		Seq:         seq,
		Title:       title,
		Description: description,
		Status:      "pending",
	})
	if err != nil {
		return db.RunTodo{}, fmt.Errorf("create run todo: %w", err)
	}

	// Look up workspace ID for event routing.
	run, runErr := o.Queries.GetRun(ctx, util.ParseUUID(runID))
	if runErr != nil {
		slog.Warn("create todo: failed to get run for broadcast", "run_id", runID, "error", runErr)
		return todo, nil
	}

	o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), protocol.EventRunTodoCreated, map[string]any{
		"todo_id": util.UUIDToString(todo.ID),
		"run_id":  runID,
		"seq":     seq,
		"title":   title,
		"status":  "pending",
	})

	return todo, nil
}

// UpdateTodo updates a todo's status and broadcasts run:todo_updated.
func (o *RunOrchestrator) UpdateTodo(ctx context.Context, todoID string, status string, blocker string) (db.RunTodo, error) {
	var statusVal pgtype.Text
	if status != "" {
		statusVal = util.StrToText(status)
	}

	var blockerVal pgtype.Text
	if blocker != "" {
		blockerVal = util.StrToText(blocker)
	}

	todo, err := o.Queries.UpdateRunTodo(ctx, db.UpdateRunTodoParams{
		ID:      util.ParseUUID(todoID),
		Status:  statusVal,
		Blocker: blockerVal,
	})
	if err != nil {
		return db.RunTodo{}, fmt.Errorf("update run todo: %w", err)
	}

	// Look up workspace ID for event routing.
	run, runErr := o.Queries.GetRun(ctx, todo.RunID)
	if runErr != nil {
		slog.Warn("update todo: failed to get run for broadcast", "run_id", util.UUIDToString(todo.RunID), "error", runErr)
		return todo, nil
	}

	o.BroadcastRunEvent(ctx, util.UUIDToString(todo.RunID), util.UUIDToString(run.WorkspaceID), protocol.EventRunTodoUpdated, map[string]any{
		"todo_id": todoID,
		"run_id":  util.UUIDToString(todo.RunID),
		"status":  status,
		"blocker": blocker,
	})

	return todo, nil
}

// CompleteRun marks a run as completed and broadcasts run:completed.
func (o *RunOrchestrator) CompleteRun(ctx context.Context, runID string) (db.Run, error) {
	run, err := o.Queries.CompleteRun(ctx, util.ParseUUID(runID))
	if err != nil {
		return db.Run{}, fmt.Errorf("complete run: %w", err)
	}

	o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), protocol.EventRunCompleted, map[string]any{
		"run_id":        runID,
		"completed_at":  run.CompletedAt.Time.Format(time.RFC3339),
		"input_tokens":  run.InputTokens,
		"output_tokens": run.OutputTokens,
		"cost_usd":      run.EstimatedCostUsd,
	})
	slog.Info("run completed", "run_id", runID)

	return run, nil
}

// FailRun marks a run as failed and broadcasts run:failed.
func (o *RunOrchestrator) FailRun(ctx context.Context, runID string, errorMsgs ...string) (db.Run, error) {
	run, err := o.Queries.FailRun(ctx, util.ParseUUID(runID))
	if err != nil {
		return db.Run{}, fmt.Errorf("fail run: %w", err)
	}

	errMsg := ""
	if len(errorMsgs) > 0 {
		errMsg = errorMsgs[0]
	}

	o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), protocol.EventRunFailed, map[string]any{
		"run_id":       runID,
		"error":        errMsg,
		"completed_at": run.CompletedAt.Time.Format(time.RFC3339),
	})
	slog.Info("run failed", "run_id", runID)

	return run, nil
}

// RetryRun creates a new run based on an existing run, preserving execution
// history via parent_run_id. The original run's system_prompt, model_name,
// permission_mode, workspace, issue, agent, task, and team are carried over.
// The new run starts in the pending phase.
func (o *RunOrchestrator) RetryRun(ctx context.Context, originalRunID string) (db.Run, error) {
	original, err := o.Queries.GetRun(ctx, util.ParseUUID(originalRunID))
	if err != nil {
		return db.Run{}, fmt.Errorf("get original run: %w", err)
	}

	newRun, err := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID:    util.UUIDToString(original.WorkspaceID),
		IssueID:        util.UUIDToString(original.IssueID),
		AgentID:        util.UUIDToString(original.AgentID),
		TaskID:         nullUUIDToString(original.TaskID),
		ParentRunID:    originalRunID,
		TeamID:         nullUUIDToString(original.TeamID),
		SystemPrompt:   original.SystemPrompt,
		ModelName:      original.ModelName,
		PermissionMode: original.PermissionMode,
	})
	if err != nil {
		return db.Run{}, fmt.Errorf("create retry run: %w", err)
	}

	slog.Info("run retried",
		"original_run_id", originalRunID,
		"new_run_id", util.UUIDToString(newRun.ID),
	)

	return newRun, nil
}

// nullUUIDToString converts a pgtype.UUID to string, returning "" if invalid.
func nullUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return util.UUIDToString(u)
}

// CancelRun marks a run as cancelled and broadcasts run:cancelled.
func (o *RunOrchestrator) CancelRun(ctx context.Context, runID string) (db.Run, error) {
	run, err := o.Queries.CancelRun(ctx, util.ParseUUID(runID))
	if err != nil {
		return db.Run{}, fmt.Errorf("cancel run: %w", err)
	}

	o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), protocol.EventRunCancelled, map[string]any{
		"run_id":       runID,
		"completed_at": run.CompletedAt.Time.Format(time.RFC3339),
	})
	slog.Info("run cancelled", "run_id", runID)

	return run, nil
}

// UpdateTokens increments the token counters on a run.
func (o *RunOrchestrator) UpdateTokens(ctx context.Context, runID string, inputTokens, outputTokens int64, cost float64) error {
	var costVal pgtype.Numeric
	if err := costVal.Scan(fmt.Sprintf("%.6f", cost)); err != nil {
		return fmt.Errorf("scan cost: %w", err)
	}

	_, err := o.Queries.UpdateRunTokens(ctx, db.UpdateRunTokensParams{
		ID:               util.ParseUUID(runID),
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		EstimatedCostUsd: costVal,
	})
	if err != nil {
		return fmt.Errorf("update tokens: %w", err)
	}
	return nil
}

// CreateHandoff records a handoff from one run to another (or to a team/agent).
func (o *RunOrchestrator) CreateHandoff(ctx context.Context, sourceRunID string, handoffType string, reason string, targetRunID string, targetTeamID string, targetAgentID string, contextPacket []byte) (db.RunHandoff, error) {
	handoff, err := o.Queries.CreateRunHandoff(ctx, db.CreateRunHandoffParams{
		SourceRunID:   util.ParseUUID(sourceRunID),
		HandoffType:   handoffType,
		Reason:        reason,
		TargetRunID:   parseNullUUID(targetRunID),
		TargetTeamID:  parseNullUUID(targetTeamID),
		TargetAgentID: parseNullUUID(targetAgentID),
		ContextPacket: contextPacket,
	})
	if err != nil {
		return db.RunHandoff{}, fmt.Errorf("create handoff: %w", err)
	}

	slog.Info("run handoff created",
		"source_run_id", sourceRunID,
		"type", handoffType,
		"target_run_id", targetRunID,
	)

	// Broadcast handoff event.
	if run, runErr := o.Queries.GetRun(ctx, util.ParseUUID(sourceRunID)); runErr == nil {
		payload := map[string]any{
			"run_id":       sourceRunID,
			"handoff_id":   util.UUIDToString(handoff.ID),
			"handoff_type": handoffType,
		}
		if targetRunID != "" {
			payload["target_run_id"] = targetRunID
		}
		if targetTeamID != "" {
			payload["target_team_id"] = targetTeamID
		}
		if targetAgentID != "" {
			payload["target_agent_id"] = targetAgentID
		}
		o.BroadcastRunEvent(ctx, sourceRunID, util.UUIDToString(run.WorkspaceID), protocol.EventRunHandoffCreated, payload)
	}

	return handoff, nil
}

// CreateContinuation saves a continuation packet for a run, enabling
// structured resumption in a future run.
func (o *RunOrchestrator) CreateContinuation(ctx context.Context, runID string, summary string, pendingTodos, keyDecisions, changedFiles, blockers, openQuestions json.RawMessage, tokenBudgetUsed int64) (db.RunContinuation, error) {
	cont, err := o.Queries.CreateRunContinuation(ctx, db.CreateRunContinuationParams{
		RunID:           util.ParseUUID(runID),
		CompactSummary:  summary,
		PendingTodos:    pendingTodos,
		KeyDecisions:    keyDecisions,
		ChangedFiles:    changedFiles,
		Blockers:        blockers,
		OpenQuestions:   openQuestions,
		TokenBudgetUsed: tokenBudgetUsed,
	})
	if err != nil {
		return db.RunContinuation{}, fmt.Errorf("create continuation: %w", err)
	}

	slog.Info("run continuation saved",
		"run_id", runID,
		"token_budget_used", tokenBudgetUsed,
	)

	return cont, nil
}

// CreateArtifact stores an artifact (file output, report, etc.) produced during a run.
func (o *RunOrchestrator) CreateArtifact(ctx context.Context, runID string, stepID string, artifactType string, name string, content string, mimeType string) (db.RunArtifact, error) {
	artifact, err := o.Queries.CreateRunArtifact(ctx, db.CreateRunArtifactParams{
		RunID:        util.ParseUUID(runID),
		StepID:       parseNullUUID(stepID),
		ArtifactType: artifactType,
		Name:         name,
		Content:      content,
		MimeType:     mimeType,
	})
	if err != nil {
		return db.RunArtifact{}, fmt.Errorf("create artifact: %w", err)
	}

	// Broadcast artifact created event.
	if run, runErr := o.Queries.GetRun(ctx, util.ParseUUID(runID)); runErr == nil {
		payload := map[string]any{
			"run_id":        runID,
			"artifact_id":   util.UUIDToString(artifact.ID),
			"artifact_type": artifactType,
			"name":          name,
		}
		if stepID != "" {
			payload["step_id"] = stepID
		}
		o.BroadcastRunEvent(ctx, runID, util.UUIDToString(run.WorkspaceID), protocol.EventRunArtifactCreated, payload)
	}

	return artifact, nil
}

// ListRuns returns runs for a workspace with pagination.
func (o *RunOrchestrator) ListRuns(ctx context.Context, workspaceID string, limit, offset int32) ([]db.Run, error) {
	return o.Queries.ListRunsByWorkspace(ctx, db.ListRunsByWorkspaceParams{
		WorkspaceID: util.ParseUUID(workspaceID),
		Limit:       limit,
		Offset:      offset,
	})
}

// ListRunsByIssue returns all runs for a given issue.
func (o *RunOrchestrator) ListRunsByIssue(ctx context.Context, issueID string) ([]db.Run, error) {
	return o.Queries.ListRunsByIssue(ctx, util.ParseUUID(issueID))
}

// GetRun returns a single run by ID.
func (o *RunOrchestrator) GetRun(ctx context.Context, runID string) (db.Run, error) {
	return o.Queries.GetRun(ctx, util.ParseUUID(runID))
}

// GetRunSteps returns all steps for a run.
func (o *RunOrchestrator) GetRunSteps(ctx context.Context, runID string) ([]db.RunStep, error) {
	return o.Queries.ListRunSteps(ctx, util.ParseUUID(runID))
}

// GetRunTodos returns all todos for a run.
func (o *RunOrchestrator) GetRunTodos(ctx context.Context, runID string) ([]db.RunTodo, error) {
	return o.Queries.ListRunTodos(ctx, util.ParseUUID(runID))
}

// GetRunArtifacts returns all artifacts for a run.
func (o *RunOrchestrator) GetRunArtifacts(ctx context.Context, runID string) ([]db.RunArtifact, error) {
	return o.Queries.ListRunArtifacts(ctx, util.ParseUUID(runID))
}

// ExecuteRunRequest contains the parameters for autonomous run execution.
type ExecuteRunRequest struct {
	RunID          string
	Cwd            string           // working directory for the agent
	Model          string           // model override
	SystemPrompt   string           // system prompt (assembled by caller)
	Prompt         string           // user/task prompt
	Backend        agent.Backend    // agent backend to execute with
	Timeout        time.Duration    // max execution time for the entire run
	MaxTurns       int              // max agent turns (0 = backend default)
	ToolPermissions *agent.ToolPermissions
}

// ExecuteRunResult is the outcome of autonomous run execution.
type ExecuteRunResult struct {
	RunID    string
	Status   string // "completed", "failed", "cancelled"
	Output   string // final text output from the agent
	Error    string // error message if failed
	Steps    int    // total tool steps recorded
	Duration time.Duration
}

// ExecuteRun runs an agent autonomously: start → execute → drain messages as
// steps → check compaction → complete/fail. This is the core orchestration
// loop that connects the run lifecycle to an agent backend.
//
// The caller provides a pre-assembled prompt and an agent.Backend. ExecuteRun
// handles phase transitions, step recording, compaction checkpointing, and
// final completion/failure. It does NOT manage the execution environment
// (workdir, skills injection) — that responsibility belongs to the caller
// (typically the daemon or a handler).
func (o *RunOrchestrator) ExecuteRun(ctx context.Context, req ExecuteRunRequest) (*ExecuteRunResult, error) {
	runID := req.RunID
	log := slog.With("run_id", runID)
	start := time.Now()

	// 1. Start the run (pending → active/executing).
	run, err := o.StartRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("start run: %w", err)
	}
	_ = run

	// 2. Build conversation messages for compaction tracking.
	var messages []Message
	mu := sync.Mutex{} // protects messages during concurrent drain

	addMessage := func(role, content string) {
		mu.Lock()
		messages = append(messages, Message{Role: role, Content: content})
		mu.Unlock()
	}

	// Seed with system + user prompt.
	if req.SystemPrompt != "" {
		addMessage("system", req.SystemPrompt)
	}
	if req.Prompt != "" {
		addMessage("user", req.Prompt)
	}

	// 3. Execute via agent backend.
	session, err := req.Backend.Execute(ctx, req.Prompt, agent.ExecOptions{
		Cwd:              req.Cwd,
		Model:            req.Model,
		SystemPrompt:     req.SystemPrompt,
		MaxTurns:         req.MaxTurns,
		Timeout:          req.Timeout,
		ToolPermissions:  req.ToolPermissions,
	})
	if err != nil {
		log.Error("agent execute failed", "error", err)
		o.FailRun(ctx, runID, fmt.Sprintf("agent execute: %s", err))
		return &ExecuteRunResult{
			RunID:    runID,
			Status:   "failed",
			Error:    fmt.Sprintf("agent execute: %s", err),
			Duration: time.Since(start),
		}, nil
	}

	// 4. Drain messages — coalesce consecutive same-type events, record steps, track for compaction.
	var toolCount int
	toolCallIDToName := map[string]string{} // call_id → tool name

	coalescer := NewCoalescer(350*time.Millisecond, func(ev CoalescedEvent) {
		switch ev.Type {
		case "thinking", "text":
			if ev.Content != "" {
				addMessage("assistant", ev.Content)
			}
		case "tool_use":
			// Tool use steps are recorded immediately (not coalesced).
		case "tool_result":
			// Tool result steps are recorded immediately (not coalesced).
		}
	})

	// StepCoalescer reduces run_steps DB writes by merging consecutive
	// same-type thinking/text events before calling RecordStep.
	stepCoalescer := NewStepCoalescer(350*time.Millisecond, func(stepType, toolName, callID, content string) {
		_, stepErr := o.RecordStep(ctx, runID, stepType, toolName, callID, nil, content, false)
		if stepErr != nil {
			log.Warn("record coalesced step failed", "error", stepErr)
		}
	})

	for msg := range session.Messages {
		switch msg.Type {
		case agent.MessageToolUse:
			toolCount++
			if msg.CallID != "" {
				mu.Lock()
				toolCallIDToName[msg.CallID] = msg.Tool
				mu.Unlock()
			}

			coalescer.PushToolUse(msg.CallID, msg.Tool, msg.Input)

			// Record step start (no output yet). StepCoalescer flushes any pending fold first.
			inputJSON, err := json.Marshal(msg.Input)
			if err != nil {
				slog.Warn("failed to marshal tool input", "tool", msg.Tool, "error", err)
				inputJSON = []byte("{}")
			}
			stepCoalescer.FlushToolUse(msg.CallID, msg.Tool, inputJSON)

			// Add tool_use to conversation for compaction.
			inputStr := string(inputJSON)
			if len(inputStr) > 500 {
				inputStr = inputStr[:500] + "..."
			}
			addMessage("assistant", fmt.Sprintf("[tool_use:%s] %s", msg.Tool, inputStr))

		case agent.MessageToolResult:
			toolName := msg.Tool
			if toolName == "" && msg.CallID != "" {
				mu.Lock()
				toolName = toolCallIDToName[msg.CallID]
				mu.Unlock()
			}

			output := msg.Output
			if len(output) > 8192 {
				output = output[:8192]
			}

			coalescer.PushToolResult(msg.CallID, toolName, output)

			// Record step completion. StepCoalescer flushes any pending fold first.
			stepCoalescer.FlushToolResult(msg.CallID, toolName, output)

			// Add tool_result to conversation for compaction.
			resultPreview := output
			if len(resultPreview) > 500 {
				resultPreview = resultPreview[:500] + "..."
			}
			addMessage("tool", fmt.Sprintf("[tool_result:%s] %s", toolName, resultPreview))

		case agent.MessageText:
			if msg.Content != "" {
				coalescer.Push("text", msg.Content, ClassFold)
				stepCoalescer.PushText(msg.Content)
			}

		case agent.MessageThinking:
			if msg.Content != "" {
				coalescer.Push("thinking", msg.Content, ClassFold)
				stepCoalescer.PushThinking(msg.Content)
			}

		case agent.MessageError:
			log.Error("agent error", "content", msg.Content)
			coalescer.Push("error", msg.Content, ClassFlush)
			addMessage("system", fmt.Sprintf("[error] %s", msg.Content))
		}

		// Check compaction after each message batch.
		if o.Compactor != nil && o.Compactor.NeedsCompaction(messages) {
			coalescer.Close() // flush pending fold events before compaction
			stepCoalescer.Close() // flush pending step events before compaction
			result, compErr := o.Compactor.Compact(ctx, messages, AutoCompact)
			if compErr != nil {
				log.Warn("compaction failed", "error", compErr)
			} else {
				mu.Lock()
				messages = result.Messages
				mu.Unlock()
				log.Info("compacted run context",
					"original_chars", result.OriginalLen,
					"compacted_chars", result.CompactedLen,
					"summary_len", len(result.Summary),
				)
			}
		}
	}

	// Flush any remaining coalesced events.
	coalescer.Close()
	stepCoalescer.Close()

	// 5. Wait for the final result.
	result := <-session.Result
	elapsed := time.Since(start)

	log.Info("agent finished",
		"status", result.Status,
		"duration", elapsed.String(),
		"tools", toolCount,
	)

	// 6. Complete or fail the run based on agent result.
	switch result.Status {
	case "completed":
		if _, err := o.CompleteRun(ctx, runID); err != nil {
			return nil, fmt.Errorf("complete run: %w", err)
		}
		return &ExecuteRunResult{
			RunID:    runID,
			Status:   "completed",
			Output:   result.Output,
			Steps:    toolCount,
			Duration: elapsed,
		}, nil
	case "timeout":
		errMsg := fmt.Sprintf("agent timed out after %s", elapsed)
		o.FailRun(ctx, runID, errMsg)
		return &ExecuteRunResult{
			RunID:    runID,
			Status:   "failed",
			Error:    errMsg,
			Steps:    toolCount,
			Duration: elapsed,
		}, nil
	default:
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("agent %s", result.Status)
		}
		o.FailRun(ctx, runID, errMsg)
		return &ExecuteRunResult{
			RunID:    runID,
			Status:   "failed",
			Error:    errMsg,
			Steps:    toolCount,
			Duration: elapsed,
		}, nil
	}
}

// broadcast sends a run event through the event bus to the specified workspace.
// Each call site constructs the exact per-event payload matching schema §4.
func (o *RunOrchestrator) Broadcast(workspaceID string, eventType string, payload map[string]any) {
	o.Bus.PublishAsync(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

// BroadcastRunEvent persists the event to DB first (at-least-once guarantee),
// then broadcasts via the event bus. The runID is the FK for the persisted row.
func (o *RunOrchestrator) BroadcastRunEvent(ctx context.Context, runID string, workspaceID string, eventType string, payload map[string]any) {
	// Persist to DB before broadcast — if DB write fails, event is lost but
	// the system continues (degraded, not broken). If broadcast fails after
	// DB write, the frontend can catch up via the events API.
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("broadcast run event: failed to marshal payload", "run_id", runID, "event_type", eventType, "error", err)
		payloadJSON = []byte("{}")
	}

	if _, err := o.Queries.CreateRunEvent(ctx, db.CreateRunEventParams{
		RunID:     util.ParseUUID(runID),
		EventType: eventType,
		Payload:   payloadJSON,
	}); err != nil {
		slog.Warn("broadcast run event: failed to persist", "run_id", runID, "event_type", eventType, "error", err)
		// Don't return — still broadcast so live clients get the event.
	}

	o.Broadcast(workspaceID, eventType, payload)
}

// parseNullUUID returns a null UUID if s is empty, otherwise parses it.
func parseNullUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	return util.ParseUUID(s)
}
