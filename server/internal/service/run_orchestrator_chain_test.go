package service

import (
	"context"
	"testing"

	"github.com/multica-ai/alphenix/server/internal/util"
)

// ---------------------------------------------------------------------------
// P6: Deep-chain integration tests — multi-step end-to-end flows
// ---------------------------------------------------------------------------

func TestRunOrchestrator_DeepChain_FullLifecycleWithStepsAndTodos(t *testing.T) {
	// Scenario: Create run → start → record thinking + 3 tool steps →
	// create 2 todos → complete 1 todo → advance to reviewing → complete run.
	ctx := context.Background()
	o, stubs, ec := newTestOrchestrator()

	// 1. Create run
	run, err := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID:    "00000000-0000-0000-0000-000000000001",
		IssueID:        "00000000-0000-0000-0000-000000000002",
		AgentID:        "00000000-0000-0000-0000-000000000003",
		SystemPrompt:   "You are a helpful agent",
		ModelName:      "claude-sonnet-4-5",
		PermissionMode: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := util.UUIDToString(run.ID)
	if run.Phase != "pending" {
		t.Fatalf("expected phase=pending, got %s", run.Phase)
	}

	// 2. Start run
	started, err := o.StartRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Phase != "executing" {
		t.Fatalf("expected phase=executing, got %s", started.Phase)
	}

	// 3. Record thinking step
	thinking, err := o.RecordStep(ctx, runID, "thinking", "", "", nil, "Let me analyse the codebase...", false)
	if err != nil {
		t.Fatal(err)
	}
	if thinking.Seq != 1 {
		t.Errorf("thinking step seq = %d, want 1", thinking.Seq)
	}

	// 4. Record tool_use step (no output yet)
	readStep, err := o.RecordStep(ctx, runID, "tool_use", "read_file", "call-1", []byte(`{"path":"main.go"}`), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if readStep.Seq != 2 {
		t.Errorf("read step seq = %d, want 2", readStep.Seq)
	}

	// 5. Complete the tool step with output
	completed, err := o.CompleteStep(ctx, util.UUIDToString(readStep.ID), "package main\nfunc main() {}", false)
	if err != nil {
		t.Fatal(err)
	}
	if !completed.ToolOutput.Valid {
		t.Error("expected ToolOutput valid after CompleteStep")
	}

	// 6. Record another tool step with immediate output
	writeStep, err := o.RecordStep(ctx, runID, "tool_use", "write_file", "call-2", []byte(`{"path":"new.go"}`), "written", false)
	if err != nil {
		t.Fatal(err)
	}
	if writeStep.Seq != 3 {
		t.Errorf("write step seq = %d, want 3", writeStep.Seq)
	}

	// 7. Record text step
	textStep, err := o.RecordStep(ctx, runID, "text", "", "", nil, "Here's the analysis.", false)
	if err != nil {
		t.Fatal(err)
	}
	if textStep.Seq != 4 {
		t.Errorf("text step seq = %d, want 4", textStep.Seq)
	}

	// 8. Create todos
	todo1, err := o.CreateTodo(ctx, runID, "Fix bug", "Critical issue")
	if err != nil {
		t.Fatal(err)
	}
	todo2, err := o.CreateTodo(ctx, runID, "Add tests", "")
	if err != nil {
		t.Fatal(err)
	}
	if todo1.Seq != 1 || todo2.Seq != 2 {
		t.Errorf("todo seqs = %d, %d, want 1, 2", todo1.Seq, todo2.Seq)
	}

	// 9. Complete first todo
	updatedTodo, err := o.UpdateTodo(ctx, util.UUIDToString(todo1.ID), "completed", "")
	if err != nil {
		t.Fatal(err)
	}
	if updatedTodo.Status != "completed" {
		t.Errorf("todo status = %s, want completed", updatedTodo.Status)
	}

	// 10. Block second todo
	blockedTodo, err := o.UpdateTodo(ctx, util.UUIDToString(todo2.ID), "blocked", "Waiting for approval")
	if err != nil {
		t.Fatal(err)
	}
	if !blockedTodo.Blocker.Valid || blockedTodo.Blocker.String != "Waiting for approval" {
		t.Error("expected blocker text on todo2")
	}

	// 11. Advance to reviewing
	reviewing, err := o.AdvancePhase(ctx, runID, "reviewing")
	if err != nil {
		t.Fatal(err)
	}
	if reviewing.Phase != "reviewing" {
		t.Errorf("phase = %s, want reviewing", reviewing.Phase)
	}

	// 12. Complete run
	final, err := o.CompleteRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Phase != "completed" {
		t.Errorf("final phase = %s, want completed", final.Phase)
	}

	// Verify: all steps persisted
	steps, _ := stubs.ListRunSteps(ctx, run.ID)
	if len(steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(steps))
	}

	// Verify: all todos persisted
	todos, _ := stubs.ListRunTodos(ctx, run.ID)
	if len(todos) != 2 {
		t.Errorf("expected 2 todos, got %d", len(todos))
	}

	// Verify: full event sequence
	ec.waitEvents()
	if len(ec.byType("run:created")) != 1 {
		t.Error("missing run:created")
	}
	if len(ec.byType("run:started")) != 1 {
		t.Error("missing run:started")
	}
	if len(ec.byType("run:phase_changed")) != 1 {
		t.Error("missing run:phase_changed")
	}
	if len(ec.byType("run:completed")) != 1 {
		t.Error("missing run:completed")
	}
	if len(ec.byType("run:todo_created")) != 2 {
		t.Errorf("expected 2 todo_created events, got %d", len(ec.byType("run:todo_created")))
	}
	if len(ec.byType("run:todo_updated")) != 2 {
		t.Errorf("expected 2 todo_updated events, got %d", len(ec.byType("run:todo_updated")))
	}
}

