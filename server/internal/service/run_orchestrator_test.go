package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

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
	runs   map[string]db.Run
	steps  map[string]db.RunStep
	todos  map[string]db.RunTodo
	nextID int
	mu     sync.Mutex
}

func newStubQueries() *stubQueries {
	return &stubQueries{
		runs:  make(map[string]db.Run),
		steps: make(map[string]db.RunStep),
		todos: make(map[string]db.RunTodo),
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
	run := s.runs[key]
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
func (s *stubQueries) CreateRunArtifact(_ context.Context, _ db.CreateRunArtifactParams) (db.RunArtifact, error) {
	return db.RunArtifact{}, nil
}
func (s *stubQueries) CreateRunHandoff(_ context.Context, _ db.CreateRunHandoffParams) (db.RunHandoff, error) {
	return db.RunHandoff{}, nil
}
func (s *stubQueries) CreateRunContinuation(_ context.Context, _ db.CreateRunContinuationParams) (db.RunContinuation, error) {
	return db.RunContinuation{}, nil
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

func (s *stubQueries) ListRunsByWorkspace(_ context.Context, _ db.ListRunsByWorkspaceParams) ([]db.Run, error) {
	return nil, nil
}
func (s *stubQueries) ListRunsByIssue(_ context.Context, _ pgtype.UUID) ([]db.Run, error) {
	return nil, nil
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
func (s *stubQueries) ListRunArtifacts(_ context.Context, _ pgtype.UUID) ([]db.RunArtifact, error) {
	return nil, nil
}

func (s *stubQueries) CreateRunEvent(_ context.Context, _ db.CreateRunEventParams) (db.RunEvent, error) {
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
