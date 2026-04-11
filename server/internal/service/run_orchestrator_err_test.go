package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/util"
)

// ---------------------------------------------------------------------------
// StartRun Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_StartRun_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("StartRun wraps DB error as 'start run'", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "StartRun"}
		_, err := o.StartRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error from StartRun")
		}
		want := "start run: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("StartRun error returns zero-value Run", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "StartRun"}
		result, err := o.StartRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error")
		}
		if result.ID.Valid {
			t.Error("expected zero-value Run on error")
		}
	})

	t.Run("StartRun error does not broadcast run:started", func(t *testing.T) {
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
		ec.waitEvents()
		ec.events = nil

		o.Queries = &errQueries{stubQueries: stubs, failOn: "StartRun"}
		o.StartRun(ctx, runID)
		ec.waitEvents()
		evts := ec.byType("run:started")
		if len(evts) != 0 {
			t.Errorf("expected no run:started event on DB error, got %d", len(evts))
		}
	})
}

// ---------------------------------------------------------------------------
// CompleteRun Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CompleteRun_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("CompleteRun wraps DB error as 'complete run'", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "CompleteRun"}
		_, err := o.CompleteRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error")
		}
		want := "complete run: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("CompleteRun error returns zero-value Run", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "CompleteRun"}
		result, err := o.CompleteRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error")
		}
		if result.ID.Valid {
			t.Error("expected zero-value Run on error")
		}
	})
}

// ---------------------------------------------------------------------------
// FailRun Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_FailRun_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("FailRun wraps DB error as 'fail run'", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "FailRun"}
		_, err := o.FailRun(ctx, runID, "boom")
		if err == nil {
			t.Fatal("expected error")
		}
		want := "fail run: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("FailRun error returns zero-value Run", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "FailRun"}
		result, err := o.FailRun(ctx, runID)
		if err == nil {
			t.Fatal("expected error")
		}
		if result.ID.Valid {
			t.Error("expected zero-value Run on error")
		}
	})
}

// ---------------------------------------------------------------------------
// AdvancePhase Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_AdvancePhase_DBErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("GetRun error wraps as 'get run for phase'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetRun")
		_, err := o.AdvancePhase(ctx, "00000000-0000-0000-0000-000000000001", "executing")
		if err == nil {
			t.Fatal("expected error")
		}
		want := "get run for phase: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("UpdateRunPhase error wraps as 'advance phase'", func(t *testing.T) {
		stubs := newStubQueries()
		bus := events.New()
		o := NewRunOrchestrator(stubs, nil, nil, bus)
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)

		o.Queries = &errQueries{stubQueries: stubs, failOn: "UpdateRunPhase"}
		_, err := o.AdvancePhase(ctx, runID, "executing")
		if err == nil {
			t.Fatal("expected error")
		}
		want := "advance phase: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("invalid transition returns error without DB call", func(t *testing.T) {
		o, _, _ := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)
		// pending -> reviewing is not allowed
		_, err := o.AdvancePhase(ctx, runID, "reviewing")
		if err == nil {
			t.Fatal("expected error for invalid transition")
		}
		if !strings.Contains(err.Error(), "cannot transition") {
			t.Errorf("expected 'cannot transition' error, got %q", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// RecordStep Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_RecordStep_DBErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("GetNextStepSeq error wraps as 'get next step seq'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetNextStepSeq")
		_, err := o.RecordStep(ctx, "00000000-0000-0000-0000-000000000001", "tool_use", "bash", "c1", []byte(`{}`), "", false)
		if err == nil {
			t.Fatal("expected error")
		}
		want := "get next step seq: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("CreateRunStep error wraps as 'create run step'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRunStep")
		_, err := o.RecordStep(ctx, "00000000-0000-0000-0000-000000000001", "tool_use", "bash", "c1", []byte(`{}`), "", false)
		if err == nil {
			t.Fatal("expected error")
		}
		want := "create run step: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("RecordStep GetNextStepSeq error returns zero-value RunStep", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetNextStepSeq")
		step, err := o.RecordStep(ctx, "00000000-0000-0000-0000-000000000001", "thinking", "", "", nil, "thoughts", false)
		if err == nil {
			t.Fatal("expected error")
		}
		if step.ID.Valid {
			t.Error("expected zero-value RunStep on error")
		}
	})
}

// ---------------------------------------------------------------------------
// CompleteStep Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CompleteStep_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("CompleteRunStep error wraps as 'complete step'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CompleteRunStep")
		_, err := o.CompleteStep(ctx, "00000000-0000-0000-0000-000000000001", "output", false)
		if err == nil {
			t.Fatal("expected error")
		}
		want := "complete step: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("CompleteStep error returns zero-value RunStep", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CompleteRunStep")
		step, err := o.CompleteStep(ctx, "00000000-0000-0000-0000-000000000001", "output", true)
		if err == nil {
			t.Fatal("expected error")
		}
		if step.ID.Valid {
			t.Error("expected zero-value RunStep on error")
		}
	})
}

