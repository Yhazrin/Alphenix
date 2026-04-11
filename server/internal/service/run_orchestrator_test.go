package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// stubQueries implements the subset of db.Queries methods used by
// RunOrchestrator.  Each method is a no-op that returns a zero-value model;
// override individual methods in tests that need specific behaviour.
type stubQueries struct {
	runs          map[string]db.Run
	steps         map[string]db.RunStep
	todos         map[string]db.RunTodo
	handoffs      map[string]db.RunHandoff
	continuations map[string]db.RunContinuation
	artifacts     map[string]db.RunArtifact
	nextID int
	mu     sync.Mutex

	// Error injection fields.
	createRunEventErr error
}

func newStubQueries() *stubQueries {
	return &stubQueries{
		runs:          make(map[string]db.Run),
		steps:         make(map[string]db.RunStep),
		todos:         make(map[string]db.RunTodo),
		handoffs:      make(map[string]db.RunHandoff),
		continuations: make(map[string]db.RunContinuation),
		artifacts:     make(map[string]db.RunArtifact),
	}
}

func (s *stubQueries) nextUUID() pgtype.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	// Deterministic UUID for test readability.
	return util.ParseUUID("00000000-0000-0000-0000-" + padID(s.nextID))
}

func padID(n int) string {
	s := ""
	for i := 0; i < 12; i++ {
		s = "0123456789ab"[n%12:n%12+1] + s
		n /= 12
	}
	return s
}

// --- stub methods (satisfy the Queries interface used by RunOrchestrator) ---

func (s *stubQueries) CreateRun(_ context.Context, p db.CreateRunParams) (db.Run, error) {
	id := s.nextUUID()
	run := db.Run{
		ID:             id,
		WorkspaceID:    p.WorkspaceID,
		IssueID:        p.IssueID,
		TaskID:         p.TaskID,
		AgentID:        p.AgentID,
		ParentRunID:    p.ParentRunID,
		TeamID:         p.TeamID,
		Phase:          p.Phase,
		Status:         p.Status,
		SystemPrompt:   p.SystemPrompt,
		ModelName:      p.ModelName,
		PermissionMode: p.PermissionMode,
	}
	s.mu.Lock()
	s.runs[util.UUIDToString(id)] = run
	s.mu.Unlock()
	return run, nil
}

func (s *stubQueries) StartRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(id)
	run := s.runs[key]
	run.Phase = "executing"
	run.Status = "running"
	run.StartedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	s.runs[key] = run
	return run, nil
}

func (s *stubQueries) UpdateRunPhase(_ context.Context, p db.UpdateRunPhaseParams) (db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(p.ID)
	run := s.runs[key]
	run.Phase = p.Phase
	s.runs[key] = run
	return run, nil
}

func (s *stubQueries) CompleteRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(id)
	run := s.runs[key]
	run.Phase = "completed"
	run.Status = "completed"
	run.CompletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	s.runs[key] = run
	return run, nil
}

func (s *stubQueries) FailRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(id)
	run := s.runs[key]
	run.Phase = "failed"
	run.Status = "failed"
	run.CompletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	s.runs[key] = run
	return run, nil
}

func (s *stubQueries) CancelRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(id)
	run, ok := s.runs[key]
	if !ok {
		return db.Run{}, fmt.Errorf("run not found: %s", key)
	}
	run.Phase = "cancelled"
	run.Status = "cancelled"
	run.CompletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	s.runs[key] = run
	return run, nil
}

func (s *stubQueries) UpdateRunTokens(_ context.Context, _ db.UpdateRunTokensParams) (db.Run, error) {
	return db.Run{}, nil
}

func (s *stubQueries) GetNextStepSeq(_ context.Context, id pgtype.UUID) (int32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := int32(0)
	for _, step := range s.steps {
		if step.RunID == id {
			count++
		}
	}
	return count + 1, nil
}

func (s *stubQueries) CreateRunStep(_ context.Context, p db.CreateRunStepParams) (db.RunStep, error) {
	id := s.nextUUID()
	step := db.RunStep{
		ID:        id,
		RunID:     p.RunID,
		Seq:       p.Seq,
		StepType:  p.StepType,
		ToolName:  p.ToolName,
		CallID:    p.CallID,
		ToolInput:  p.ToolInput,
		ToolOutput: p.ToolOutput,
		IsError:    p.IsError,
	}
	s.mu.Lock()
	s.steps[util.UUIDToString(id)] = step
	s.mu.Unlock()
	return step, nil
}

func (s *stubQueries) CompleteRunStep(_ context.Context, p db.CompleteRunStepParams) (db.RunStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(p.ID)
	step := s.steps[key]
	step.ToolOutput = p.ToolOutput
	step.IsError = p.IsError
	step.CompletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	s.steps[key] = step
	return step, nil
}

func (s *stubQueries) GetNextTodoSeq(_ context.Context, id pgtype.UUID) (int32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := int32(0)
	for _, todo := range s.todos {
		if todo.RunID == id {
			count++
		}
	}
	return count + 1, nil
}

func (s *stubQueries) CreateRunTodo(_ context.Context, p db.CreateRunTodoParams) (db.RunTodo, error) {
	id := s.nextUUID()
	todo := db.RunTodo{
		ID:          id,
		RunID:       p.RunID,
		Seq:         p.Seq,
		Title:       p.Title,
		Description: p.Description,
		Status:      p.Status,
	}
	s.mu.Lock()
	s.todos[util.UUIDToString(id)] = todo
	s.mu.Unlock()
	return todo, nil
}

func (s *stubQueries) UpdateRunTodo(_ context.Context, p db.UpdateRunTodoParams) (db.RunTodo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(p.ID)
	todo := s.todos[key]
	if p.Status.Valid {
		todo.Status = p.Status.String
	}
	if p.Blocker.Valid {
		todo.Blocker = p.Blocker
	}
	s.todos[key] = todo
	return todo, nil
}

// No-op stubs for unused query methods (artifacts, handoffs, continuations).
func (s *stubQueries) CreateRunArtifact(_ context.Context, p db.CreateRunArtifactParams) (db.RunArtifact, error) {
	id := s.nextUUID()
	a := db.RunArtifact{
		ID:           id,
		RunID:        p.RunID,
		StepID:       p.StepID,
		ArtifactType: p.ArtifactType,
		Name:         p.Name,
		Content:      p.Content,
		MimeType:     p.MimeType,
	}
	s.mu.Lock()
	s.artifacts[util.UUIDToString(id)] = a
	s.mu.Unlock()
	return a, nil
}
func (s *stubQueries) CreateRunHandoff(_ context.Context, p db.CreateRunHandoffParams) (db.RunHandoff, error) {
	id := s.nextUUID()
	h := db.RunHandoff{
		ID:            id,
		SourceRunID:   p.SourceRunID,
		HandoffType:   p.HandoffType,
		Reason:        p.Reason,
		TargetRunID:   p.TargetRunID,
		TargetTeamID:  p.TargetTeamID,
		TargetAgentID: p.TargetAgentID,
		ContextPacket: p.ContextPacket,
	}
	s.mu.Lock()
	s.handoffs[util.UUIDToString(id)] = h
	s.mu.Unlock()
	return h, nil
}
func (s *stubQueries) CreateRunContinuation(_ context.Context, p db.CreateRunContinuationParams) (db.RunContinuation, error) {
	id := s.nextUUID()
	c := db.RunContinuation{
		ID:              id,
		RunID:           p.RunID,
		CompactSummary:  p.CompactSummary,
		PendingTodos:    p.PendingTodos,
		KeyDecisions:    p.KeyDecisions,
		ChangedFiles:    p.ChangedFiles,
		Blockers:        p.Blockers,
		OpenQuestions:   p.OpenQuestions,
		TokenBudgetUsed: p.TokenBudgetUsed,
	}
	s.mu.Lock()
	s.continuations[util.UUIDToString(id)] = c
	s.mu.Unlock()
	return c, nil
}

func (s *stubQueries) GetRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := util.UUIDToString(id)
	run, ok := s.runs[key]
	if !ok {
		return db.Run{}, fmt.Errorf("run not found: %s", key)
	}
	return run, nil
}

func (s *stubQueries) GetRunByTask(_ context.Context, taskID pgtype.UUID) (db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, run := range s.runs {
		if run.TaskID == taskID {
			return run, nil
		}
	}
	return db.Run{}, fmt.Errorf("no run found for task %s", util.UUIDToString(taskID))
}