func TestRunOrchestrator_DeepChain_RetryChain(t *testing.T) {
	// Scenario: Create run → start → fail → retry (creates child with parent_run_id) →
	// start child → complete child. Verify parent-child linkage.
	ctx := context.Background()
	o, stubs, ec := newTestOrchestrator()

	// Create + fail original
	original, err := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
		ModelName:   "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatal(err)
	}
	originalID := util.UUIDToString(original.ID)

	o.StartRun(ctx, originalID)
	o.FailRun(ctx, originalID, "timeout exceeded")

	// Retry → creates new run with parent_run_id
	retried, err := o.RetryRun(ctx, originalID)
	if err != nil {
		t.Fatal(err)
	}
	retriedID := util.UUIDToString(retried.ID)

	// Verify parent linkage
	if !retried.ParentRunID.Valid {
		t.Fatal("retried run should have ParentRunID")
	}
	if util.UUIDToString(retried.ParentRunID) != originalID {
		t.Errorf("parent = %s, want %s", util.UUIDToString(retried.ParentRunID), originalID)
	}

	// Retried run inherits config
	if retried.ModelName != "claude-sonnet-4-5" {
		t.Errorf("retried model = %s, want claude-sonnet-4-5", retried.ModelName)
	}

	// Start + complete retried run
	o.StartRun(ctx, retriedID)
	_, _ = o.RecordStep(ctx, retriedID, "tool_use", "read_file", "c1", []byte(`{}`), "output", false)
	o.CompleteRun(ctx, retriedID)

	// Verify original is still failed
	origRun, _ := stubs.GetRun(ctx, util.ParseUUID(originalID))
	if origRun.Phase != "failed" {
		t.Errorf("original phase = %s, want failed", origRun.Phase)
	}

	// Verify retried is completed
	retriedRun, _ := stubs.GetRun(ctx, util.ParseUUID(retriedID))
	if retriedRun.Phase != "completed" {
		t.Errorf("retried phase = %s, want completed", retriedRun.Phase)
	}

	// Events: 2 created + 2 started + 1 failed + 1 completed
	ec.waitEvents()
	if len(ec.byType("run:created")) != 2 {
		t.Errorf("expected 2 run:created, got %d", len(ec.byType("run:created")))
	}
	if len(ec.byType("run:failed")) != 1 {
		t.Errorf("expected 1 run:failed, got %d", len(ec.byType("run:failed")))
	}
	if len(ec.byType("run:completed")) != 1 {
		t.Errorf("expected 1 run:completed, got %d", len(ec.byType("run:completed")))
	}
}

