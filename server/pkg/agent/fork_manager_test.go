package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockBackend implements Backend for testing ForkManager.
type mockBackend struct {
	forkFunc func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error)
}

func (m *mockBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	return nil, nil
}

func (m *mockBackend) Fork(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
	return m.forkFunc(ctx, prompt, opts)
}

// newMockBackend creates a mock that completes forks immediately with the given status/output.
func newMockBackend(status, output string) *mockBackend {
	return &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			resultCh := make(chan ForkResult, 1)
			resultCh <- ForkResult{
				Status:     status,
				Output:     output,
				DurationMs: 100,
			}
			return &ForkSession{
				Result:     resultCh,
				OutputFile: opts.OutputFile,
			}, nil
		},
	}
}

// newSlowMockBackend creates a mock that delays before completing.
func newSlowMockBackend(delay time.Duration, status, output string) *mockBackend {
	return &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			resultCh := make(chan ForkResult, 1)
			go func() {
				time.Sleep(delay)
				resultCh <- ForkResult{
					Status:     status,
					Output:     output,
					DurationMs: int64(delay.Milliseconds()),
				}
			}()
			return &ForkSession{
				Result:     resultCh,
				OutputFile: opts.OutputFile,
			}, nil
		},
	}
}

// newFailingMockBackend creates a mock that returns an error on Fork().
func newFailingMockBackend(errMsg string) *mockBackend {
	return &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			return nil, fmt.Errorf("%s", errMsg)
		},
	}
}

func TestForkManager_StartAndWaitSingle(t *testing.T) {
	backend := newMockBackend("completed", "result output")
	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})
	defer fm.Close()

	id, err := fm.StartFork(context.Background(), ForkSpec{
		ID:     "fork-1",
		Prompt: "do something",
	})
	if err != nil {
		t.Fatalf("StartFork: %v", err)
	}
	if id != "fork-1" {
		t.Errorf("StartFork returned %q, want %q", id, "fork-1")
	}

	output, err := fm.WaitFork("fork-1")
	if err != nil {
		t.Fatalf("WaitFork: %v", err)
	}
	if output.Status != "completed" {
		t.Errorf("Status = %q, want %q", output.Status, "completed")
	}
	if output.Output != "result output" {
		t.Errorf("Output = %q, want %q", output.Output, "result output")
	}
}

func TestForkManager_ParallelForks(t *testing.T) {
	backend := newSlowMockBackend(50*time.Millisecond, "completed", "done")
	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})
	defer fm.Close()

	// Start 3 forks in parallel.
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("fork-%d", i)
		_, err := fm.StartFork(context.Background(), ForkSpec{
			ID:     id,
			Prompt: fmt.Sprintf("task %d", i),
		})
		if err != nil {
			t.Fatalf("StartFork(%s): %v", id, err)
		}
	}

	results := fm.WaitAll()
	if len(results) != 3 {
		t.Fatalf("WaitAll returned %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.Status != "completed" {
			t.Errorf("fork %s status = %q, want %q", r.Spec.ID, r.Status, "completed")
		}
		if r.Output != "done" {
			t.Errorf("fork %s output = %q, want %q", r.Spec.ID, r.Output, "done")
		}
	}
}

func TestForkManager_ForkFailureDoesNotAffectOthers(t *testing.T) {
	callCount := 0
	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			callCount++
			if callCount == 2 {
				return nil, fmt.Errorf("launch failed")
			}
			resultCh := make(chan ForkResult, 1)
			resultCh <- ForkResult{Status: "completed", Output: "ok"}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})
	defer fm.Close()

	// Fork 1 succeeds.
	_, err := fm.StartFork(context.Background(), ForkSpec{ID: "fork-1", Prompt: "task 1"})
	if err != nil {
		t.Fatalf("StartFork(fork-1): %v", err)
	}
	// Fork 2 fails to launch.
	_, err = fm.StartFork(context.Background(), ForkSpec{ID: "fork-2", Prompt: "task 2"})
	if err == nil {
		t.Fatal("StartFork(fork-2) should have failed")
	}
	// Fork 3 succeeds.
	_, err = fm.StartFork(context.Background(), ForkSpec{ID: "fork-3", Prompt: "task 3"})
	if err != nil {
		t.Fatalf("StartFork(fork-3): %v", err)
	}

	// Only 2 forks should be tracked (fork-1 and fork-3).
	results := fm.WaitAll()
	if len(results) != 2 {
		t.Fatalf("WaitAll returned %d results, want 2", len(results))
	}
}

func TestForkManager_WorktreeIsolation(t *testing.T) {
	// Verify that each fork gets its own worktree directory.
	var worktrees []string
	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			worktrees = append(worktrees, opts.Cwd)
			resultCh := make(chan ForkResult, 1)
			resultCh <- ForkResult{Status: "completed", Output: "ok"}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})
	defer fm.Close()

	for i := 1; i <= 3; i++ {
		_, err := fm.StartFork(context.Background(), ForkSpec{
			ID:     fmt.Sprintf("fork-%d", i),
			Prompt: "task",
		})
		if err != nil {
			t.Fatalf("StartFork: %v", err)
		}
	}

	fm.WaitAll()

	if len(worktrees) != 3 {
		t.Fatalf("got %d worktrees, want 3", len(worktrees))
	}

	// All worktrees must be unique.
	seen := make(map[string]bool)
	for _, wt := range worktrees {
		if seen[wt] {
			t.Errorf("duplicate worktree: %s", wt)
		}
		seen[wt] = true
		// Each must be a real directory.
		if _, err := os.Stat(wt); os.IsNotExist(err) {
			t.Errorf("worktree %s does not exist", wt)
		}
	}
}

