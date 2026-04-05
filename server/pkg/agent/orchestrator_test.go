package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Scope 1: Task Assignment Protocol ---

func TestBuildWorkerContext_Basic(t *testing.T) {
	spec := TaskSpec{
		ID:          "task-1",
		Description: "Add a 'priority' field to the handler",
		Context:     "See server/internal/handler/issue.go for context.",
		Timeout:     30 * time.Second,
	}

	wctx := BuildWorkerContext(spec, 60*time.Second)

	if wctx.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", wctx.TaskID, "task-1")
	}
	if !strings.Contains(wctx.Prompt, "Add a 'priority' field") {
		t.Errorf("Prompt missing description: %q", wctx.Prompt)
	}
	if !strings.Contains(wctx.Prompt, "issue.go") {
		t.Errorf("Prompt missing context: %q", wctx.Prompt)
	}
	if wctx.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", wctx.Timeout, 30*time.Second)
	}
}

func TestBuildWorkerContext_DefaultTimeout(t *testing.T) {
	spec := TaskSpec{
		ID:          "task-2",
		Description: "Run tests",
	}

	wctx := BuildWorkerContext(spec, 90*time.Second)
	if wctx.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want %v (should use default)", wctx.Timeout, 90*time.Second)
	}
}

func TestBuildWorkerContext_ContextBeforeDescription(t *testing.T) {
	spec := TaskSpec{
		ID:          "task-3",
		Description: "Fix the bug",
		Context:     "file.go:42 has a nil dereference",
	}

	wctx := BuildWorkerContext(spec, 60*time.Second)

	ctxIdx := strings.Index(wctx.Prompt, "file.go:42")
	descIdx := strings.Index(wctx.Prompt, "Fix the bug")
	if ctxIdx < 0 || descIdx < 0 {
		t.Fatalf("Prompt missing context or description: %q", wctx.Prompt)
	}
	if ctxIdx > descIdx {
		t.Errorf("Context should appear before description in prompt")
	}
}

// --- Scope 2: Handoff Mechanism / Result Aggregation ---

func TestOrchestrator_AggregateByCompletionOrder(t *testing.T) {
	delays := map[string]time.Duration{
		"task-1": 100 * time.Millisecond,
		"task-2": 50 * time.Millisecond,
		"task-3": 75 * time.Millisecond,
	}

	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			delay := 50 * time.Millisecond
			for id, d := range delays {
				if strings.Contains(prompt, id) {
					delay = d
					break
				}
			}

			resultCh := make(chan ForkResult, 1)
			go func() {
				time.Sleep(delay)
				resultCh <- ForkResult{
					Status: "completed",
					Output: fmt.Sprintf("result-%s", prompt),
				}
			}()
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
	})
	defer orch.Close()

	tasks := []TaskSpec{
		{ID: "task-1", Description: "task-1 work"},
		{ID: "task-2", Description: "task-2 work"},
		{ID: "task-3", Description: "task-3 work"},
	}

	n, err := orch.SubmitTasks(context.Background(), tasks)
	if err != nil {
		t.Fatalf("SubmitTasks: %v", err)
	}
	if n != 3 {
		t.Errorf("queued = %d, want 3", n)
	}

	result := orch.WaitAll()

	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if result.Succeeded != 3 {
		t.Errorf("Succeeded = %d, want 3", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}

	if len(result.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(result.Results))
	}

	seen := make(map[string]bool)
	for _, r := range result.Results {
		seen[r.TaskID] = true
		if r.Status != WorkerStateCompleted {
			t.Errorf("worker %s status = %q, want %q", r.TaskID, r.Status, WorkerStateCompleted)
		}
	}
	for _, id := range []string{"task-1", "task-2", "task-3"} {
		if !seen[id] {
			t.Errorf("missing result for %s", id)
		}
	}
}

func TestOrchestrator_StructuredOutputExtraction(t *testing.T) {
	backend := newMockBackend("completed", "Files changed: handler.go\nTests: 5/5 passed")

	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "do work"},
	})

	result := orch.WaitAll()
	if len(result.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(result.Results))
	}

	output := result.Results[0].Output
	if !strings.Contains(output, "Files changed") {
		t.Errorf("Output missing structured content: %q", output)
	}
}

func TestOrchestrator_DeduplicateByTaskID(t *testing.T) {
	backend := newMockBackend("completed", "ok")
	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
	})
	defer orch.Close()

	_, err := orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})
	if err != nil {
		t.Fatalf("first SubmitTasks: %v", err)
	}

	_, err = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "duplicate"},
	})
	if err == nil {
		t.Fatal("duplicate task ID should fail")
	}
}