func TestRunOrchestrator_DeepChain_MultiRunSameIssue(t *testing.T) {
	// Scenario: Two runs for the same issue (fork pattern) —
	// create parent, create child with parent_run_id, both execute independently.
	ctx := context.Background()
	o, stubs, ec := newTestOrchestrator()

	// Parent run
	_, parentID := createParentRun(t, o, "task-multi")

	// Child run (fork)
	_, childID := createChildRun(t, o, "task-multi-child", parentID)

	// Both run independently
	o.StartRun(ctx, parentID)
	o.StartRun(ctx, childID)

	// Parent records steps
	_, _ = o.RecordStep(ctx, parentID, "tool_use", "read_file", "p1", []byte(`{}`), "parent output", false)
	o.CompleteRun(ctx, parentID)

	// Child records different steps
	_, _ = o.RecordStep(ctx, childID, "tool_use", "write_file", "c1", []byte(`{}`), "child output", false)
	o.AdvancePhase(ctx, childID, "reviewing")
	o.CompleteRun(ctx, childID)

	// Verify: both runs completed independently
	parentRun, _ := stubs.GetRun(ctx, util.ParseUUID(parentID))
	childRun, _ := stubs.GetRun(ctx, util.ParseUUID(childID))
	if parentRun.Phase != "completed" {
		t.Errorf("parent phase = %s, want completed", parentRun.Phase)
	}
	if childRun.Phase != "completed" {
		t.Errorf("child phase = %s, want completed", childRun.Phase)
	}

	// Verify: child has correct parent linkage
	if util.UUIDToString(childRun.ParentRunID) != parentID {
		t.Errorf("child ParentRunID = %s, want %s", util.UUIDToString(childRun.ParentRunID), parentID)
	}

	// Verify: steps are isolated per run
	parentSteps, _ := stubs.ListRunSteps(ctx, parentRun.ID)
	childSteps, _ := stubs.ListRunSteps(ctx, childRun.ID)
	if len(parentSteps) != 1 {
		t.Errorf("parent steps = %d, want 1", len(parentSteps))
	}
	if len(childSteps) != 1 {
		t.Errorf("child steps = %d, want 1", len(childSteps))
	}

	// Verify events
	ec.waitEvents()
	if len(ec.byType("run:created")) != 2 {
		t.Errorf("expected 2 run:created, got %d", len(ec.byType("run:created")))
	}
	if len(ec.byType("run:completed")) != 2 {
		t.Errorf("expected 2 run:completed, got %d", len(ec.byType("run:completed")))
	}
}