// ---------------------------------------------------------------------------
// CreateTodo Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CreateTodo_DBErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("GetNextTodoSeq error wraps as 'get next todo seq'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetNextTodoSeq")
		_, err := o.CreateTodo(ctx, "00000000-0000-0000-0000-000000000001", "Task", "desc")
		if err == nil {
			t.Fatal("expected error")
		}
		want := "get next todo seq: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("CreateRunTodo error wraps as 'create run todo'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRunTodo")
		_, err := o.CreateTodo(ctx, "00000000-0000-0000-0000-000000000001", "Task", "desc")
		if err == nil {
			t.Fatal("expected error")
		}
		want := "create run todo: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("CreateTodo error returns zero-value RunTodo", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("GetNextTodoSeq")
		todo, err := o.CreateTodo(ctx, "00000000-0000-0000-0000-000000000001", "Task", "")
		if err == nil {
			t.Fatal("expected error")
		}
		if todo.ID.Valid {
			t.Error("expected zero-value RunTodo on error")
		}
	})
}

// ---------------------------------------------------------------------------
// UpdateTodo Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_UpdateTodo_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("UpdateRunTodo error wraps as 'update run todo'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("UpdateRunTodo")
		_, err := o.UpdateTodo(ctx, "00000000-0000-0000-0000-000000000001", "completed", "")
		if err == nil {
			t.Fatal("expected error")
		}
		want := "update run todo: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("UpdateTodo error returns zero-value RunTodo", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("UpdateRunTodo")
		todo, err := o.UpdateTodo(ctx, "00000000-0000-0000-0000-000000000001", "blocked", "reason")
		if err == nil {
			t.Fatal("expected error")
		}
		if todo.ID.Valid {
			t.Error("expected zero-value RunTodo on error")
		}
	})
}

// ---------------------------------------------------------------------------
// UpdateTokens Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_UpdateTokens_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("UpdateRunTokens error wraps as 'update tokens'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("UpdateRunTokens")
		err := o.UpdateTokens(ctx, "00000000-0000-0000-0000-000000000001", 100, 50, 0.01)
		if err == nil {
			t.Fatal("expected error")
		}
		want := "update tokens: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// BroadcastRunEvent Tests (DB failure resilience)
// ---------------------------------------------------------------------------

func TestRunOrchestrator_BroadcastRunEvent_DBResilience(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateRunEvent DB failure still broadcasts event", func(t *testing.T) {
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
		ec.waitEvents()
		ec.events = nil

		// Swap to errQueries that fails on CreateRunEvent.
		o.Queries = &errQueries{stubQueries: stubs, failOn: "CreateRunEvent"}
		o.BroadcastRunEvent(ctx, runID, "00000000-0000-0000-0000-000000000001", "run:test_event", map[string]any{
			"run_id": runID,
		})
		ec.waitEvents()

		evts := ec.byType("run:test_event")
		if len(evts) != 1 {
			t.Errorf("expected event to still be broadcast despite DB failure, got %d events", len(evts))
		}
	})

	t.Run("nil payload does not panic", func(t *testing.T) {
		o, _, ec := newTestOrchestrator()
		run, _ := o.CreateRun(ctx, CreateRunRequest{
			WorkspaceID: "00000000-0000-0000-0000-000000000001",
			IssueID:     "00000000-0000-0000-0000-000000000002",
			AgentID:     "00000000-0000-0000-0000-000000000003",
		})
		runID := util.UUIDToString(run.ID)
		ec.waitEvents()
		ec.events = nil

		// nil payload should not panic (json.Marshal(nil) = "null")
		o.BroadcastRunEvent(ctx, runID, "00000000-0000-0000-0000-000000000001", "run:nil_test", nil)
		ec.waitEvents()

		evts := ec.byType("run:nil_test")
		if len(evts) != 1 {
			t.Errorf("expected 1 event, got %d", len(evts))
		}
	})
}

// ---------------------------------------------------------------------------
// CreateHandoff Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CreateHandoff_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateRunHandoff error wraps as 'create handoff'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRunHandoff")
		_, err := o.CreateHandoff(ctx, "00000000-0000-0000-0000-000000000001", "team", "reason", "", "team-1", "", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
		want := "create handoff: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// CreateContinuation Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CreateContinuation_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateRunContinuation error wraps as 'create continuation'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRunContinuation")
		_, err := o.CreateContinuation(ctx, "00000000-0000-0000-0000-000000000001", "summary", json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), json.RawMessage(`[]`), 100)
		if err == nil {
			t.Fatal("expected error")
		}
		want := "create continuation: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// CreateArtifact Error-Path Tests
// ---------------------------------------------------------------------------

func TestRunOrchestrator_CreateArtifact_DBError(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateRunArtifact error wraps as 'create artifact'", func(t *testing.T) {
		o, _, _ := newTestOrchestratorWithErrQueries("CreateRunArtifact")
		_, err := o.CreateArtifact(ctx, "00000000-0000-0000-0000-000000000001", "", "file", "main.go", "content", "text/plain")
		if err == nil {
			t.Fatal("expected error")
		}
		want := "create artifact: injected database error"
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})
}