func (s *stubQueries) ListRunsByWorkspace(_ context.Context, p db.ListRunsByWorkspaceParams) ([]db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []db.Run
	for _, r := range s.runs {
		if r.WorkspaceID == p.WorkspaceID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *stubQueries) ListRunsByIssue(_ context.Context, issueID pgtype.UUID) ([]db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []db.Run
	for _, r := range s.runs {
		if r.IssueID == issueID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *stubQueries) ListRunSteps(_ context.Context, id pgtype.UUID) ([]db.RunStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []db.RunStep
	for _, step := range s.steps {
		if step.RunID == id {
			out = append(out, step)
		}
	}
	return out, nil
}
func (s *stubQueries) ListRunTodos(_ context.Context, id pgtype.UUID) ([]db.RunTodo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []db.RunTodo
	for _, todo := range s.todos {
		if todo.RunID == id {
			out = append(out, todo)
		}
	}
	return out, nil
}
func (s *stubQueries) ListRunArtifacts(_ context.Context, runID pgtype.UUID) ([]db.RunArtifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []db.RunArtifact
	for _, a := range s.artifacts {
		if a.RunID == runID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *stubQueries) CreateRunEvent(_ context.Context, _ db.CreateRunEventParams) (db.RunEvent, error) {
	if s.createRunEventErr != nil {
		return db.RunEvent{}, s.createRunEventErr
	}
	return db.RunEvent{}, nil
}

func (s *stubQueries) ListRunEvents(_ context.Context, _ db.ListRunEventsParams) ([]db.RunEvent, error) {
	return nil, nil
}

func (s *stubQueries) ListRunEventsAll(_ context.Context, _ db.ListRunEventsAllParams) ([]db.RunEvent, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Event collector — captures broadcast events for assertion.
// ---------------------------------------------------------------------------

type eventCollector struct {
	mu     sync.Mutex
	events []events.Event
}

func (ec *eventCollector) collect(ev events.Event) {
	ec.mu.Lock()
	ec.events = append(ec.events, ev)
	ec.mu.Unlock()
}

func (ec *eventCollector) all() []events.Event {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	dst := make([]events.Event, len(ec.events))
	copy(dst, ec.events)
	return dst
}

func (ec *eventCollector) byType(t string) []events.Event {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	var out []events.Event
	for _, ev := range ec.events {
		if ev.Type == t {
			out = append(out, ev)
		}
	}
	return out
}

// waitEvents waits briefly for PublishAsync handlers to complete.
func (ec *eventCollector) waitEvents() {
	time.Sleep(20 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// Helper — builds a RunOrchestrator with stubs + event collector.
// ---------------------------------------------------------------------------

func newTestOrchestrator() (*RunOrchestrator, *stubQueries, *eventCollector) {
	stubs := newStubQueries()
	ec := &eventCollector{}
	bus := events.New()
	bus.SubscribeAll(ec.collect)
	o := NewRunOrchestrator(stubs, nil, nil, bus)
	return o, stubs, ec
}

// ---------------------------------------------------------------------------
// Phase Transition Tests
// ---------------------------------------------------------------------------

// Phase transitions + broadcast verification.

func TestRunOrchestrator_PhaseTransitionSpec(t *testing.T) {
	ctx := context.Background()

	req := CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		SystemPrompt:   "test",
		ModelName:      "test-model",
		PermissionMode: "auto",
	}

	t.Run("CreateRun sets phase=pending", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, err := o.CreateRun(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if run.Phase != "pending" {
			t.Errorf("expected phase=pending, got %s", run.Phase)
		}
		if run.Status != "pending" {
			t.Errorf("expected status=pending, got %s", run.Status)
		}
		ec.waitEvents()
		evts := ec.byType("run:created")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:created event, got %d", len(evts))
		}
	})

	t.Run("StartRun transitions pending->executing", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		started, err := o.StartRun(ctx, util.UUIDToString(run.ID))
		if err != nil {
			t.Fatal(err)
		}
		if started.Phase != "executing" {
			t.Errorf("expected phase=executing, got %s", started.Phase)
		}
		if !started.StartedAt.Valid {
			t.Error("expected StartedAt to be valid")
		}
		ec.waitEvents()
		if len(ec.byType("run:started")) != 1 {
			t.Error("expected 1 run:started event")
		}
	})

	t.Run("AdvancePhase moves to new phase", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)

		advanced, err := o.AdvancePhase(ctx, runID, "reviewing")
		if err != nil {
			t.Fatal(err)
		}
		if advanced.Phase != "reviewing" {
			t.Errorf("expected phase=reviewing, got %s", advanced.Phase)
		}
		ec.waitEvents()
		evts := ec.byType("run:phase_changed")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:phase_changed event, got %d", len(evts))
		}
		payload := evts[0].Payload.(map[string]any)
		if payload["new_phase"] != "reviewing" {
			t.Errorf("expected new_phase=reviewing, got %v", payload["new_phase"])
		}
	})

	t.Run("CompleteRun transitions to completed", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)

		completed, err := o.CompleteRun(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if completed.Phase != "completed" {
			t.Errorf("expected phase=completed, got %s", completed.Phase)
		}
		if !completed.CompletedAt.Valid {
			t.Error("expected CompletedAt to be valid")
		}
		ec.waitEvents()
		if len(ec.byType("run:completed")) != 1 {
			t.Error("expected 1 run:completed event")
		}
	})

	t.Run("FailRun transitions to failed", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)

		failed, err := o.FailRun(ctx, runID, "something broke")
		if err != nil {
			t.Fatal(err)
		}
		if failed.Phase != "failed" {
			t.Errorf("expected phase=failed, got %s", failed.Phase)
		}
		if !failed.CompletedAt.Valid {
			t.Error("expected CompletedAt to be valid")
		}
		ec.waitEvents()
		evts := ec.byType("run:failed")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:failed event, got %d", len(evts))
		}
		payload := evts[0].Payload.(map[string]any)
		if payload["error"] != "something broke" {
			t.Errorf("expected error='something broke', got %v", payload["error"])
		}
	})

	t.Run("CancelRun transitions to cancelled", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		cancelled, err := o.CancelRun(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if cancelled.Phase != "cancelled" {
			t.Errorf("expected phase=cancelled, got %s", cancelled.Phase)
		}
		ec.waitEvents()
		if len(ec.byType("run:cancelled")) != 1 {
			t.Error("expected 1 run:cancelled event")
		}
	})

	t.Run("RetryRun creates new run with parent_run_id", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		original, _ := o.CreateRun(ctx, req)
		originalID := util.UUIDToString(original.ID)

		retried, err := o.RetryRun(ctx, originalID)
		if err != nil {
			t.Fatal(err)
		}
		retriedID := util.UUIDToString(retried.ID)
		if retriedID == originalID {
			t.Error("retried run should have a different ID than the original")
		}
		if !retried.ParentRunID.Valid {
			t.Error("expected ParentRunID to be valid")
		}
		if util.UUIDToString(retried.ParentRunID) != originalID {
			t.Errorf("expected parent_run_id=%s, got %s", originalID, util.UUIDToString(retried.ParentRunID))
		}
		if retried.SystemPrompt != original.SystemPrompt {
			t.Errorf("expected system_prompt=%s, got %s", original.SystemPrompt, retried.SystemPrompt)
		}
		if retried.ModelName != original.ModelName {
			t.Errorf("expected model_name=%s, got %s", original.ModelName, retried.ModelName)
		}
		if retried.PermissionMode != original.PermissionMode {
			t.Errorf("expected permission_mode=%s, got %s", original.PermissionMode, retried.PermissionMode)
		}
		if retried.Phase != "pending" {
			t.Errorf("expected phase=pending, got %s", retried.Phase)
		}
		if retried.Status != "pending" {
			t.Errorf("expected status=pending, got %s", retried.Status)
		}
		ec.waitEvents()
		if len(ec.byType("run:created")) != 2 {
			t.Errorf("expected 2 run:created events, got %d", len(ec.byType("run:created")))
		}
	})

	t.Run("RetryRun returns error for nonexistent original", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		_, err := o.RetryRun(ctx, "00000000-0000-0000-0000-999999999999")
		if err == nil {
			t.Error("expected error for nonexistent run, got nil")
		}
	})

	t.Run("full lifecycle: pending->executing->reviewing->completed", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.AdvancePhase(ctx, runID, "reviewing")
		o.CompleteRun(ctx, runID)

		ec.waitEvents()
		if len(ec.byType("run:created")) != 1 {
			t.Error("missing run:created")
		}
		ec.waitEvents()
		if len(ec.byType("run:started")) != 1 {
			t.Error("missing run:started")
		}
		ec.waitEvents()
		if len(ec.byType("run:phase_changed")) != 1 {
			t.Error("missing run:phase_changed")
		}
		ec.waitEvents()
		if len(ec.byType("run:completed")) != 1 {
			t.Error("missing run:completed")
		}
	})

	t.Run("full lifecycle: pending->executing->failed", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.FailRun(ctx, runID, "agent error")

		ec.waitEvents()
		if len(ec.byType("run:created")) != 1 {
			t.Error("missing run:created")
		}
		ec.waitEvents()
		if len(ec.byType("run:started")) != 1 {
			t.Error("missing run:started")
		}
		ec.waitEvents()
		if len(ec.byType("run:failed")) != 1 {
			t.Error("missing run:failed")
		}
	})
}

// ---------------------------------------------------------------------------
// Step Recording Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_RecordStepSpec(t *testing.T) {
	ctx := context.Background()

	// helper: create a run and return its ID
	mustCreateRun := func(t *testing.T, o *RunOrchestrator) string {
		t.Helper()
		run, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		if err != nil {
			t.Fatal(err)
		}
		return util.UUIDToString(run.ID)
	}

	t.Run("RecordStep with output completes immediately", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, err := o.RecordStep(ctx, runID, "tool_use", "read_file", "c1", []byte(`{}`), "output", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.Seq != 1 {
			t.Errorf("expected seq=1, got %d", step.Seq)
		}
		if step.ToolName != "read_file" {
			t.Errorf("expected tool=read_file, got %s", step.ToolName)
		}
		if !step.ToolOutput.Valid {
			t.Error("expected ToolOutput to be valid")
		}
		ec.waitEvents()
		if len(ec.byType("run:step_completed")) != 1 {
			t.Error("expected 1 run:step_completed event")
		}
	})

	t.Run("RecordStep without output starts step", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, err := o.RecordStep(ctx, runID, "tool_use", "bash", "c1", []byte(`{}`), "", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.ToolOutput.Valid {
			t.Error("expected ToolOutput to be invalid (no output)")
		}
		ec.waitEvents()
		if len(ec.byType("run:step_started")) != 1 {
			t.Error("expected 1 run:step_started event")
		}
	})

	t.Run("CompleteStep fills output on existing step", func(t *testing.T) {
		o, stubs, ec := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, _ := o.RecordStep(ctx, runID, "tool_use", "bash", "c1", []byte(`{}`), "", false)
		stepID := util.UUIDToString(step.ID)

		completed, err := o.CompleteStep(ctx, stepID, "output here", false)
		if err != nil {
			t.Fatal(err)
		}
		if !completed.ToolOutput.Valid {
			t.Error("expected ToolOutput to be valid after CompleteStep")
		}
		// Verify stub was updated.
		stored := stubs.steps[stepID]
		if stored.ToolOutput.String != "output here" {
			t.Errorf("stub step output = %q, want 'output here'", stored.ToolOutput.String)
		}
		ec.waitEvents()
		if len(ec.byType("run:step_completed")) != 1 {
			t.Error("expected 1 run:step_completed event")
		}
	})

	t.Run("step seq increments across calls", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		s1, _ := o.RecordStep(ctx, runID, "tool_use", "read_file", "c1", []byte(`{}`), "out1", false)
		s2, _ := o.RecordStep(ctx, runID, "tool_use", "write_file", "c2", []byte(`{}`), "out2", false)
		if s1.Seq != 1 {
			t.Errorf("s1.Seq = %d, want 1", s1.Seq)
		}
		if s2.Seq != 2 {
			t.Errorf("s2.Seq = %d, want 2", s2.Seq)
		}
	})

	t.Run("error step sets is_error=true", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, _ := o.RecordStep(ctx, runID, "tool_use", "bash", "c1", []byte(`{}`), "command not found", true)
		if !step.IsError {
			t.Error("expected IsError=true")
		}
	})

	// --- step_type / call_id tests (migration 047) ---

	t.Run("step_type=thinking records thinking step with empty call_id", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, err := o.RecordStep(ctx, runID, "thinking", "", "", nil, "analysing code...", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.StepType != "thinking" {
			t.Errorf("expected stepType=thinking, got %s", step.StepType)
		}
		if step.ToolName != "" {
			t.Errorf("expected empty toolName, got %s", step.ToolName)
		}
		if step.CallID.Valid {
			t.Error("expected CallID to be invalid (empty)")
		}
		if step.IsError {
			t.Error("expected IsError=false")
		}
	})

	t.Run("step_type=text records text step with empty call_id", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, err := o.RecordStep(ctx, runID, "text", "", "", nil, "Here is the result", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.StepType != "text" {
			t.Errorf("expected stepType=text, got %s", step.StepType)
		}
		if step.CallID.Valid {
			t.Error("expected CallID to be invalid")
		}
		if step.ToolOutput.String != "Here is the result" {
			t.Errorf("expected output='Here is the result', got %q", step.ToolOutput.String)
		}
	})

	t.Run("step_type=tool_use records with call_id", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, err := o.RecordStep(ctx, runID, "tool_use", "read_file", "call-abc", []byte(`{"path":"main.go"}`), "", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.StepType != "tool_use" {
			t.Errorf("expected stepType=tool_use, got %s", step.StepType)
		}
		if step.ToolName != "read_file" {
			t.Errorf("expected tool=read_file, got %s", step.ToolName)
		}
		if step.CallID.String != "call-abc" {
			t.Errorf("expected callID=call-abc, got %s", step.CallID.String)
		}
		if step.ToolOutput.Valid {
			t.Error("expected ToolOutput invalid (no output)")
		}
	})

	t.Run("step_type=tool_result with matching call_id", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		useStep, _ := o.RecordStep(ctx, runID, "tool_use", "read_file", "call-abc", []byte(`{"path":"main.go"}`), "", false)
		resultStep, err := o.RecordStep(ctx, runID, "tool_result", "read_file", "call-abc", nil, "package main...", false)
		if err != nil {
			t.Fatal(err)
		}
		if resultStep.StepType != "tool_result" {
			t.Errorf("expected stepType=tool_result, got %s", resultStep.StepType)
		}
		if resultStep.CallID.String != "call-abc" {
			t.Errorf("expected callID=call-abc, got %s", resultStep.CallID.String)
		}
		if resultStep.ToolOutput.String != "package main..." {
			t.Errorf("expected output='package main...', got %q", resultStep.ToolOutput.String)
		}
		if useStep.CallID.String != resultStep.CallID.String {
			t.Errorf("tool_use and tool_result callIDs don't match: %s vs %s", useStep.CallID.String, resultStep.CallID.String)
		}
	})

	t.Run("empty call_id produces nullable callID field", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, _ := o.RecordStep(ctx, runID, "tool_use", "bash", "", []byte(`{"command":"ls"}`), "file.txt", false)
		if step.CallID.Valid {
			t.Error("expected CallID to be invalid (empty string)")
		}
	})
}

// ---------------------------------------------------------------------------
// Todo Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_TodoSpec(t *testing.T) {
	ctx := context.Background()

	mustCreateRun := func(t *testing.T, o *RunOrchestrator) string {
		t.Helper()
		run, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		if err != nil {
			t.Fatal(err)
		}
		return util.UUIDToString(run.ID)
	}

	t.Run("CreateTodo sets pending status", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		todo, err := o.CreateTodo(ctx, runID, "Fix bug", "Critical")
		if err != nil {
			t.Fatal(err)
		}
		if todo.Status != "pending" {
			t.Errorf("expected status=pending, got %s", todo.Status)
		}
		if todo.Seq != 1 {
			t.Errorf("expected seq=1, got %d", todo.Seq)
		}
		if todo.Title != "Fix bug" {
			t.Errorf("expected title='Fix bug', got %s", todo.Title)
		}
		ec.waitEvents()
		if len(ec.byType("run:todo_created")) != 1 {
			t.Error("expected 1 run:todo_created event")
		}
	})

	t.Run("UpdateTodo changes status", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		todo, _ := o.CreateTodo(ctx, runID, "Fix bug", "")
		todoID := util.UUIDToString(todo.ID)

		updated, err := o.UpdateTodo(ctx, todoID, "completed", "")
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != "completed" {
			t.Errorf("expected status=completed, got %s", updated.Status)
		}
		ec.waitEvents()
		if len(ec.byType("run:todo_updated")) != 1 {
			t.Error("expected 1 run:todo_updated event")
		}
	})

	t.Run("UpdateTodo with blocker sets blocker text", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		todo, _ := o.CreateTodo(ctx, runID, "Deploy", "")
		todoID := util.UUIDToString(todo.ID)

		updated, _ := o.UpdateTodo(ctx, todoID, "blocked", "Waiting for approval")
		if !updated.Blocker.Valid {
			t.Fatal("expected Blocker to be valid")
		}
		if updated.Blocker.String != "Waiting for approval" {
			t.Errorf("expected blocker='Waiting for approval', got %s", updated.Blocker.String)
		}
	})

	t.Run("todo seq increments across calls", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		t1, _ := o.CreateTodo(ctx, runID, "Task 1", "")
		t2, _ := o.CreateTodo(ctx, runID, "Task 2", "")
		if t1.Seq != 1 {
			t.Errorf("t1.Seq = %d, want 1", t1.Seq)
		}
		if t2.Seq != 2 {
			t.Errorf("t2.Seq = %d, want 2", t2.Seq)
		}
	})
}