func TestOrchestrator_AggregatedResultSummary(t *testing.T) {
	agg := AggregatedResult{
		Results: []WorkerOutput{
			{TaskID: "a", Status: WorkerStateCompleted, Output: "ok", Attempt: 1, DurationMs: 100},
			{TaskID: "b", Status: WorkerStateFailed, Error: "boom", Attempt: 2, DurationMs: 200},
		},
		Succeeded: 1,
		Failed:    1,
		Total:     2,
	}

	summary := agg.Summary()

	if !strings.Contains(summary, "1 succeeded") {
		t.Error("summary should mention 1 succeeded")
	}
	if !strings.Contains(summary, "1 failed") {
		t.Error("summary should mention 1 failed")
	}
	if !strings.Contains(summary, "a (status: completed") {
		t.Error("summary should contain task a details")
	}
	if !strings.Contains(summary, "b (status: failed") {
		t.Error("summary should contain task b details")
	}
}

// --- Scope 3: Worker State Machine ---

func TestOrchestrator_WorkerStateTransitions(t *testing.T) {
	backend := newMockBackend("completed", "done")

	var transitions []string
	var transMu sync.Mutex
	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
		OnWorkerStateChange: func(taskID string, oldState, newState WorkerState) {
			transMu.Lock()
			transitions = append(transitions, fmt.Sprintf("%s:%s->%s", taskID, oldState, newState))
			transMu.Unlock()
		},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})

	orch.WaitAll()

	transMu.Lock()
	defer transMu.Unlock()

	if len(transitions) < 2 {
		t.Fatalf("got %d transitions, want at least 2: %v", len(transitions), transitions)
	}

	found := false
	for _, tr := range transitions {
		if tr == "task-1:pending->running" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing pending→running transition, got: %v", transitions)
	}

	status, _ := orch.WorkerStatus("task-1")
	if status.State != WorkerStateCompleted {
		t.Errorf("final state = %q, want %q", status.State, WorkerStateCompleted)
	}
}

func TestOrchestrator_WorkerStatusQuery(t *testing.T) {
	backend := newSlowMockBackend(100*time.Millisecond, "completed", "ok")
	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})

	time.Sleep(20 * time.Millisecond)
	status, err := orch.WorkerStatus("task-1")
	if err != nil {
		t.Fatalf("WorkerStatus: %v", err)
	}
	if status.State != WorkerStateRunning {
		t.Errorf("state while running = %q, want %q", status.State, WorkerStateRunning)
	}

	orch.WaitAll()

	status, err = orch.WorkerStatus("task-1")
	if err != nil {
		t.Fatalf("WorkerStatus after completion: %v", err)
	}
	if status.State != WorkerStateCompleted {
		t.Errorf("final state = %q, want %q", status.State, WorkerStateCompleted)
	}
	if status.Attempt != 1 {
		t.Errorf("attempt = %d, want 1", status.Attempt)
	}
}

func TestOrchestrator_FailedWorkerStateTransition(t *testing.T) {
	backend := newFailingMockBackend("launch failed")

	var transitions []string
	var transMu sync.Mutex
	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0, BackoffBase: 1 * time.Millisecond},
		OnWorkerStateChange: func(taskID string, oldState, newState WorkerState) {
			transMu.Lock()
			transitions = append(transitions, fmt.Sprintf("%s:%s->%s", taskID, oldState, newState))
			transMu.Unlock()
		},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})

	result := orch.WaitAll()

	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}

	status, _ := orch.WorkerStatus("task-1")
	if status.State != WorkerStateFailed {
		t.Errorf("final state = %q, want %q", status.State, WorkerStateFailed)
	}
}

func TestOrchestrator_ActiveWorkers(t *testing.T) {
	backend := newSlowMockBackend(200*time.Millisecond, "completed", "ok")
	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
		{ID: "task-2", Description: "work"},
	})

	time.Sleep(50 * time.Millisecond)
	if n := orch.ActiveWorkers(); n != 2 {
		t.Errorf("ActiveWorkers = %d, want 2", n)
	}

	orch.WaitAll()
	if n := orch.ActiveWorkers(); n != 0 {
		t.Errorf("ActiveWorkers after WaitAll = %d, want 0", n)
	}
}

// --- Scope 4: Error Handling (Retry, Backoff, Timeout) ---

func TestOrchestrator_RetryOnFailure(t *testing.T) {
	callCount := 0
	callMu := sync.Mutex{}
	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			callMu.Lock()
			callCount++
			c := callCount
			callMu.Unlock()

			resultCh := make(chan ForkResult, 1)
			if c <= 2 {
				resultCh <- ForkResult{Status: "failed", Error: "transient error"}
			} else {
				resultCh <- ForkResult{Status: "completed", Output: "success"}
			}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy: RetryPolicy{
			MaxRetries:    2,
			BackoffBase:   1 * time.Millisecond,
			BackoffFactor: 2.0,
		},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})

	result := orch.WaitAll()

	if result.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", result.Succeeded)
	}

	status, _ := orch.WorkerStatus("task-1")
	if status.State != WorkerStateCompleted {
		t.Errorf("final state = %q, want %q", status.State, WorkerStateCompleted)
	}
	if status.Attempt != 3 {
		t.Errorf("attempt = %d, want 3", status.Attempt)
	}
	callMu.Lock()
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3 (should have retried twice)", callCount)
	}
	callMu.Unlock()
}