func TestForkManager_CleanupOnClose(t *testing.T) {
	backend := newMockBackend("completed", "done")
	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})

	var worktrees []string
	// Override backend to capture worktrees.
	backend2 := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			worktrees = append(worktrees, opts.Cwd)
			resultCh := make(chan ForkResult, 1)
			resultCh <- ForkResult{Status: "completed", Output: "ok"}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}
	fm = NewForkManager(backend2, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})

	_, _ = fm.StartFork(context.Background(), ForkSpec{ID: "fork-1", Prompt: "task"})
	fm.WaitAll()
	fm.Close()

	// Worktrees should be cleaned up.
	for _, wt := range worktrees {
		if _, err := os.Stat(wt); !os.IsNotExist(err) {
			t.Errorf("worktree %s should have been cleaned up", wt)
		}
	}

	// Cannot start new forks after close.
	_, err := fm.StartFork(context.Background(), ForkSpec{ID: "fork-2", Prompt: "task"})
	if err == nil {
		t.Fatal("StartFork after Close should fail")
	}
}

func TestForkManager_LifecycleCallbacks(t *testing.T) {
	var startedIDs []string
	var completedIDs []string
	var failedIDs []string

	backend := newMockBackend("completed", "ok")
	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
		OnForkStarted: func(forkID string, spec ForkSpec) {
			startedIDs = append(startedIDs, forkID)
		},
		OnForkCompleted: func(forkID string, output ForkOutput) {
			completedIDs = append(completedIDs, forkID)
		},
	})
	defer fm.Close()

	_, _ = fm.StartFork(context.Background(), ForkSpec{ID: "fork-1", Prompt: "task"})
	fm.WaitAll()

	if len(startedIDs) != 1 || startedIDs[0] != "fork-1" {
		t.Errorf("startedIDs = %v, want [fork-1]", startedIDs)
	}
	if len(completedIDs) != 1 || completedIDs[0] != "fork-1" {
		t.Errorf("completedIDs = %v, want [fork-1]", completedIDs)
	}
	if len(failedIDs) != 0 {
		t.Errorf("failedIDs = %v, want []", failedIDs)
	}
}

func TestForkManager_DuplicateID(t *testing.T) {
	backend := newMockBackend("completed", "ok")
	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})
	defer fm.Close()

	_, err := fm.StartFork(context.Background(), ForkSpec{ID: "fork-1", Prompt: "task"})
	if err != nil {
		t.Fatalf("first StartFork: %v", err)
	}
	_, err = fm.StartFork(context.Background(), ForkSpec{ID: "fork-1", Prompt: "task"})
	if err == nil {
		t.Fatal("duplicate fork ID should fail")
	}
}

func TestForkManager_OutputFileContent(t *testing.T) {
	// Simulate a fork that writes to its output file.
	backend := &mockBackend{
		forkFunc: func(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
			// Write to the output file to simulate the fork writing its result.
			if opts.OutputFile != "" {
				os.MkdirAll(filepath.Dir(opts.OutputFile), 0o755)
				os.WriteFile(opts.OutputFile, []byte("file-based output"), 0o644)
			}
			resultCh := make(chan ForkResult, 1)
			// Return empty Output in ForkResult — should read from file.
			resultCh <- ForkResult{Status: "completed", Output: ""}
			return &ForkSession{Result: resultCh, OutputFile: opts.OutputFile}, nil
		},
	}

	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})
	defer fm.Close()

	_, _ = fm.StartFork(context.Background(), ForkSpec{ID: "fork-1", Prompt: "task"})
	output, err := fm.WaitFork("fork-1")
	if err != nil {
		t.Fatalf("WaitFork: %v", err)
	}
	if output.Output != "file-based output" {
		t.Errorf("Output = %q, want %q", output.Output, "file-based output")
	}
}

func TestForkManager_ActiveCount(t *testing.T) {
	backend := newSlowMockBackend(100*time.Millisecond, "completed", "ok")
	fm := NewForkManager(backend, t.TempDir(), ForkManagerOptions{
		DefaultTimeout: 10 * time.Second,
	})
	defer fm.Close()

	_, _ = fm.StartFork(context.Background(), ForkSpec{ID: "fork-1", Prompt: "task"})
	_, _ = fm.StartFork(context.Background(), ForkSpec{ID: "fork-2", Prompt: "task"})

	if n := fm.ActiveCount(); n != 2 {
		t.Errorf("ActiveCount = %d, want 2", n)
	}

	fm.WaitAll()

	if n := fm.ActiveCount(); n != 0 {
		t.Errorf("ActiveCount = %d after WaitAll, want 0", n)
	}
}