// ---------------------------------------------------------------------------
// Broadcast / Event Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_BroadcastSpec(t *testing.T) {
	ctx := context.Background()

	req := CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		SystemPrompt:   "test",
		ModelName:      "test-model",
		PermissionMode: "auto",
	}

	t.Run("every lifecycle method broadcasts exactly one event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		ec.waitEvents()
		if len(ec.byType("run:created")) != 1 {
			t.Fatal("expected 1 run:created event")
		}

		o.StartRun(ctx, runID)
		ec.waitEvents()
		if len(ec.byType("run:started")) != 1 {
			t.Fatal("expected 1 run:started event")
		}

		o.CompleteRun(ctx, runID)
		ec.waitEvents()
		if len(ec.byType("run:completed")) != 1 {
			t.Fatal("expected 1 run:completed event")
		}
	})

	t.Run("event payload contains run_id", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		ec.waitEvents()
		evts := ec.byType("run:created")
		if len(evts) != 1 {
			t.Fatal("expected 1 run:created event")
		}
		payload := evts[0].Payload.(map[string]any)
		if payload["run_id"] != runID {
			t.Errorf("expected payload run_id=%s, got %v", runID, payload["run_id"])
		}
	})

	t.Run("step events contain tool_name and seq", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.RecordStep(ctx, runID, "tool_use", "read_file", "c1", []byte(`{}`), "output", false)
		ec.waitEvents()
		evts := ec.byType("run:step_completed")
		if len(evts) != 1 {
			t.Fatal("expected 1 run:step_completed event")
		}
		payload := evts[0].Payload.(map[string]any)
		if payload["tool_name"] != "read_file" {
			t.Errorf("expected tool_name=read_file, got %v", payload["tool_name"])
		}
		if payload["seq"] != int32(1) {
			t.Errorf("expected seq=1, got %v", payload["seq"])
		}
	})

	t.Run("workspace_id is set on all events", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)
		ec.waitEvents()
		for _, ev := range ec.all() {
			if ev.WorkspaceID == "" {
			t.Errorf("event type %s has empty WorkspaceID", ev.Type)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Coalescer → Orchestrator integration (no DB needed)
// ---------------------------------------------------------------------------

func TestStepCoalescer_OrchestratorWriteFn(t *testing.T) {
	t.Run("coalesced thinking produces stepType=thinking, callID empty", func(t *testing.T) {
		var mu sync.Mutex
		var writes []stepWrite

		sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
			mu.Lock()
			writes = append(writes, stepWrite{stepType, toolName, callID, content})
			mu.Unlock()
		})

		sc.PushThinking("analysing...")
		sc.PushThinking(" more analysis")
		time.Sleep(150 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if len(writes) != 1 {
			t.Fatalf("expected 1 write, got %d", len(writes))
		}
		if writes[0].StepType != "thinking" {
			t.Errorf("expected stepType=thinking, got %s", writes[0].StepType)
		}
		if writes[0].CallID != "" {
			t.Errorf("expected empty callID for thinking, got %s", writes[0].CallID)
		}
	})

	t.Run("tool_use passes stepType=tool_use + callID", func(t *testing.T) {
		var mu sync.Mutex
		var writes []stepWrite

		sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
			mu.Lock()
			writes = append(writes, stepWrite{stepType, toolName, callID, content})
			mu.Unlock()
		})

		sc.FlushToolUse("call-123", "read_file", []byte(`{"path":"test.go"}`))
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if len(writes) != 1 {
			t.Fatalf("expected 1 write, got %d", len(writes))
		}
		if writes[0].StepType != "tool_use" {
			t.Errorf("expected stepType=tool_use, got %s", writes[0].StepType)
		}
		if writes[0].ToolName != "read_file" {
			t.Errorf("expected toolName=read_file, got %s", writes[0].ToolName)
		}
		if writes[0].CallID != "call-123" {
			t.Errorf("expected callID=call-123, got %s", writes[0].CallID)
		}
	})

	t.Run("tool_result passes stepType=tool_result + callID", func(t *testing.T) {
		var mu sync.Mutex
		var writes []stepWrite

		sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
			mu.Lock()
			writes = append(writes, stepWrite{stepType, toolName, callID, content})
			mu.Unlock()
		})

		sc.FlushToolResult("call-123", "read_file", "file contents here")
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if len(writes) != 1 {
			t.Fatalf("expected 1 write, got %d", len(writes))
		}
		if writes[0].StepType != "tool_result" {
			t.Errorf("expected stepType=tool_result, got %s", writes[0].StepType)
		}
		if writes[0].CallID != "call-123" {
			t.Errorf("expected callID=call-123, got %s", writes[0].CallID)
		}
	})

	t.Run("mixed sequence: thinking->tool_use->tool_result", func(t *testing.T) {
		var mu sync.Mutex
		var writes []stepWrite

		sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
			mu.Lock()
			writes = append(writes, stepWrite{stepType, toolName, callID, content})
			mu.Unlock()
		})

		// Simulate a realistic drain-loop sequence.
		sc.PushThinking("Let me read the file...")
		sc.FlushToolUse("c1", "read_file", []byte(`{"path":"main.go"}`))
		sc.FlushToolResult("c1", "read_file", "package main...")
		sc.PushText("Here's the file content.")
		time.Sleep(150 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// thinking (fold flush) + tool_use + tool_result + text (fold flush) = 4
		if len(writes) != 4 {
			t.Fatalf("expected 4 writes, got %d: %v", len(writes), writes)
		}

		// Verify stepType sequence.
		expected := []string{"thinking", "tool_use", "tool_result", "text"}
		for i, w := range writes {
			if w.StepType != expected[i] {
				t.Errorf("write[%d]: expected stepType=%s, got %s", i, expected[i], w.StepType)
			}
		}

		// tool_use and tool_result share callID.
		if writes[1].CallID != "c1" || writes[2].CallID != "c1" {
			t.Errorf("tool_use/tool_result should share callID=c1, got %s and %s",
				writes[1].CallID, writes[2].CallID)
		}
		})

		t.Run("text step has empty callID", func(t *testing.T) {
			var mu sync.Mutex
			var writes []stepWrite

			sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
				mu.Lock()
				writes = append(writes, stepWrite{stepType, toolName, callID, content})
				mu.Unlock()
			})

			sc.PushText("Here is the analysis result.")
			time.Sleep(150 * time.Millisecond)

			mu.Lock()
			defer mu.Unlock()

			if len(writes) != 1 {
				t.Fatalf("expected 1 write, got %d", len(writes))
			}
			if writes[0].StepType != "text" {
				t.Errorf("expected stepType=text, got %s", writes[0].StepType)
			}
			if writes[0].CallID != "" {
				t.Errorf("expected empty callID for text, got %s", writes[0].CallID)
			}
			if writes[0].Content != "Here is the analysis result." {
				t.Errorf("expected content preserved, got %s", writes[0].Content)
			}
		})

		t.Run("tool_use with empty callID passes empty callID", func(t *testing.T) {
			var mu sync.Mutex
			var writes []stepWrite

			sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
				mu.Lock()
				writes = append(writes, stepWrite{stepType, toolName, callID, content})
				mu.Unlock()
			})

			sc.FlushToolUse("", "bash", []byte(`{"command":"ls"}`))
			time.Sleep(100 * time.Millisecond)

			mu.Lock()
			defer mu.Unlock()

			if len(writes) != 1 {
				t.Fatalf("expected 1 write, got %d", len(writes))
			}
			if writes[0].StepType != "tool_use" {
				t.Errorf("expected stepType=tool_use, got %s", writes[0].StepType)
			}
			if writes[0].CallID != "" {
				t.Errorf("expected empty callID, got %s", writes[0].CallID)
			}
		})

		t.Run("tool_result with empty callID passes empty callID", func(t *testing.T) {
			var mu sync.Mutex
			var writes []stepWrite

			sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
				mu.Lock()
				writes = append(writes, stepWrite{stepType, toolName, callID, content})
				mu.Unlock()
			})

			sc.FlushToolResult("", "bash", "output")
			time.Sleep(100 * time.Millisecond)

			mu.Lock()
			defer mu.Unlock()

			if len(writes) != 1 {
				t.Fatalf("expected 1 write, got %d", len(writes))
			}
			if writes[0].StepType != "tool_result" {
				t.Errorf("expected stepType=tool_result, got %s", writes[0].StepType)
			}
			if writes[0].CallID != "" {
				t.Errorf("expected empty callID, got %s", writes[0].CallID)
			}
		})

		t.Run("error tool_result preserves callID pairing", func(t *testing.T) {
			var mu sync.Mutex
			var writes []stepWrite

			sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
				mu.Lock()
				writes = append(writes, stepWrite{stepType, toolName, callID, content})
				mu.Unlock()
			})

			sc.FlushToolUse("err-1", "bash", []byte(`{"command":"rm -rf /"}`))
			sc.FlushToolResult("err-1", "bash", "permission denied")
			time.Sleep(150 * time.Millisecond)

			mu.Lock()
			defer mu.Unlock()

			if len(writes) != 2 {
				t.Fatalf("expected 2 writes, got %d", len(writes))
			}
			if writes[0].CallID != "err-1" || writes[1].CallID != "err-1" {
				t.Errorf("both writes should share callID=err-1, got %s and %s",
					writes[0].CallID, writes[1].CallID)
			}
			if writes[1].Content != "permission denied" {
				t.Errorf("expected error content preserved, got %s", writes[1].Content)
			}
		})
	}

// ---------------------------------------------------------------------------
// Token counter test (no DB needed — pure call verification)
// ---------------------------------------------------------------------------