func TestRunOrchestrator_DeepChain_HandoffAndContinuation(t *testing.T) {
	// Scenario: Run executes → creates handoff to another team →
	// creates continuation record → completes. Verify all artifacts are linked.
	ctx := context.Background()
	o, stubs, ec := newTestOrchestrator()

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	o.StartRun(ctx, runID)

	// Record some work
	_, _ = o.RecordStep(ctx, runID, "tool_use", "read_file", "h1", []byte(`{}`), "code content", false)

	// Create handoff
	handoff, err := o.CreateHandoff(ctx, runID, "team",
		"Need backend team to review API changes",
		"", "team-backend", "agent-reviewer",
		[]byte(`{"files":["api.go"],"context":"review needed"}`))
	if err != nil {
		t.Fatal("CreateHandoff:", err)
	}
	if handoff.HandoffType != "team" {
		t.Errorf("handoff type = %s, want team", handoff.HandoffType)
	}
	if handoff.Reason != "Need backend team to review API changes" {
		t.Errorf("handoff reason = %s", handoff.Reason)
	}

	// Create continuation
	cont, err := o.CreateContinuation(ctx, runID,
		"Reviewed 3 files, found 2 issues",
		[]byte(`["Fix race condition","Add error handling"]`),
		[]byte(`["Use sync.Mutex","Check err return"]`),
		[]byte(`["api.go","handler.go"]`),
		[]byte(`[]`),
		[]byte(`["Need performance benchmarks"]`),
		5000)
	if err != nil {
		t.Fatal("CreateContinuation:", err)
	}
	if cont.CompactSummary != "Reviewed 3 files, found 2 issues" {
		t.Errorf("continuation summary = %s", cont.CompactSummary)
	}
	if cont.TokenBudgetUsed != 5000 {
		t.Errorf("token budget = %d, want 5000", cont.TokenBudgetUsed)
	}

	// Create artifact
	artifact, err := o.CreateArtifact(ctx, runID, "", "patch", "fix.patch", "--- a/api.go\n+++ b/api.go", "text/x-patch")
	if err != nil {
		t.Fatal("CreateArtifact:", err)
	}
	if artifact.ArtifactType != "patch" {
		t.Errorf("artifact type = %s, want patch", artifact.ArtifactType)
	}

	// Complete run
	o.CompleteRun(ctx, runID)

	// Verify: handoff stored
	if len(stubs.handoffs) != 1 {
		t.Errorf("expected 1 handoff, got %d", len(stubs.handoffs))
	}
	// Verify: continuation stored
	if len(stubs.continuations) != 1 {
		t.Errorf("expected 1 continuation, got %d", len(stubs.continuations))
	}
	// Verify: artifact stored
	artifacts, _ := stubs.ListRunArtifacts(ctx, run.ID)
	if len(artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(artifacts))
	}

	// Verify: events include handoff broadcast
	ec.waitEvents()
	if len(ec.byType("run:completed")) != 1 {
		t.Error("missing run:completed")
	}
}

func TestRunOrchestrator_DeepChain_CancelAfterPartialWork(t *testing.T) {
	// Scenario: Run starts, does partial work (steps + todos), then gets cancelled.
	// Verify: run is cancelled, steps and todos persist.
	ctx := context.Background()
	o, stubs, ec := newTestOrchestrator()

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	o.StartRun(ctx, runID)

	// Do partial work
	_, _ = o.RecordStep(ctx, runID, "tool_use", "read_file", "c1", []byte(`{}`), "partial", false)
	_, _ = o.RecordStep(ctx, runID, "tool_use", "write_file", "c2", []byte(`{}`), "wrote half", false)
	todo, _ := o.CreateTodo(ctx, runID, "Finish implementation", "")
	_, _ = o.UpdateTodo(ctx, util.UUIDToString(todo.ID), "in_progress", "")

	// Cancel mid-execution
	cancelled, err := o.CancelRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Phase != "cancelled" {
		t.Errorf("phase = %s, want cancelled", cancelled.Phase)
	}
	if !cancelled.CompletedAt.Valid {
		t.Error("expected CompletedAt to be set on cancellation")
	}

	// Partial work persists
	steps, _ := stubs.ListRunSteps(ctx, run.ID)
	if len(steps) != 2 {
		t.Errorf("expected 2 steps to persist after cancel, got %d", len(steps))
	}
	todos, _ := stubs.ListRunTodos(ctx, run.ID)
	if len(todos) != 1 {
		t.Errorf("expected 1 todo to persist after cancel, got %d", len(todos))
	}
	if todos[0].Status != "in_progress" {
		t.Errorf("todo status = %s, want in_progress", todos[0].Status)
	}

	// Events
	ec.waitEvents()
	if len(ec.byType("run:cancelled")) != 1 {
		t.Error("missing run:cancelled event")
	}
}