func TestOrchestrator_RetryExhausted(t *testing.T) {
	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			resultCh := make(chan ForkResult, 1)
			resultCh <- ForkResult{Status: "failed", Error: "permanent error"}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy: RetryPolicy{
			MaxRetries:    2,
			BackoffBase:   1 * time.Millisecond,
			BackoffFactor: 2.0,
		},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})

	result := orch.WaitAll()

	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}

	status, _ := orch.WorkerStatus("task-1")
	if status.State != WorkerStateFailed {
		t.Errorf("final state = %q, want %q", status.State, WorkerStateFailed)
	}
	if status.Attempt != 3 {
		t.Errorf("attempt = %d, want 3 (maxRetries=2 means 3 total attempts)", status.Attempt)
	}
}

func TestRetryPolicy_BackoffDelay(t *testing.T) {
	rp := RetryPolicy{
		MaxRetries:    3,
		BackoffBase:   1 * time.Second,
		BackoffMax:    10 * time.Second,
		BackoffFactor: 2.0,
	}

	if d := rp.BackoffDelay(1); d != 0 {
		t.Errorf("attempt 1: delay = %v, want 0", d)
	}
	if d := rp.BackoffDelay(2); d != 1*time.Second {
		t.Errorf("attempt 2: delay = %v, want 1s", d)
	}
	if d := rp.BackoffDelay(3); d != 2*time.Second {
		t.Errorf("attempt 3: delay = %v, want 2s", d)
	}
	if d := rp.BackoffDelay(4); d != 4*time.Second {
		t.Errorf("attempt 4: delay = %v, want 4s", d)
	}

	rp2 := RetryPolicy{
		BackoffBase:   1 * time.Second,
		BackoffMax:    5 * time.Second,
		BackoffFactor: 2.0,
	}
	if d := rp2.BackoffDelay(5); d != 5*time.Second {
		t.Errorf("attempt 5 with cap: delay = %v, want 5s (capped)", d)
	}
}

func TestOrchestrator_DegradationContinue(t *testing.T) {
	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			resultCh := make(chan ForkResult, 1)
			if strings.Contains(prompt, "will fail") {
				resultCh <- ForkResult{Status: "failed", Error: "task failed"}
			} else {
				resultCh <- ForkResult{Status: "completed", Output: "ok"}
			}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0, BackoffBase: 1 * time.Millisecond},
		Degradation:    DegradationContinue,
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "will fail"},
		{ID: "task-2", Description: "will succeed"},
	})

	result := orch.WaitAll()

	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if result.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
}

func TestOrchestrator_TimeoutNoRetry(t *testing.T) {
	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			resultCh := make(chan ForkResult, 1)
			resultCh <- ForkResult{Status: "timeout", Error: "context deadline exceeded"}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 2, BackoffBase: 1 * time.Millisecond},
	})
	defer orch.Close()

	_, _ = orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})

	orch.WaitAll()

	status, _ := orch.WorkerStatus("task-1")
	if status.State != WorkerStateTimeout {
		t.Errorf("state = %q, want %q (timeout should not retry)", status.State, WorkerStateTimeout)
	}
	if status.Attempt != 1 {
		t.Errorf("attempt = %d, want 1 (should not retry on timeout)", status.Attempt)
	}
}

func TestOrchestrator_ClosePreventsSubmission(t *testing.T) {
	backend := newMockBackend("completed", "ok")
	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
	})

	orch.Close()

	_, err := orch.SubmitTasks(context.Background(), []TaskSpec{
		{ID: "task-1", Description: "work"},
	})
	if err == nil {
		t.Fatal("SubmitTasks after Close should fail")
	}
}

func TestOrchestrator_EmptyDispatch(t *testing.T) {
	backend := newMockBackend("completed", "ok")
	orch := NewOrchestrator(backend, t.TempDir(), OrchestratorConfig{
		DefaultTimeout: 10 * time.Second,
		RetryPolicy:    RetryPolicy{MaxRetries: 0},
	})
	defer orch.Close()

	n, err := orch.SubmitTasks(context.Background(), nil)
	if err != nil {
		t.Fatalf("SubmitTasks nil: %v", err)
	}
	if n != 0 {
		t.Errorf("queued = %d, want 0", n)
	}

	agg := orch.WaitAll()
	if len(agg.Results) != 0 {
		t.Errorf("expected 0 results for empty dispatch, got %d", len(agg.Results))
	}
}