func TestRunOrchestrator_UpdateTokensSpec(t *testing.T) {
	t.Run("UpdateTokens calls Queries.UpdateRunTokens without error", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(context.Background(), CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)
		err := o.UpdateTokens(context.Background(), runID, 1000, 500, 0.045)
		if err != nil {
			t.Fatalf("UpdateTokens returned error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Fork Helpers — test fixtures for #26 fork testing
// ---------------------------------------------------------------------------

// forkWorkspaceID is a fixed workspace UUID for fork tests.
const forkWorkspaceID = "00000000-0000-0000-0000-0000000000a1"

// createParentRun creates a parent run with the given task ID, returning the
// run and its string ID.  Convenience for fork handler tests.
func createParentRun(t *testing.T, o *RunOrchestrator, taskID string) (db.Run, string) {
	t.Helper()
	run, err := o.CreateRun(context.Background(), CreateRunRequest{
		WorkspaceID: forkWorkspaceID,
		IssueID:     "00000000-0000-0000-0000-0000000000a2",
		AgentID:     "00000000-0000-0000-0000-0000000000a3",
		TaskID:      taskID,
	})
	if err != nil {
		t.Fatalf("createParentRun: %v", err)
	}
	return run, util.UUIDToString(run.ID)
}

// createChildRun creates a child run linked to the parent, returning the
// run and its string ID.
func createChildRun(t *testing.T, o *RunOrchestrator, taskID, parentRunID string) (db.Run, string) {
	t.Helper()
	run, err := o.CreateRun(context.Background(), CreateRunRequest{
		WorkspaceID: forkWorkspaceID,
		IssueID:     "00000000-0000-0000-0000-0000000000a2",
		AgentID:     "00000000-0000-0000-0000-0000000000a3",
		TaskID:      taskID,
		ParentRunID: parentRunID,
	})
	if err != nil {
		t.Fatalf("createChildRun: %v", err)
	}
	return run, util.UUIDToString(run.ID)
}

// ---------------------------------------------------------------------------
// Fork Tests — parent_run_id verification and fork broadcast events
// ---------------------------------------------------------------------------

func TestRunOrchestrator_ForkCreateSpec(t *testing.T) {
	ctx := context.Background()

	t.Run("child run has correct ParentRunID", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		_, parentID := createParentRun(t, o, "task-parent")
		child, childID := createChildRun(t, o, "task-child-1", parentID)

		if util.UUIDToString(child.ParentRunID) != parentID {
			t.Errorf("expected ParentRunID %s, got %s", parentID, util.UUIDToString(child.ParentRunID))
		}
		_ = childID
	})

	t.Run("child run has different ID from parent", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		_, parentID := createParentRun(t, o, "task-parent")
		_, childID := createChildRun(t, o, "task-child-2", parentID)

		if childID == parentID {
			t.Error("child run ID should differ from parent run ID")
		}
	})

	t.Run("GetRunByTask returns existing parent run", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		_, parentID := createParentRun(t, o, "task-findme")

		existing, err := stubs.GetRunByTask(ctx, util.ParseUUID("task-findme"))
		if err != nil {
			t.Fatalf("GetRunByTask: %v", err)
		}
		if util.UUIDToString(existing.ID) != parentID {
			t.Errorf("expected run ID %s, got %s", parentID, util.UUIDToString(existing.ID))
		}
	})

	t.Run("GetOrCreateRun reuses existing run for same task", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		parent, parentID := createParentRun(t, o, "task-reuse")

		again, err := o.GetOrCreateRun(ctx, CreateRunRequest{
			WorkspaceID: forkWorkspaceID,
			IssueID:     "00000000-0000-0000-0000-0000000000a2",
			AgentID:     "00000000-0000-0000-0000-0000000000a3",
			TaskID:      "task-reuse",
		})
		if err != nil {
			t.Fatalf("GetOrCreateRun: %v", err)
		}
		if util.UUIDToString(again.ID) != parentID {
			t.Errorf("expected same run ID %s, got %s", parentID, util.UUIDToString(again.ID))
		}
		_ = parent
	})
}

func TestRunOrchestrator_ForkBroadcastSpec(t *testing.T) {
	t.Run("Broadcast fork_started event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		_, parentID := createParentRun(t, o, "task-bcast-1")
		_, childID := createChildRun(t, o, "task-fork-bcast-1", parentID)

		o.Broadcast(forkWorkspaceID, "agent:fork_started", map[string]any{
			"fork_id":       "fork-1",
			"parent_run_id": parentID,
			"child_run_id":  childID,
			"role":          "researcher",
		})
		ec.waitEvents()

		events := ec.byType("agent:fork_started")
		if len(events) != 1 {
			t.Fatalf("expected 1 fork_started event, got %d", len(events))
		}
		payload, ok := events[0].Payload.(map[string]any)
			if !ok {
				t.Fatal("payload is not map[string]any")
			}
		if payload["fork_id"] != "fork-1" {
			t.Errorf("expected fork_id=fork-1, got %v", payload["fork_id"])
		}
		if payload["parent_run_id"] != parentID {
			t.Errorf("expected parent_run_id=%s, got %v", parentID, payload["parent_run_id"])
		}
		if payload["child_run_id"] != childID {
			t.Errorf("expected child_run_id=%s, got %v", childID, payload["child_run_id"])
		}
	})

	t.Run("Broadcast fork_completed event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		_, parentID := createParentRun(t, o, "task-bcast-2")
		_, childID := createChildRun(t, o, "task-fork-bcast-2", parentID)

		o.Broadcast(forkWorkspaceID, "agent:fork_completed", map[string]any{
			"fork_id":       "fork-2",
			"parent_run_id": parentID,
			"child_run_id":  childID,
			"duration_ms":   1500,
		})
		ec.waitEvents()

		events := ec.byType("agent:fork_completed")
		if len(events) != 1 {
			t.Fatalf("expected 1 fork_completed event, got %d", len(events))
		}
		payload, ok := events[0].Payload.(map[string]any)
			if !ok {
				t.Fatal("payload is not map[string]any")
			}
		if payload["fork_id"] != "fork-2" {
			t.Errorf("expected fork_id=fork-2, got %v", payload["fork_id"])
		}
		if payload["duration_ms"] != 1500 {
			t.Errorf("expected duration_ms=1500, got %v", payload["duration_ms"])
		}
	})

	t.Run("Broadcast fork_failed event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		_, parentID := createParentRun(t, o, "task-bcast-3")
		_, childID := createChildRun(t, o, "task-fork-bcast-3", parentID)

		o.Broadcast(forkWorkspaceID, "agent:fork_failed", map[string]any{
			"fork_id":       "fork-3",
			"parent_run_id": parentID,
			"child_run_id":  childID,
			"error":         "context deadline exceeded",
		})
		ec.waitEvents()

		events := ec.byType("agent:fork_failed")
		if len(events) != 1 {
			t.Fatalf("expected 1 fork_failed event, got %d", len(events))
		}
		payload, ok := events[0].Payload.(map[string]any)
			if !ok {
				t.Fatal("payload is not map[string]any")
			}
		if payload["error"] != "context deadline exceeded" {
			t.Errorf("expected error message, got %v", payload["error"])
		}
	})
}

// ---------------------------------------------------------------------------
// CanRunTransition unit tests — exhaustive valid/invalid matrix
// ---------------------------------------------------------------------------

func TestCanRunTransition_ValidTransitionMatrix(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
	}{
		{"pending → planning", PhasePending, PhasePlanning},
		{"pending → executing", PhasePending, PhaseExecuting},
		{"pending → failed", PhasePending, PhaseFailed},
		{"pending → cancelled", PhasePending, PhaseCancelled},
		{"planning → executing", PhasePlanning, PhaseExecuting},
		{"planning → failed", PhasePlanning, PhaseFailed},
		{"planning → cancelled", PhasePlanning, PhaseCancelled},
		{"executing → reviewing", PhaseExecuting, PhaseReviewing},
		{"executing → completed", PhaseExecuting, PhaseCompleted},
		{"executing → failed", PhaseExecuting, PhaseFailed},
		{"executing → cancelled", PhaseExecuting, PhaseCancelled},
		{"reviewing → completed", PhaseReviewing, PhaseCompleted},
		{"reviewing → failed", PhaseReviewing, PhaseFailed},
		{"reviewing → cancelled", PhaseReviewing, PhaseCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !CanRunTransition(tt.from, tt.to) {
				t.Fatalf("expected CanRunTransition(%q, %q) to be true", tt.from, tt.to)
			}
		})
	}
}

func TestCanRunTransition_InvalidTransitions(t *testing.T) {
	allPhases := []string{PhasePending, PhasePlanning, PhaseExecuting, PhaseReviewing, PhaseCompleted, PhaseFailed, PhaseCancelled}

	validSet := map[[2]string]bool{
		{PhasePending, PhasePlanning}:    true,
		{PhasePending, PhaseExecuting}:   true,
		{PhasePending, PhaseFailed}:      true,
		{PhasePending, PhaseCancelled}:   true,
		{PhasePlanning, PhaseExecuting}:  true,
		{PhasePlanning, PhaseFailed}:     true,
		{PhasePlanning, PhaseCancelled}:  true,
		{PhaseExecuting, PhaseReviewing}: true,
		{PhaseExecuting, PhaseCompleted}: true,
		{PhaseExecuting, PhaseFailed}:    true,
		{PhaseExecuting, PhaseCancelled}: true,
		{PhaseReviewing, PhaseCompleted}: true,
		{PhaseReviewing, PhaseFailed}:    true,
		{PhaseReviewing, PhaseCancelled}: true,
	}

	for _, from := range allPhases {
		for _, to := range allPhases {
			pair := [2]string{from, to}
			if validSet[pair] {
				continue
			}
			t.Run(from+" → "+to, func(t *testing.T) {
				if CanRunTransition(from, to) {
					t.Fatalf("expected CanRunTransition(%q, %q) to be false", from, to)
				}
			})
		}
	}
}

func TestCanRunTransition_UnknownPhase(t *testing.T) {
	if CanRunTransition("bogus", PhaseExecuting) {
		t.Fatal("expected CanRunTransition('bogus', 'executing') to be false")
	}
	if CanRunTransition(PhasePending, "bogus") {
		t.Fatal("expected CanRunTransition('pending', 'bogus') to be false")
	}
}

// ---------------------------------------------------------------------------
// AdvancePhase boundary tests — integration-level rejection of invalid transitions
// ---------------------------------------------------------------------------

func TestRunOrchestrator_AdvancePhase_InvalidTransitionReturnsError(t *testing.T) {
	ctx := context.Background()

	req := CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		SystemPrompt:   "test",
		ModelName:      "test-model",
		PermissionMode: "auto",
	}

	t.Run("completed→executing is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)

		_, err := o.AdvancePhase(ctx, runID, "executing")
		if err == nil {
			t.Fatal("expected error transitioning completed→executing")
		}
		if want := "cannot transition run from completed to executing"; err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("completed→planning is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)

		_, err := o.AdvancePhase(ctx, runID, "planning")
		if err == nil {
			t.Fatal("expected error transitioning completed→planning")
		}
	})

	t.Run("failed→executing is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.FailRun(ctx, runID, "boom")

		_, err := o.AdvancePhase(ctx, runID, "executing")
		if err == nil {
			t.Fatal("expected error transitioning failed→executing")
		}
	})

	t.Run("failed→reviewing is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.FailRun(ctx, runID, "boom")

		_, err := o.AdvancePhase(ctx, runID, "reviewing")
		if err == nil {
			t.Fatal("expected error transitioning failed→reviewing")
		}
	})

	t.Run("cancelled→executing is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.CancelRun(ctx, runID)

		_, err := o.AdvancePhase(ctx, runID, "executing")
		if err == nil {
			t.Fatal("expected error transitioning cancelled→executing")
		}
	})

	t.Run("cancelled→reviewing is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.CancelRun(ctx, runID)

		_, err := o.AdvancePhase(ctx, runID, "reviewing")
		if err == nil {
			t.Fatal("expected error transitioning cancelled→reviewing")
		}
	})

	t.Run("executing→planning is rejected (no backward)", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)

		_, err := o.AdvancePhase(ctx, runID, "planning")
		if err == nil {
			t.Fatal("expected error transitioning executing→planning")
		}
	})

	t.Run("executing→pending is rejected (no backward)", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)

		_, err := o.AdvancePhase(ctx, runID, "pending")
		if err == nil {
			t.Fatal("expected error transitioning executing→pending")
		}
	})

	t.Run("reviewing→executing is rejected (no backward)", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.AdvancePhase(ctx, runID, "reviewing")

		_, err := o.AdvancePhase(ctx, runID, "executing")
		if err == nil {
			t.Fatal("expected error transitioning reviewing→executing")
		}
	})

	t.Run("reviewing→planning is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.AdvancePhase(ctx, runID, "reviewing")

		_, err := o.AdvancePhase(ctx, runID, "planning")
		if err == nil {
			t.Fatal("expected error transitioning reviewing→planning")
		}
	})

	t.Run("planning→pending is rejected (no backward)", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		// CreateRun sets phase=pending, so advance to planning first.
		o.AdvancePhase(ctx, runID, "planning")

		_, err := o.AdvancePhase(ctx, runID, "pending")
		if err == nil {
			t.Fatal("expected error transitioning planning→pending")
		}
	})

	t.Run("unknown phase is rejected", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		_, err := o.AdvancePhase(ctx, runID, "bogus_phase")
		if err == nil {
			t.Fatal("expected error transitioning pending→bogus_phase")
		}
	})

	t.Run("AdvancePhase does not mutate phase on rejection", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)

		// Attempt invalid transition.
		o.AdvancePhase(ctx, runID, "executing")

		// Verify phase is still "completed".
		stored, err := stubs.GetRun(ctx, util.ParseUUID(runID))
		if err != nil {
			t.Fatal(err)
		}
		if stored.Phase != "completed" {
			t.Errorf("expected phase to remain 'completed', got %s", stored.Phase)
		}
	})

	t.Run("no broadcast event on invalid transition", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)
		ec.waitEvents()

		// Clear previous events count baseline.
		beforeCount := len(ec.byType("run:phase_changed"))

		o.AdvancePhase(ctx, runID, "executing")
		ec.waitEvents()

		afterCount := len(ec.byType("run:phase_changed"))
		if afterCount != beforeCount {
			t.Errorf("expected no new phase_changed event on rejection, before=%d after=%d", beforeCount, afterCount)
		}
	})

	t.Run("GetRun failure propagates from AdvancePhase", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		// Use a run ID that doesn't exist.
		fakeID := "00000000-0000-0000-0000-deadbeef0001"
		_, err := o.AdvancePhase(ctx, fakeID, "executing")
		if err == nil {
			t.Fatal("expected error for nonexistent run")
		}
	})
}

