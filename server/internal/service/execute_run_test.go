package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/util"
	"github.com/multica-ai/alphenix/server/pkg/agent"
)

// ---------------------------------------------------------------------------
// Stub agent backend
// ---------------------------------------------------------------------------

// stubBackend implements agent.Backend for testing ExecuteRun.
type stubBackend struct {
	messages []agent.Message
	result   agent.Result
	execErr  error
}

func (b *stubBackend) Execute(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
	if b.execErr != nil {
		return nil, b.execErr
	}

	msgCh := make(chan agent.Message, len(b.messages))
	for _, m := range b.messages {
		msgCh <- m
	}
	close(msgCh)

	resCh := make(chan agent.Result, 1)
	resCh <- b.result

	return &agent.Session{
		Messages: msgCh,
		Result:   resCh,
	}, nil
}

func (b *stubBackend) Fork(ctx context.Context, prompt string, opts agent.ForkOptions) (*agent.ForkSession, error) {
	return nil, fmt.Errorf("not implemented")
}

// ---------------------------------------------------------------------------
// ExecuteRun happy-path tests
// ---------------------------------------------------------------------------

func TestExecuteRun_CompletedRun(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	// Create a run first.
	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageText, Content: "Hello"},
			{Type: agent.MessageThinking, Content: "thinking..."},
		},
		result: agent.Result{Status: "completed", Output: "done"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:    runID,
		Prompt:   "test prompt",
		Backend:  backend,
		MaxTurns: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
	if result.RunID != runID {
		t.Errorf("expected runID %q, got %q", runID, result.RunID)
	}
	if result.Output != "done" {
		t.Errorf("expected output 'done', got %q", result.Output)
	}
}