func TestRunOrchestrator_DeepChain_ErrorStepsAndFailRecovery(t *testing.T) {
	// Scenario: Run starts → records error step → records normal step → fails →
	// retry → succeeds.
	ctx := context.Background()
	o, stubs, ec := newTestOrchestrator()

	// First attempt: fails with error step
	run1, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	run1ID := util.UUIDToString(run1.ID)

	o.StartRun(ctx, run1ID)
	errorStep, _ := o.RecordStep(ctx, run1ID, "tool_use", "bash", "e1", []byte(`{"command":"deploy"}`), "permission denied", true)
	if !errorStep.IsError {
		t.Error("expected IsError=true on error step")
	}
	o.FailRun(ctx, run1ID, "deployment failed: permission denied")

	// Retry
	run2, _ := o.RetryRun(ctx, run1ID)
	run2ID := util.UUIDToString(run2.ID)

	o.StartRun(ctx, run2ID)
	okStep, _ := o.RecordStep(ctx, run2ID, "tool_use", "bash", "r1", []byte(`{"command":"deploy"}`), "deployed successfully", false)
	if okStep.IsError {
		t.Error("expected IsError=false on success step")
	}
	o.CompleteRun(ctx, run2ID)

	// Verify: first run has error step
	steps1, _ := stubs.ListRunSteps(ctx, run1.ID)
	if len(steps1) != 1 || !steps1[0].IsError {
		t.Error("first run should have 1 error step")
	}

	// Verify: second run has success step
	steps2, _ := stubs.ListRunSteps(ctx, run2.ID)
	if len(steps2) != 1 || steps2[0].IsError {
		t.Error("second run should have 1 non-error step")
	}

	// Verify phases
	r1, _ := stubs.GetRun(ctx, util.ParseUUID(run1ID))
	r2, _ := stubs.GetRun(ctx, util.ParseUUID(run2ID))
	if r1.Phase != "failed" {
		t.Errorf("run1 phase = %s, want failed", r1.Phase)
	}
	if r2.Phase != "completed" {
		t.Errorf("run2 phase = %s, want completed", r2.Phase)
	}

	ec.waitEvents()
	if len(ec.byType("run:failed")) != 1 {
		t.Errorf("expected 1 run:failed, got %d", len(ec.byType("run:failed")))
	}
	if len(ec.byType("run:completed")) != 1 {
		t.Errorf("expected 1 run:completed, got %d", len(ec.byType("run:completed")))
	}
}

func TestRunOrchestrator_DeepChain_FullPhaseProgression(t *testing.T) {
	// Scenario: Walk through every valid phase: pending → planning → executing → reviewing → completed.
	// Verify each transition broadcasts the correct event.
	ctx := context.Background()
	o, _, ec := newTestOrchestrator()

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// pending → planning
	phase1, err := o.AdvancePhase(ctx, runID, "planning")
	if err != nil {
		t.Fatal("pending→planning:", err)
	}
	if phase1.Phase != "planning" {
		t.Errorf("phase = %s, want planning", phase1.Phase)
	}

	// planning → executing
	phase2, err := o.AdvancePhase(ctx, runID, "executing")
	if err != nil {
		t.Fatal("planning→executing:", err)
	}
	if phase2.Phase != "executing" {
		t.Errorf("phase = %s, want executing", phase2.Phase)
	}

	// executing → reviewing
	phase3, err := o.AdvancePhase(ctx, runID, "reviewing")
	if err != nil {
		t.Fatal("executing→reviewing:", err)
	}
	if phase3.Phase != "reviewing" {
		t.Errorf("phase = %s, want reviewing", phase3.Phase)
	}

	// reviewing → completed
	phase4, err := o.CompleteRun(ctx, runID)
	if err != nil {
		t.Fatal("reviewing→completed:", err)
	}
	if phase4.Phase != "completed" {
		t.Errorf("phase = %s, want completed", phase4.Phase)
	}

	// Verify: exactly 3 phase_changed events (planning, executing, reviewing)
	ec.waitEvents()
	phaseEvents := ec.byType("run:phase_changed")
	if len(phaseEvents) != 3 {
		t.Errorf("expected 3 phase_changed events, got %d", len(phaseEvents))
	}

	// Verify: completed event
	if len(ec.byType("run:completed")) != 1 {
		t.Error("missing run:completed")
	}
}