// ---------------------------------------------------------------------------
// RecordStep boundary tests — step_type/call_id edge cases
// ---------------------------------------------------------------------------

func TestRunOrchestrator_RecordStep_Boundary(t *testing.T) {
	ctx := context.Background()

	mustCreateRun := func(t *testing.T, o *RunOrchestrator) string {
		t.Helper()
		run, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		if err != nil {
			t.Fatal(err)
		}
		return util.UUIDToString(run.ID)
	}

	t.Run("step_type=error records error step", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, err := o.RecordStep(ctx, runID, "error", "", "", nil, "something went wrong", true)
		if err != nil {
			t.Fatal(err)
		}
		if step.StepType != "error" {
			t.Errorf("expected stepType=error, got %s", step.StepType)
		}
		if !step.IsError {
			t.Error("expected IsError=true for error step")
		}
		if step.ToolOutput.String != "something went wrong" {
			t.Errorf("expected output='something went wrong', got %q", step.ToolOutput.String)
		}
	})

	t.Run("multiple tool_use with same callID are allowed", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		s1, _ := o.RecordStep(ctx, runID, "tool_use", "bash", "dup-call", []byte(`{}`), "", false)
		s2, _ := o.RecordStep(ctx, runID, "tool_use", "bash", "dup-call", []byte(`{}`), "", false)
		if s1.CallID.String != "dup-call" || s2.CallID.String != "dup-call" {
			t.Errorf("both steps should have callID=dup-call")
		}
		if s1.Seq == s2.Seq {
			t.Error("steps should have different seq numbers even with same callID")
		}
	})

	t.Run("tool_result before tool_use still records correctly", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		// Deliberately send tool_result before tool_use — orchestrator shouldn't care about ordering.
		step, err := o.RecordStep(ctx, runID, "tool_result", "read_file", "out-of-order", nil, "file contents", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.StepType != "tool_result" {
			t.Errorf("expected stepType=tool_result, got %s", step.StepType)
		}
		if step.CallID.String != "out-of-order" {
			t.Errorf("expected callID=out-of-order, got %s", step.CallID.String)
		}
	})

	t.Run("nil tool_input is accepted", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		step, err := o.RecordStep(ctx, runID, "thinking", "", "", nil, "thinking aloud", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.ToolInput != nil {
			// nil is fine; just verify it doesn't crash.
			t.Logf("tool_input = %v", step.ToolInput)
		}
	})

	t.Run("very long callID is preserved", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun(t, o)
		longCallID := "call-" + string(make([]byte, 500))
		for i := range longCallID[5:] {
			longCallID = longCallID[:5+i] + "a" + longCallID[6+i:]
		}
		step, err := o.RecordStep(ctx, runID, "tool_use", "bash", longCallID, []byte(`{}`), "", false)
		if err != nil {
			t.Fatal(err)
		}
		if step.CallID.String != longCallID {
			t.Error("long callID was not preserved")
		}
	})

	t.Run("step across different runs has independent seq", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run1 := mustCreateRun(t, o)
		run2 := mustCreateRun(t, o)
		s1, _ := o.RecordStep(ctx, run1, "tool_use", "bash", "r1c1", []byte(`{}`), "out1", false)
		s2, _ := o.RecordStep(ctx, run2, "tool_use", "bash", "r2c1", []byte(`{}`), "out2", false)
		if s1.Seq != 1 {
			t.Errorf("run1 first step seq = %d, want 1", s1.Seq)
		}
		if s2.Seq != 1 {
			t.Errorf("run2 first step seq = %d, want 1", s2.Seq)
		}
	})

	t.Run("CompleteStep on nonexistent step is a no-op with stub (DB would error)", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		fakeStepID := "00000000-0000-0000-0000-deadbeef0099"
		// The stub CompleteRunStep is a no-op that doesn't check existence.
		// In production, the DB would return sql.ErrNoRows. This test documents
		// the stub boundary — CompleteStep itself doesn't add extra validation.
		step, err := o.CompleteStep(ctx, fakeStepID, "output", false)
		if err != nil {
			t.Fatalf("stub does not error on nonexistent step (expected): %v", err)
		}
		if step.ToolOutput.String != "output" {
			t.Errorf("stub should pass through output, got %q", step.ToolOutput.String)
		}
	})
}

// ---------------------------------------------------------------------------
// Terminal state rejection — orchestrator-level boundary tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_TerminalStateRejection(t *testing.T) {
	ctx := context.Background()

	req := CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		SystemPrompt:   "test",
		ModelName:      "test-model",
		PermissionMode: "auto",
	}

	// NOTE: StartRun/CompleteRun/FailRun/CancelRun call dedicated DB methods that
	// unconditionally set the phase — they do NOT go through the state machine.
	// Only AdvancePhase enforces CanRunTransition. These boundary tests document
	// that boundary — including cases where the stub allows re-entry that the
	// real DB might prevent via CHECK constraints.

	t.Run("StartRun after completed overwrites phase (no state machine guard)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)

		// StartRun does not check state machine — it unconditionally sets executing.
		o.StartRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "executing" {
			t.Errorf("expected phase=executing (StartRun overwrites), got %s", stored.Phase)
		}
	})

	t.Run("double CompleteRun is idempotent", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)
		o.CompleteRun(ctx, runID)

		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "completed" {
			t.Errorf("expected phase=completed after double complete, got %s", stored.Phase)
		}
	})

	t.Run("double FailRun is idempotent", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.FailRun(ctx, runID, "first error")
		o.FailRun(ctx, runID, "second error")

		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "failed" {
			t.Errorf("expected phase=failed after double fail, got %s", stored.Phase)
		}
	})

	t.Run("CancelRun after completed overwrites phase (no state machine guard)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)

		// CancelRun does not check state machine — it unconditionally sets cancelled.
		o.CancelRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "cancelled" {
			t.Errorf("expected phase=cancelled (CancelRun overwrites), got %s", stored.Phase)
		}
	})

	t.Run("AdvancePhase from cancelled to any phase is rejected", func(t *testing.T) {
		targets := []string{"pending", "planning", "executing", "reviewing", "completed", "failed"}
		for _, target := range targets {
			t.Run("cancelled→"+target, func(t *testing.T) {
				o, _, _ := newTestOrchestrator()
				run, _ := o.CreateRun(ctx, req)
				runID := util.UUIDToString(run.ID)
				o.CancelRun(ctx, runID)

				_, err := o.AdvancePhase(ctx, runID, target)
				if err == nil {
					t.Fatalf("expected error for cancelled→%s", target)
				}
			})
		}
	})

	t.Run("AdvancePhase from completed to any phase is rejected", func(t *testing.T) {
		targets := []string{"pending", "planning", "executing", "reviewing", "failed", "cancelled"}
		for _, target := range targets {
			t.Run("completed→"+target, func(t *testing.T) {
				o, _, _ := newTestOrchestrator()
				run, _ := o.CreateRun(ctx, req)
				runID := util.UUIDToString(run.ID)
				o.StartRun(ctx, runID)
				o.CompleteRun(ctx, runID)

				_, err := o.AdvancePhase(ctx, runID, target)
				if err == nil {
					t.Fatalf("expected error for completed→%s", target)
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// CreateRun Boundary Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CreateRun_Boundary(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateRun sets phase=pending and status=pending", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID:    "00000000-0000-0000-0000-000000000001",
			IssueID:        "00000000-0000-0000-0000-000000000002",
			AgentID:        "00000000-0000-0000-0000-000000000003",
			SystemPrompt:   "test prompt",
			ModelName:      "claude-sonnet",
			PermissionMode: "auto",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if run.Phase != PhasePending {
			t.Errorf("expected phase=pending, got %s", run.Phase)
		}
		if run.Status != "pending" {
			t.Errorf("expected status=pending, got %s", run.Status)
		}
	})

	t.Run("CreateRun with empty optional fields produces valid UUIDs", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			// TaskID, ParentRunID, TeamID all empty
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !run.ID.Valid {
			t.Error("expected valid run ID")
		}
		if !run.WorkspaceID.Valid {
			t.Error("expected valid workspace ID")
		}
		// TaskID, ParentRunID, TeamID should be invalid (empty UUIDs)
		if run.TaskID.Valid {
			t.Error("expected invalid TaskID for empty task")
		}
		if run.ParentRunID.Valid {
			t.Error("expected invalid ParentRunID for empty parent")
		}
	})

	t.Run("CreateRun broadcasts run:created event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		_, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ec.waitEvents()
		created := ec.byType("run:created")
		if len(created) != 1 {
			t.Fatalf("expected 1 run:created event, got %d", len(created))
		}
	})

	t.Run("CreateRun with parent_run_id links parent", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		parent, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		parentID := util.UUIDToString(parent.ID)

		child, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			ParentRunID: parentID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !child.ParentRunID.Valid {
			t.Fatal("expected valid ParentRunID")
		}
		if got := util.UUIDToString(child.ParentRunID); got != parentID {
			t.Errorf("expected parent_run_id=%s, got %s", parentID, got)
		}
	})

	t.Run("CreateRun multiple times produces unique IDs", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		ids := make(map[string]bool)
		for i := 0; i < 10; i++ {
			run, err := o.CreateRun(ctx, CreateRunRequest{
				WorkspaceID: "00000000-0000-0000-0000-000000000001",
				IssueID:     "00000000-0000-0000-0000-000000000002",
				AgentID:     "00000000-0000-0000-0000-000000000003",
			})
			if err != nil {
				t.Fatalf("unexpected error on run %d: %v", i, err)
			}
			id := util.UUIDToString(run.ID)
			if ids[id] {
				t.Fatalf("duplicate run ID: %s", id)
			}
			ids[id] = true
		}
	})
}

// ---------------------------------------------------------------------------
// CompleteRun / FailRun Terminal State Boundary Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CompleteRun_FailRun_TerminalBoundary(t *testing.T) {
	ctx := context.Background()

	req := CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		SystemPrompt:   "test",
		ModelName:      "claude-sonnet",
		PermissionMode: "auto",
	}

	t.Run("CompleteRun sets phase=completed unconditionally (no state machine guard)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		// Complete directly from pending (skipping executing — AdvancePhase would reject this)
		o.CompleteRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "completed" {
			t.Errorf("expected phase=completed, got %s", stored.Phase)
		}
		if stored.Status != "completed" {
			t.Errorf("expected status=completed, got %s", stored.Status)
		}
	})

	t.Run("CompleteRun sets completed_at timestamp", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CompleteRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if !stored.CompletedAt.Valid {
			t.Error("expected completed_at to be set")
		}
	})

	t.Run("CompleteRun broadcasts run:completed event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CompleteRun(ctx, runID)
		ec.waitEvents()
		evts := ec.byType("run:completed")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:completed event, got %d", len(evts))
		}
	})

	t.Run("FailRun sets phase=failed unconditionally (no state machine guard)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		// Fail directly from pending (skipping executing — AdvancePhase would reject this)
		o.FailRun(ctx, runID, "something broke")
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "failed" {
			t.Errorf("expected phase=failed, got %s", stored.Phase)
		}
		if stored.Status != "failed" {
			t.Errorf("expected status=failed, got %s", stored.Status)
		}
	})

	t.Run("FailRun with no error message broadcasts empty error", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.FailRun(ctx, runID) // no error message varargs
		ec.waitEvents()
		evts := ec.byType("run:failed")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:failed event, got %d", len(evts))
		}
	})

	t.Run("FailRun with error message includes error in broadcast", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.FailRun(ctx, runID, "oom kill")
		ec.waitEvents()
		evts := ec.byType("run:failed")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:failed event, got %d", len(evts))
		}
	})

	t.Run("Double CompleteRun overwrites phase (stub is idempotent)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CompleteRun(ctx, runID)
		o.CompleteRun(ctx, runID) // double complete — stub doesn't care
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "completed" {
			t.Errorf("expected phase=completed after double complete, got %s", stored.Phase)
		}
	})

	t.Run("CompleteRun then FailRun overwrites phase (stub is unconditional)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CompleteRun(ctx, runID)
		o.FailRun(ctx, runID, "post-completion failure")
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		// Stub unconditionally sets — DB would enforce constraints
		if stored.Phase != "failed" {
			t.Errorf("expected phase=failed (stub overwrites), got %s", stored.Phase)
		}
	})
}