func TestExecuteRun_WithSystemPrompt(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		result: agent.Result{Status: "completed", Output: "ok"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:        runID,
		Prompt:       "user prompt",
		SystemPrompt: "system instructions",
		Backend:      backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

func TestExecuteRun_ToolUseMessages(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageToolUse, Tool: "bash", CallID: "c1", Input: map[string]any{"cmd": "ls"}},
			{Type: agent.MessageToolResult, Tool: "bash", CallID: "c1", Output: "file.txt"},
			{Type: agent.MessageText, Content: "result text"},
		},
		result: agent.Result{Status: "completed", Output: "done"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "run ls",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps < 1 {
		t.Errorf("expected at least 1 step, got %d", result.Steps)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

func TestExecuteRun_ToolResultWithoutToolName(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Tool result with empty Tool field but matching CallID should resolve name from tool_use.
	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageToolUse, Tool: "grep", CallID: "c1", Input: map[string]any{"pattern": "foo"}},
			{Type: agent.MessageToolResult, Tool: "", CallID: "c1", Output: "found"},
		},
		result: agent.Result{Status: "completed", Output: "done"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "search",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

// ---------------------------------------------------------------------------
// ExecuteRun failure tests
// ---------------------------------------------------------------------------

func TestExecuteRun_BackendError(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		execErr: fmt.Errorf("backend connection refused"),
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	// ExecuteRun returns result with status "failed" (not an error) when backend fails.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected failed, got %q", result.Status)
	}
	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestExecuteRun_AgentFailedResult(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		result: agent.Result{Status: "failed", Error: "agent crashed"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected failed, got %q", result.Status)
	}
	if result.Error != "agent crashed" {
		t.Errorf("expected error 'agent crashed', got %q", result.Error)
	}
}

func TestExecuteRun_AgentTimeoutResult(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		result: agent.Result{Status: "timeout"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected failed, got %q", result.Status)
	}
}

func TestExecuteRun_AgentEmptyErrorResult(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Status is something other than completed/timeout, with empty error — should generate default error.
	backend := &stubBackend{
		result: agent.Result{Status: "aborted"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected failed, got %q", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty error for aborted status")
	}
}

func TestExecuteRun_ErrorMessageMessages(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageError, Content: "something went wrong"},
		},
		result: agent.Result{Status: "completed", Output: "recovered"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

func TestExecuteRun_LargeToolOutput(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Tool output > 8192 chars should be truncated.
	largeOutput := string(make([]byte, 10000))
	for i := range largeOutput {
		largeOutput = largeOutput[:i] + "x" + largeOutput[i+1:]
	}

	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageToolUse, Tool: "cat", CallID: "c1", Input: map[string]any{"file": "big.txt"}},
			{Type: agent.MessageToolResult, Tool: "cat", CallID: "c1", Output: largeOutput},
		},
		result: agent.Result{Status: "completed", Output: "ok"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "read big file",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

func TestExecuteRun_LargeToolInput(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Large input map with long values.
	largeInput := map[string]any{
		"data": string(make([]byte, 1000)),
	}

	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageToolUse, Tool: "write", CallID: "c1", Input: largeInput},
			{Type: agent.MessageToolResult, Tool: "write", CallID: "c1", Output: "ok"},
		},
		result: agent.Result{Status: "completed", Output: "done"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "write file",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

// ---------------------------------------------------------------------------
// ExecuteRun with compactor
// ---------------------------------------------------------------------------

func TestExecuteRun_WithCompactor(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()

	// Create a compactor with a small threshold to trigger compaction.
	compactor := NewCompactor()
	compactor.MaxChars = 100 // very small to trigger compaction

	o := NewRunOrchestrator(stubs, compactor, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Generate enough text content to trigger compaction.
	longText := "This is a long message that should exceed the compactor threshold. "
	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageText, Content: longText + longText + longText},
			{Type: agent.MessageText, Content: longText + longText + longText},
		},
		result: agent.Result{Status: "completed", Output: "done"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "generate long output",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

// ---------------------------------------------------------------------------
// ExecuteRun concurrent messages
// ---------------------------------------------------------------------------

func TestExecuteRun_ConcurrentMessages(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Send messages from a goroutine to simulate real async backend.
	msgCh := make(chan agent.Message, 10)
	go func() {
		for i := 0; i < 5; i++ {
			msgCh <- agent.Message{Type: agent.MessageText, Content: fmt.Sprintf("message %d", i)}
		}
		close(msgCh)
	}()

	resCh := make(chan agent.Result, 1)
	resCh <- agent.Result{Status: "completed", Output: "all done"}

	mockBackend := &concurrentBackend{
		session: &agent.Session{
			Messages: msgCh,
			Result:   resCh,
		},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: mockBackend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
}

type concurrentBackend struct {
	session *agent.Session
}

func (b *concurrentBackend) Execute(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
	return b.session, nil
}

func (b *concurrentBackend) Fork(ctx context.Context, prompt string, opts agent.ForkOptions) (*agent.ForkSession, error) {
	return nil, fmt.Errorf("not implemented")
}

// ---------------------------------------------------------------------------
// ExecuteRun start failure
// ---------------------------------------------------------------------------

func TestExecuteRun_StartRunError(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()

	// Create a run first so we have a valid ID.
	o := NewRunOrchestrator(stubs, nil, nil, bus)
	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Swap to errQueries that fails on StartRun.
	o.Queries = &errQueries{stubQueries: stubs, failOn: "StartRun"}
	_, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: &stubBackend{result: agent.Result{Status: "completed"}},
	})
	if err == nil {
		t.Fatal("expected error when StartRun fails")
	}
	if !containsStr(err.Error(), "start run") {
		t.Errorf("expected 'start run' in error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ExecuteRun complete run DB error
// ---------------------------------------------------------------------------

func TestExecuteRun_CompleteRunDBError(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// Now swap to errQueries that fails on CompleteRun.
	o.Queries = &errQueries{stubQueries: stubs, failOn: "CompleteRun"}

	backend := &stubBackend{
		result: agent.Result{Status: "completed", Output: "done"},
	}

	_, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err == nil {
		t.Fatal("expected error when CompleteRun fails")
	}
	if !containsStr(err.Error(), "complete run") {
		t.Errorf("expected 'complete run' in error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ExecuteRun empty messages channel
// ---------------------------------------------------------------------------

func TestExecuteRun_EmptyMessages(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	// No messages at all, just a result.
	backend := &stubBackend{
		messages: nil,
		result:   agent.Result{Status: "completed", Output: "silent"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
	if result.Steps != 0 {
		t.Errorf("expected 0 steps, got %d", result.Steps)
	}
}

// ---------------------------------------------------------------------------
// ExecuteRun broadcast events
// ---------------------------------------------------------------------------

func TestExecuteRun_BroadcastsRunEvents(t *testing.T) {
	ctx := context.Background()
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

	backend := &stubBackend{
		messages: []agent.Message{
			{Type: agent.MessageText, Content: "hello"},
		},
		result: agent.Result{Status: "completed", Output: "done"},
	}

	_, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ec.waitEvents()

	// Should have run:started and run:completed events.
	started := ec.byType("run:started")
	completed := ec.byType("run:completed")
	if len(started) != 1 {
		t.Errorf("expected 1 run:started event, got %d", len(started))
	}
	if len(completed) != 1 {
		t.Errorf("expected 1 run:completed event, got %d", len(completed))
	}
}

// ---------------------------------------------------------------------------
// ExecuteRun with concurrent tool calls (race condition test)
// ---------------------------------------------------------------------------

func TestExecuteRun_MultipleToolCalls(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	var wg sync.WaitGroup
	wg.Add(1)

	// Backend that sends messages from a goroutine.
	msgCh := make(chan agent.Message, 20)
	go func() {
		defer wg.Done()
		// Interleave tool use/result pairs.
		for i := 0; i < 5; i++ {
			callID := fmt.Sprintf("call-%d", i)
			msgCh <- agent.Message{Type: agent.MessageToolUse, Tool: "read", CallID: callID, Input: map[string]any{"path": fmt.Sprintf("file%d.txt", i)}}
			msgCh <- agent.Message{Type: agent.MessageToolResult, Tool: "read", CallID: callID, Output: fmt.Sprintf("content %d", i)}
		}
		msgCh <- agent.Message{Type: agent.MessageText, Content: "all files read"}
		close(msgCh)
	}()

	resCh := make(chan agent.Result, 1)
	resCh <- agent.Result{Status: "completed", Output: "done"}

	mockBackend := &concurrentBackend{
		session: &agent.Session{
			Messages: msgCh,
			Result:   resCh,
		},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "read files",
		Backend: mockBackend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps < 5 {
		t.Errorf("expected at least 5 tool steps, got %d", result.Steps)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %q", result.Status)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// ExecuteRun duration tracking
// ---------------------------------------------------------------------------

func TestExecuteRun_TracksDuration(t *testing.T) {
	ctx := context.Background()
	stubs := newStubQueries()
	bus := events.New()
	o := NewRunOrchestrator(stubs, nil, nil, bus)

	run, _ := o.CreateRun(ctx, CreateRunRequest{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		IssueID:     "00000000-0000-0000-0000-000000000002",
		AgentID:     "00000000-0000-0000-0000-000000000003",
	})
	runID := util.UUIDToString(run.ID)

	backend := &stubBackend{
		result: agent.Result{Status: "completed", Output: "ok"},
	}

	result, err := o.ExecuteRun(ctx, ExecuteRunRequest{
		RunID:   runID,
		Prompt:  "test",
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Duration < 0 {
		t.Error("expected non-negative duration")
	}
}