// ---------------------------------------------------------------------------
// RetryRun Boundary Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_RetryRun_Boundary(t *testing.T) {
	ctx := context.Background()

	req := CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		TaskID:         "00000000-0000-0000-0000-000000000004",
		TeamID:         "00000000-0000-0000-0000-000000000005",
		SystemPrompt:   "test prompt",
		ModelName:      "claude-sonnet",
		PermissionMode: "auto",
	}

	t.Run("RetryRun from failed creates new pending run", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.FailRun(ctx, runID, "timeout")

		retry, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		retryID := util.UUIDToString(retry.ID)
		if retryID == runID {
			t.Error("expected retry run to have a different ID")
		}
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(retryID))
		if stored.Phase != PhasePending {
			t.Errorf("expected retry phase=pending, got %s", stored.Phase)
		}
	})

	t.Run("RetryRun links parent_run_id to original", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		retry, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !retry.ParentRunID.Valid {
			t.Fatal("expected retry to have parent_run_id set")
		}
		if got := util.UUIDToString(retry.ParentRunID); got != runID {
			t.Errorf("expected parent_run_id=%s, got %s", runID, got)
		}
	})

	t.Run("RetryRun preserves workspace, issue, agent, system_prompt, model", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		retry, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if util.UUIDToString(retry.WorkspaceID) != req.WorkspaceID {
			t.Errorf("workspace mismatch: expected %s, got %s", req.WorkspaceID, util.UUIDToString(retry.WorkspaceID))
		}
		if util.UUIDToString(retry.IssueID) != req.IssueID {
			t.Errorf("issue mismatch: expected %s, got %s", req.IssueID, util.UUIDToString(retry.IssueID))
		}
		if util.UUIDToString(retry.AgentID) != req.AgentID {
			t.Errorf("agent mismatch: expected %s, got %s", req.AgentID, util.UUIDToString(retry.AgentID))
		}
		if retry.SystemPrompt != req.SystemPrompt {
			t.Errorf("system_prompt mismatch: expected %s, got %s", req.SystemPrompt, retry.SystemPrompt)
		}
		if retry.ModelName != req.ModelName {
			t.Errorf("model mismatch: expected %s, got %s", req.ModelName, retry.ModelName)
		}
	})

	t.Run("RetryRun preserves task_id and team_id", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		retry, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !retry.TaskID.Valid {
			t.Error("expected retry to preserve task_id")
		}
		if !retry.TeamID.Valid {
			t.Error("expected retry to preserve team_id")
		}
	})

	t.Run("RetryRun from non-existent run returns error", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		_, err := o.RetryRun(ctx, "00000000-0000-0000-0000-999999999999")
		if err == nil {
			t.Fatal("expected error for non-existent run")
		}
	})

	t.Run("RetryRun from completed run creates new run (no state guard)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.CompleteRun(ctx, runID)

		// RetryRun doesn't check state — it just reads the run and creates a new one
		retry, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		retryID := util.UUIDToString(retry.ID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(retryID))
		if stored.Phase != PhasePending {
			t.Errorf("expected retry phase=pending, got %s", stored.Phase)
		}
	})

	t.Run("RetryRun twice creates two separate runs with same parent", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		retry1, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("retry1 error: %v", err)
		}
		retry2, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("retry2 error: %v", err)
		}
		if util.UUIDToString(retry1.ID) == util.UUIDToString(retry2.ID) {
			t.Error("expected two different retry run IDs")
		}
		// Both should point to original as parent
		if util.UUIDToString(retry1.ParentRunID) != runID {
			t.Errorf("retry1 parent should be original, got %s", util.UUIDToString(retry1.ParentRunID))
		}
		if util.UUIDToString(retry2.ParentRunID) != runID {
			t.Errorf("retry2 parent should be original, got %s", util.UUIDToString(retry2.ParentRunID))
		}
	})
}

// ---------------------------------------------------------------------------
// CancelRun Edge Case Boundary Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CancelRun_EdgeCases(t *testing.T) {
	ctx := context.Background()

	req := CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		SystemPrompt:   "test",
		ModelName:      "claude-sonnet",
		PermissionMode: "auto",
	}

	t.Run("CancelRun from pending sets phase=cancelled", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CancelRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "cancelled" {
			t.Errorf("expected phase=cancelled, got %s", stored.Phase)
		}
		if stored.Status != "cancelled" {
			t.Errorf("expected status=cancelled, got %s", stored.Status)
		}
	})

	t.Run("CancelRun from executing sets phase=cancelled (stub is unconditional)", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)

		o.CancelRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "cancelled" {
			t.Errorf("expected phase=cancelled, got %s", stored.Phase)
		}
	})

	t.Run("CancelRun broadcasts run:cancelled event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CancelRun(ctx, runID)
		ec.waitEvents()
		evts := ec.byType("run:cancelled")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:cancelled event, got %d", len(evts))
		}
	})

	t.Run("CancelRun sets completed_at timestamp", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CancelRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if !stored.CompletedAt.Valid {
			t.Error("expected completed_at to be set after cancel")
		}
	})

	t.Run("Double CancelRun is idempotent with stub", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)

		o.CancelRun(ctx, runID)
		o.CancelRun(ctx, runID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(runID))
		if stored.Phase != "cancelled" {
			t.Errorf("expected phase=cancelled after double cancel, got %s", stored.Phase)
		}
	})

	t.Run("CancelRun then AdvancePhase to any phase is rejected", func(t *testing.T) {
		targets := []string{PhasePlanning, PhaseExecuting, PhaseReviewing, PhaseCompleted, PhaseFailed}
		for _, target := range targets {
			t.Run("cancelled→"+target, func(t *testing.T) {
				o, _, _ := newTestOrchestrator()
				run, _ := o.CreateRun(ctx, req)
				runID := util.UUIDToString(run.ID)
				o.CancelRun(ctx, runID)

				_, err := o.AdvancePhase(ctx, runID, target)
				if err == nil {
					t.Fatalf("expected error for AdvancePhase cancelled→%s", target)
				}
			})
		}
	})

	t.Run("FailRun then RetryRun then CancelRun the retry", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, req)
		runID := util.UUIDToString(run.ID)
		o.StartRun(ctx, runID)
		o.FailRun(ctx, runID, "oom")

		retry, err := o.RetryRun(ctx, runID)
		if err != nil {
			t.Fatalf("unexpected retry error: %v", err)
		}
		retryID := util.UUIDToString(retry.ID)

		o.CancelRun(ctx, retryID)
		stored, _ := stubs.GetRun(ctx, util.ParseUUID(retryID))
		if stored.Phase != "cancelled" {
			t.Errorf("expected retry phase=cancelled, got %s", stored.Phase)
		}
	})
}

// ---------------------------------------------------------------------------
// Error-Injecting Stub — overrides specific methods to return errors on demand.
// ---------------------------------------------------------------------------

// errQueries embeds *stubQueries and overrides one method at a time to return errors.
// Set failOn to the method name that should fail; all others delegate to the embedded stub.
type errQueries struct {
	*stubQueries
	failOn string
}

var errInject = fmt.Errorf("injected database error")

func (e *errQueries) CreateRun(ctx context.Context, p db.CreateRunParams) (db.Run, error) {
	if e.failOn == "CreateRun" {
		return db.Run{}, errInject
	}
	return e.stubQueries.CreateRun(ctx, p)
}

func (e *errQueries) CancelRun(ctx context.Context, id pgtype.UUID) (db.Run, error) {
	if e.failOn == "CancelRun" {
		return db.Run{}, errInject
	}
	return e.stubQueries.CancelRun(ctx, id)
}

func (e *errQueries) GetRun(ctx context.Context, id pgtype.UUID) (db.Run, error) {
	if e.failOn == "GetRun" {
		return db.Run{}, errInject
	}
	return e.stubQueries.GetRun(ctx, id)
}

func (e *errQueries) GetRunByTask(ctx context.Context, taskID pgtype.UUID) (db.Run, error) {
	if e.failOn == "GetRunByTask" {
		return db.Run{}, errInject
	}
	return e.stubQueries.GetRunByTask(ctx, taskID)
}

func (e *errQueries) StartRun(ctx context.Context, id pgtype.UUID) (db.Run, error) {
	if e.failOn == "StartRun" {
		return db.Run{}, errInject
	}
	return e.stubQueries.StartRun(ctx, id)
}

func (e *errQueries) UpdateRunPhase(ctx context.Context, p db.UpdateRunPhaseParams) (db.Run, error) {
	if e.failOn == "UpdateRunPhase" {
		return db.Run{}, errInject
	}
	return e.stubQueries.UpdateRunPhase(ctx, p)
}

func (e *errQueries) CompleteRun(ctx context.Context, id pgtype.UUID) (db.Run, error) {
	if e.failOn == "CompleteRun" {
		return db.Run{}, errInject
	}
	return e.stubQueries.CompleteRun(ctx, id)
}

func (e *errQueries) FailRun(ctx context.Context, id pgtype.UUID) (db.Run, error) {
	if e.failOn == "FailRun" {
		return db.Run{}, errInject
	}
	return e.stubQueries.FailRun(ctx, id)
}

func (e *errQueries) UpdateRunTokens(ctx context.Context, p db.UpdateRunTokensParams) (db.Run, error) {
	if e.failOn == "UpdateRunTokens" {
		return db.Run{}, errInject
	}
	return e.stubQueries.UpdateRunTokens(ctx, p)
}

func (e *errQueries) GetNextStepSeq(ctx context.Context, id pgtype.UUID) (int32, error) {
	if e.failOn == "GetNextStepSeq" {
		return 0, errInject
	}
	return e.stubQueries.GetNextStepSeq(ctx, id)
}

func (e *errQueries) CreateRunStep(ctx context.Context, p db.CreateRunStepParams) (db.RunStep, error) {
	if e.failOn == "CreateRunStep" {
		return db.RunStep{}, errInject
	}
	return e.stubQueries.CreateRunStep(ctx, p)
}

func (e *errQueries) CompleteRunStep(ctx context.Context, p db.CompleteRunStepParams) (db.RunStep, error) {
	if e.failOn == "CompleteRunStep" {
		return db.RunStep{}, errInject
	}
	return e.stubQueries.CompleteRunStep(ctx, p)
}

func (e *errQueries) GetNextTodoSeq(ctx context.Context, id pgtype.UUID) (int32, error) {
	if e.failOn == "GetNextTodoSeq" {
		return 0, errInject
	}
	return e.stubQueries.GetNextTodoSeq(ctx, id)
}

func (e *errQueries) CreateRunTodo(ctx context.Context, p db.CreateRunTodoParams) (db.RunTodo, error) {
	if e.failOn == "CreateRunTodo" {
		return db.RunTodo{}, errInject
	}
	return e.stubQueries.CreateRunTodo(ctx, p)
}

func (e *errQueries) UpdateRunTodo(ctx context.Context, p db.UpdateRunTodoParams) (db.RunTodo, error) {
	if e.failOn == "UpdateRunTodo" {
		return db.RunTodo{}, errInject
	}
	return e.stubQueries.UpdateRunTodo(ctx, p)
}

func (e *errQueries) CreateRunHandoff(ctx context.Context, p db.CreateRunHandoffParams) (db.RunHandoff, error) {
	if e.failOn == "CreateRunHandoff" {
		return db.RunHandoff{}, errInject
	}
	return e.stubQueries.CreateRunHandoff(ctx, p)
}

func (e *errQueries) CreateRunContinuation(ctx context.Context, p db.CreateRunContinuationParams) (db.RunContinuation, error) {
	if e.failOn == "CreateRunContinuation" {
		return db.RunContinuation{}, errInject
	}
	return e.stubQueries.CreateRunContinuation(ctx, p)
}

func (e *errQueries) CreateRunArtifact(ctx context.Context, p db.CreateRunArtifactParams) (db.RunArtifact, error) {
	if e.failOn == "CreateRunArtifact" {
		return db.RunArtifact{}, errInject
	}
	return e.stubQueries.CreateRunArtifact(ctx, p)
}

func (e *errQueries) CreateRunEvent(ctx context.Context, p db.CreateRunEventParams) (db.RunEvent, error) {
	if e.failOn == "CreateRunEvent" {
		return db.RunEvent{}, errInject
	}
	return e.stubQueries.CreateRunEvent(ctx, p)
}

func newTestOrchestratorWithErrQueries(failOn string) (*RunOrchestrator, *errQueries, *eventCollector) {
	stubs := newStubQueries()
	eq := &errQueries{stubQueries: stubs, failOn: failOn}
	ec := &eventCollector{}
	bus := events.New()
	bus.SubscribeAll(ec.collect)
	o := NewRunOrchestrator(eq, nil, nil, bus)
	return o, eq, ec
}

// ---------------------------------------------------------------------------
// CreateRun Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CreateRun_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateRun propagates DB error wrapped as 'create run'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRun")
		_, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		if err == nil {
			t.Fatal("expected error from CreateRun")
		}
		want := "create run: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("CreateRun DB error does not broadcast event", func(t *testing.T) {
		o, _, ec := newTestOrchestratorWithErrQueries("CreateRun")
		o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		ec.waitEvents()
		evts := ec.byType("run:created")
		if len(evts) != 0 {
			t.Errorf("expected no run:created event on DB error, got %d", len(evts))
		}
	})

	t.Run("CreateRun with all optional fields set still propagates DB error", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRun")
		_, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID:    "00000000-0000-0000-0000-000000000001",
			IssueID:        "00000000-0000-0000-0000-000000000002",
			AgentID:        "00000000-0000-0000-0000-000000000003",
			TaskID:         "00000000-0000-0000-0000-000000000004",
			ParentRunID:    "00000000-0000-0000-0000-000000000005",
			TeamID:         "00000000-0000-0000-0000-000000000006",
			SystemPrompt:   "test",
			ModelName:      "claude-sonnet",
			PermissionMode: "auto",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "create run: injected database error" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// RetryRun Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_RetryRun_DBErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("RetryRun wraps GetRun failure as 'get original run'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetRun")
		_, err := o.RetryRun(ctx, "00000000-0000-0000-0000-000000000001")
		if err == nil {
			t.Fatal("expected error from RetryRun with failing GetRun")
		}
		want := "get original run: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("RetryRun wraps inner CreateRun failure as 'create retry run'", func(t *testing.T) {
		// Use a normal stub first to create the original run, then swap to errQueries.
		stubs := newStubQueries()
		ec := &eventCollector{}
		bus := events.New()
		bus.SubscribeAll(ec.collect)
		o := NewRunOrchestrator(stubs, nil, nil, bus)

		// Create original run successfully.
		run, err := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		if err != nil {
			t.Fatalf("unexpected error creating original: %v", err)
		}
		runID := util.UUIDToString(run.ID)

		// Now replace Queries with errQueries that fails on CreateRun but keeps GetRun working.
		// errQueries wraps the same stubs so GetRun can still find the original run.
		eq := &errQueries{stubQueries: stubs, failOn: "CreateRun"}
		o.Queries = eq

		_, err = o.RetryRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error from RetryRun with failing CreateRun")
		}
		want := "create retry run: create run: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("RetryRun GetRun failure returns zero-value Run", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetRun")
		run, err := o.RetryRun(ctx, "00000000-0000-0000-0000-000000000001")
		if err == nil {
			t.Fatal("expected error")
		}
		if run.ID.Valid {
			t.Error("expected zero-value Run on error")
		}
	})

	t.Run("RetryRun CreateRun failure returns zero-value Run", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "CreateRun"}
		retry, err := o.RetryRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error")
		}
		if retry.ID.Valid {
			t.Error("expected zero-value Run on CreateRun failure")
		}
	})

	t.Run("RetryRun GetRun failure does not create a new run", func(t *testing.T) {
		o, eq, _ := newTestOrchestratorWithErrQueries("GetRun")
		o.RetryRun(ctx, "00000000-0000-0000-0000-000000000001")

		// Verify no run was created (the stub's map should be empty)
		eq.failOn = "" // disable error to access stub
		runs, _ := eq.stubQueries.ListRunsByWorkspace(ctx, db.ListRunsByWorkspaceParams{
			WorkspaceID: util.ParseUUID("00000000-0000-0000-0000-000000000001"),
		})
		if len(runs) != 0 {
			t.Errorf("expected no runs created after GetRun failure, got %d", len(runs))
		}
	})
}

// ---------------------------------------------------------------------------
// CancelRun Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CancelRun_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("CancelRun propagates DB error wrapped as 'cancel run'", func(t *testing.T) {
		// Create run with normal stubs, then swap to error queries.
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "CancelRun"}
		_, err := o.CancelRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error from CancelRun")
		}
		want := "cancel run: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("CancelRun error returns zero-value Run", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "CancelRun"}
		result, err := o.CancelRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error")
		}
		if result.ID.Valid {
			t.Error("expected zero-value Run on error")
		}
	})

	t.Run("CancelRun error does not broadcast run:cancelled event", func(t *testing.T) {
		stubs := newStubQueries()
		ec := &eventCollector{}
		bus := events.New()
		bus.SubscribeAll(ec.collect)
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)
		// Clear events from CreateRun
		ec.waitEvents()
		ec.events = nil

		o.Queries = &errQueries{stubQueries: stubs, failOn: "CancelRun"}
		o.CancelRun(ctx, runID)
		ec.waitEvents()
		evts := ec.byType("run:cancelled")
		if len(evts) != 0 {
			t.Errorf("expected no run:cancelled event on DB error, got %d", len(evts))
		}
	})

	t.Run("CancelRun on non-existent run returns error from stub", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		_, err := o.CancelRun(ctx, "00000000-0000-0000-0000-999999999999")
		if err == nil {
			t.Fatal("expected error for non-existent run")
		}
	})
}
func mustCreateRun2(t *testing.T, o *RunOrchestrator, workspaceID string) string {
	t.Helper()
	run, err := o.CreateRun(context.Background(), CreateRunRequest{
		WorkspaceID: workspaceID,
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	if err != nil {
		t.Fatal(err)
	}
	return util.UUIDToString(run.ID)
}

func TestRunOrchestrator_CreateHandoffSpec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("CreateHandoff persists handoff to stub store", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		handoff, err := o.CreateHandoff(ctx, runID, "team", "Target team needs this context", "target-run-1", "target-team-1", "target-agent-1", []byte(`{"key":"val"}`))
		if err != nil {
			t.Fatalf("CreateHandoff failed: %v", err)
		}
		if handoff.HandoffType != "team" {
			t.Errorf("expected HandoffType 'team', got %q", handoff.HandoffType)
		}
		stubs.mu.Lock()
		if len(stubs.handoffs) == 0 {
			t.Fatal("handoff not persisted in stub store")
		}
		stubs.mu.Unlock()
	})

	t.Run("CreateHandoff sets Reason correctly", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		_, err := o.CreateHandoff(ctx, runID, "agent", "Needs review", "trun-1", "tteam-1", "tagent-1", []byte(`{}`))
		if err != nil {
			t.Fatalf("CreateHandoff failed: %v", err)
		}
		stubs.mu.Lock()
		defer stubs.mu.Unlock()
		for _, h := range stubs.handoffs {
			if h.Reason != "Needs review" {
				t.Errorf("expected reason 'Needs review', got %q", h.Reason)
			}
			return
		}
		t.Fatal("no handoff found in store")
	})

	t.Run("CreateHandoff broadcasts handoff event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		ec.waitEvents()
		_, err := o.CreateHandoff(ctx, runID, "team", "reason", "", "team-1", "", []byte(`{}`))
		if err != nil {
			t.Fatalf("CreateHandoff failed: %v", err)
		}
		ec.waitEvents()
		handoffEvts := ec.byType("run:handoff_created")
		if len(handoffEvts) != 1 {
			t.Errorf("expected 1 handoff event, got %d", len(handoffEvts))
		}
	})
}

func TestRunOrchestrator_CreateContinuationSpec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("CreateContinuation persists continuation to stub store", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		cont, err := o.CreateContinuation(ctx, runID, "Summary of work", json.RawMessage(`["todo1","todo2"]`), json.RawMessage(`["decision1"]`), json.RawMessage(`["file.go"]`), json.RawMessage(`["blocker1"]`), json.RawMessage(`["question1"]`), 500)
		if err != nil {
			t.Fatalf("CreateContinuation failed: %v", err)
		}
		if cont.CompactSummary != "Summary of work" {
			t.Errorf("expected summary, got %q", cont.CompactSummary)
		}
		stubs.mu.Lock()
		if len(stubs.continuations) == 0 {
			t.Fatal("continuation not persisted in stub store")
		}
		stubs.mu.Unlock()
	})

	t.Run("CreateContinuation stores token budget", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		_, err := o.CreateContinuation(ctx, runID, "summary", json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), 1024)
		if err != nil {
			t.Fatalf("CreateContinuation failed: %v", err)
		}
		stubs.mu.Lock()
		defer stubs.mu.Unlock()
		for _, c := range stubs.continuations {
			if c.TokenBudgetUsed != 1024 {
				t.Errorf("expected TokenBudgetUsed 1024, got %d", c.TokenBudgetUsed)
			}
			return
		}
		t.Fatal("no continuation found in store")
	})

	t.Run("CreateContinuation stores CompactSummary", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		_, err := o.CreateContinuation(ctx, runID, "my summary", json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), 0)
		if err != nil {
			t.Fatalf("CreateContinuation failed: %v", err)
		}
		stubs.mu.Lock()
		defer stubs.mu.Unlock()
		for _, c := range stubs.continuations {
			if c.CompactSummary != "my summary" {
				t.Errorf("expected 'my summary', got %q", c.CompactSummary)
			}
			return
		}
		t.Fatal("no continuation found in store")
	})
}

func TestRunOrchestrator_CreateArtifactSpec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("CreateArtifact persists artifact to stub store", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		art, err := o.CreateArtifact(ctx, runID, "", "file", "main.go", "package main", "text/plain")
		if err != nil {
			t.Fatalf("CreateArtifact failed: %v", err)
		}
		if art.Name != "main.go" {
			t.Errorf("expected name 'main.go', got %q", art.Name)
		}
		stubs.mu.Lock()
		if len(stubs.artifacts) == 0 {
			t.Fatal("artifact not persisted in stub store")
		}
		stubs.mu.Unlock()
	})

	t.Run("CreateArtifact sets ArtifactType correctly", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		art, err := o.CreateArtifact(ctx, runID, "", "diff", "changes.patch", "--- a/file", "text/x-patch")
		if err != nil {
			t.Fatalf("CreateArtifact failed: %v", err)
		}
		if art.ArtifactType != "diff" {
			t.Errorf("expected ArtifactType 'diff', got %q", art.ArtifactType)
		}
	})

	t.Run("CreateArtifact with stepID", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		stepID := "00000000-0000-0000-0000-000000000099"
		art, err := o.CreateArtifact(ctx, runID, stepID, "log", "output.log", "log content", "text/plain")
		if err != nil {
			t.Fatalf("CreateArtifact failed: %v", err)
		}
		if !art.StepID.Valid {
			t.Error("expected StepID to be valid")
		}
	})

	t.Run("CreateArtifact broadcasts event", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		runID := mustCreateRun2(t, o, "00000000-0000-0000-0000-000000000001")
		ec.waitEvents()
		_, err := o.CreateArtifact(ctx, runID, "", "file", "f.go", "c", "text/plain")
		if err != nil {
			t.Fatalf("CreateArtifact failed: %v", err)
		}
		ec.waitEvents()
		artEvts := ec.byType("run:artifact_created")
		if len(artEvts) != 1 {
			t.Errorf("expected 1 artifact event, got %d", len(artEvts))
		}
	})
}

func TestRunOrchestrator_ListGetSpec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ListRuns returns runs for the workspace", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		wsID := "00000000-0000-0000-0000-000000000001"
		_, _ = o.CreateRun(ctx, CreateRunRequest{WorkspaceID: wsID, IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		_, _ = o.CreateRun(ctx, CreateRunRequest{WorkspaceID: wsID, IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		_, _ = o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000099", IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		runs, err := o.ListRuns(ctx, wsID, 100, 0)
		if err != nil {
			t.Fatalf("ListRuns failed: %v", err)
		}
		if len(runs) != 2 {
			t.Errorf("expected 2 runs, got %d", len(runs))
		}
	})

	t.Run("ListRuns returns empty for workspace with no runs", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		runs, err := o.ListRuns(ctx, "00000000-0000-0000-0000-000000000099", 100, 0)
		if err != nil {
			t.Fatalf("ListRuns failed: %v", err)
		}
		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})

	t.Run("ListRunsByIssue filters by issueID", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		issueID := "11111111-1111-1111-1111-111111111111"
		_, _ = o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: issueID, AgentID: "00000000-0000-0000-0000-000000000003"})
		_, _ = o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: "22222222-2222-2222-2222-222222222222", AgentID: "00000000-0000-0000-0000-000000000003"})
		runs, err := o.ListRunsByIssue(ctx, issueID)
		if err != nil {
			t.Fatalf("ListRunsByIssue failed: %v", err)
		}
		if len(runs) != 1 {
			t.Errorf("expected 1 run for issue, got %d", len(runs))
		}
	})

	t.Run("GetRun returns the run by ID", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		created, _ := o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		runID := util.UUIDToString(created.ID)
		run, err := o.GetRun(ctx, runID)
		if err != nil {
			t.Fatalf("GetRun failed: %v", err)
		}
		if util.UUIDToString(run.ID) != runID {
			t.Errorf("expected runID %s, got %s", runID, util.UUIDToString(run.ID))
		}
	})

	t.Run("GetRun on non-existent run returns error", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		_, err := o.GetRun(ctx, "00000000-0000-0000-0000-999999999999")
		if err == nil {
			t.Fatal("expected error for non-existent run")
		}
	})

	t.Run("GetRunSteps returns empty for run with no steps", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		created, _ := o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		steps, err := o.GetRunSteps(ctx, util.UUIDToString(created.ID))
		if err != nil {
			t.Fatalf("GetRunSteps failed: %v", err)
		}
		if len(steps) != 0 {
			t.Errorf("expected 0 steps, got %d", len(steps))
		}
	})

	t.Run("GetRunTodos returns empty for run with no todos", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		created, _ := o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		todos, err := o.GetRunTodos(ctx, util.UUIDToString(created.ID))
		if err != nil {
			t.Fatalf("GetRunTodos failed: %v", err)
		}
		if len(todos) != 0 {
			t.Errorf("expected 0 todos, got %d", len(todos))
		}
	})

	t.Run("GetRunArtifacts returns artifacts created for a run", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		created, _ := o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		runID := util.UUIDToString(created.ID)
		_, _ = o.CreateArtifact(ctx, runID, "", "file", "a.go", "code", "text/plain")
		_, _ = o.CreateArtifact(ctx, runID, "", "file", "b.go", "code2", "text/plain")
		arts, err := o.GetRunArtifacts(ctx, runID)
		if err != nil {
			t.Fatalf("GetRunArtifacts failed: %v", err)
		}
		if len(arts) != 2 {
			t.Errorf("expected 2 artifacts, got %d", len(arts))
		}
	})

	t.Run("GetRunArtifacts returns empty for run with no artifacts", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		created, _ := o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		arts, err := o.GetRunArtifacts(ctx, util.UUIDToString(created.ID))
		if err != nil {
			t.Fatalf("GetRunArtifacts failed: %v", err)
		}
		if len(arts) != 0 {
			t.Errorf("expected 0 artifacts, got %d", len(arts))
		}
	})
}

func TestRunOrchestrator_BroadcastRunEventSpec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Broadcast publishes event to the event bus", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		o.Broadcast("ws-broadcast", "run:started", map[string]any{"run_id": "test"})
		ec.waitEvents()
		evts := ec.byType("run:started")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:started event, got %d", len(evts))
		}
	})

	t.Run("BroadcastRunEvent persists then broadcasts", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		created, _ := o.CreateRun(ctx, CreateRunRequest{WorkspaceID: "00000000-0000-0000-0000-000000000001", IssueID: "00000000-0000-0000-0000-000000000002", AgentID: "00000000-0000-0000-0000-000000000003"})
		ec.waitEvents()
		runID := util.UUIDToString(created.ID)
		wsID := util.UUIDToString(created.WorkspaceID)
		o.BroadcastRunEvent(ctx, runID, wsID, "run:phase_updated", map[string]any{"phase": "reviewing"})
		ec.waitEvents()
		evts := ec.byType("run:phase_updated")
		if len(evts) != 1 {
			t.Fatalf("expected 1 run:phase_updated event, got %d", len(evts))
		}
	})

	t.Run("Broadcast with empty event type still works", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		o.Broadcast("ws-empty", "", nil)
		ec.waitEvents()
		_ = ec.events
	})
}

func TestNullUUIDToString_RunOrch(t *testing.T) {
	t.Parallel()

	t.Run("valid UUID returns string representation", func(t *testing.T) {
		id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
		u := pgtype.UUID{Valid: true, Bytes: id}
		got := nullUUIDToString(u)
		if got != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
			t.Errorf("expected UUID string, got %q", got)
		}
	})

	t.Run("invalid UUID returns empty string", func(t *testing.T) {
		u := pgtype.UUID{Valid: false}
		got := nullUUIDToString(u)
		if got != "" {
			t.Errorf("expected empty string for invalid UUID, got %q", got)
		}
	})
}

func TestParseNullUUID(t *testing.T) {
	t.Parallel()

	t.Run("empty string returns invalid UUID", func(t *testing.T) {
		got := parseNullUUID("")
		if got.Valid {
			t.Error("expected invalid UUID for empty string")
		}
	})

	t.Run("valid UUID string returns valid pgtype.UUID", func(t *testing.T) {
		got := parseNullUUID("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
		if !got.Valid {
			t.Fatal("expected valid UUID")
		}
		if got.String() != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
			t.Errorf("expected UUID string, got %q", got.String())
		}
	})
}

// ---------------------------------------------------------------------------
// GetOrCreateRun Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_GetOrCreateRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty TaskID delegates to CreateRun", func(t *testing.T) {
		o, stubs, _ := newTestOrchestrator()
		req := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		}
		run, err := o.GetOrCreateRun(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !run.ID.Valid {
			t.Fatal("expected valid run ID")
		}
		stubs.mu.Lock()
		_, ok := stubs.runs[util.UUIDToString(run.ID)]
		stubs.mu.Unlock()
		if !ok {
			t.Error("run not found in stub store after GetOrCreateRun")
		}
	})

	t.Run("first call with TaskID creates new run", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		req := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			TaskID:      "00000000-0000-0000-0000-000000000010",
		}
		run, err := o.GetOrCreateRun(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !run.ID.Valid {
			t.Fatal("expected valid run ID")
		}
	})

	t.Run("second call with same TaskID returns existing run", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		req := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			TaskID:      "00000000-0000-0000-0000-000000000010",
		}
		first, err := o.GetOrCreateRun(ctx, req)
		if err != nil {
			t.Fatalf("first call error: %v", err)
		}
		second, err := o.GetOrCreateRun(ctx, req)
		if err != nil {
			t.Fatalf("second call error: %v", err)
		}
		if first.ID != second.ID {
			t.Errorf("expected same run ID on second call, first=%s second=%s",
				util.UUIDToString(first.ID), util.UUIDToString(second.ID))
		}
	})

	t.Run("different TaskIDs create different runs", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		req1 := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			TaskID:      "00000000-0000-0000-0000-000000000010",
		}
		req2 := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			TaskID:      "00000000-0000-0000-0000-000000000020",
		}
		first, err := o.GetOrCreateRun(ctx, req1)
		if err != nil {
			t.Fatalf("first call error: %v", err)
		}
		second, err := o.GetOrCreateRun(ctx, req2)
		if err != nil {
			t.Fatalf("second call error: %v", err)
		}
		if first.ID == second.ID {
			t.Error("expected different run IDs for different TaskIDs")
		}
	})

	t.Run("broadcasts run:created on first call only", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		req := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			TaskID:      "00000000-0000-0000-0000-000000000010",
		}
		o.GetOrCreateRun(ctx, req)
		ec.waitEvents()
		firstCount := len(ec.byType("run:created"))

		o.GetOrCreateRun(ctx, req)
		ec.waitEvents()
		secondCount := len(ec.byType("run:created"))

		if firstCount != 1 {
			t.Errorf("expected 1 run:created after first call, got %d", firstCount)
		}
		if secondCount != 1 {
			t.Errorf("expected still 1 run:created after second call, got %d", secondCount)
		}
	})

	t.Run("DB error on CreateRun propagates", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRun")
		req := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		}
		_, err := o.GetOrCreateRun(ctx, req)
		if err == nil {
			t.Fatal("expected error from GetOrCreateRun with failing CreateRun")
		}
		if err.Error() != "create run: injected database error" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("DB error on CreateRun with TaskID propagates", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRun")
		req := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			TaskID:      "00000000-0000-0000-0000-000000000010",
		}
		_, err := o.GetOrCreateRun(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "create run: injected database error" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("GetRunByTask failure falls back to CreateRun", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetRunByTask")
		req := CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
			TaskID:      "00000000-0000-0000-0000-000000000010",
		}
		run, err := o.GetOrCreateRun(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error (GetRunByTask failure should fall back to CreateRun): %v", err)
		}
		if !run.ID.Valid {
			t.Fatal("expected valid run ID from fallback CreateRun")
		}
	})
}

// ---------------------------------------------------------------------------
// GetOrCreateRun — concurrency test
// ---------------------------------------------------------------------------

func TestRunOrchestrator_GetOrCreateRun_Concurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	o, stubs, _ := newTestOrchestrator()

	const goroutines = 20
	req := CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
		TaskID:      "00000000-0000-0000-0000-000000000010",
	}

	var wg sync.WaitGroup
	results := make([]db.Run, goroutines)
	errs := make([]error, goroutines)

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // release all goroutines simultaneously
			results[idx], errs[idx] = o.GetOrCreateRun(ctx, req)
		}(i)
	}
	close(start) // fire!
	wg.Wait()

	// All calls should succeed.
	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, errs[i])
		}
	}

	// All should return the same run ID.
	firstID := results[0].ID
	for i := 1; i < goroutines; i++ {
		if results[i].ID != firstID {
			t.Errorf("goroutine %d: expected run ID %s, got %s", i,
				util.UUIDToString(firstID), util.UUIDToString(results[i].ID))
		}
	}

	// Verify only one run was actually stored.
	stubs.mu.Lock()
	runCount := 0
	for _, r := range stubs.runs {
		if r.TaskID == util.ParseUUID(req.TaskID) {
			runCount++
		}
	}
	stubs.mu.Unlock()
	if runCount != 1 {
		t.Errorf("expected exactly 1 run for the task, got %d", runCount)
	}
}

// ---------------------------------------------------------------------------
// BroadcastRunEvent error paths
// ---------------------------------------------------------------------------

func TestBroadcastRunEvent_MarshalPayloadFallback(t *testing.T) {
	o, _, ec := newTestOrchestrator()
	created, _ := o.CreateRun(context.Background(), CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	ec.waitEvents()

	runID := util.UUIDToString(created.ID)
	wsID := util.UUIDToString(created.WorkspaceID)

	// A channel value cannot be marshalled → triggers fallback to "{}".
	o.BroadcastRunEvent(context.Background(), runID, wsID, "run:test", map[string]any{
		"bad": make(chan int),
	})
	ec.waitEvents()

	// Should still broadcast even with marshal error.
	evts := ec.byType("run:test")
	if len(evts) != 1 {
		t.Fatalf("expected 1 run:test event despite marshal error, got %d", len(evts))
	}
}

func TestBroadcastRunEvent_CreateRunEventDBError(t *testing.T) {
	o, stubs, ec := newTestOrchestrator()
	stubs.createRunEventErr = fmt.Errorf("db write failed")

	created, _ := o.CreateRun(context.Background(), CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	ec.waitEvents()

	runID := util.UUIDToString(created.ID)
	wsID := util.UUIDToString(created.WorkspaceID)

	// Should NOT panic or return — continues to broadcast despite DB error.
	o.BroadcastRunEvent(context.Background(), runID, wsID, "run:db_fail", map[string]any{"ok": true})
	ec.waitEvents()

	evts := ec.byType("run:db_fail")
	if len(evts) != 1 {
		t.Fatalf("expected 1 event despite DB error, got %d", len(evts))
	}
}

// ---------------------------------------------------------------------------
// UpdateTokens edge cases
// ---------------------------------------------------------------------------

func TestUpdateTokens_ZeroCost(t *testing.T) {
	o, _, _ := newTestOrchestrator()
	run, _ := o.CreateRun(context.Background(), CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	err := o.UpdateTokens(context.Background(), runID, 0, 0, 0.0)
	if err != nil {
		t.Fatalf("UpdateTokens with zero values: %v", err)
	}
}

func TestUpdateTokens_NegativeCost(t *testing.T) {
	o, _, _ := newTestOrchestrator()
	run, _ := o.CreateRun(context.Background(), CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	err := o.UpdateTokens(context.Background(), runID, 100, 50, -0.001)
	if err != nil {
		t.Fatalf("UpdateTokens with negative cost: %v", err)
	}
}
