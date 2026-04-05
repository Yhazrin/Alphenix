package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multicode/server/pkg/agent"
)

// --- Mock Backend for ForkManager tests ---

// mockBackend implements agent.Backend for testing fork orchestration.
type mockBackend struct {
	mu      sync.Mutex
	forks   []mockForkRecord
	results map[string]agent.ForkResult // keyed by output file path
}

type mockForkRecord struct {
	Prompt  string
	Opts    agent.ForkOptions
	StartAt time.Time
}

func (m *mockBackend) Execute(_ context.Context, _ string, _ agent.ExecOptions) (*agent.Session, error) {
	return nil, fmt.Errorf("mockBackend.Execute not implemented")
}

func (m *mockBackend) Fork(_ context.Context, prompt string, opts agent.ForkOptions) (*agent.ForkSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.forks = append(m.forks, mockForkRecord{
		Prompt:  prompt,
		Opts:    opts,
		StartAt: time.Now(),
	})

	ch := make(chan agent.ForkResult, 1)
	result, ok := m.results[opts.OutputFile]
	if !ok {
		result = agent.ForkResult{Status: "completed", Output: "default output"}
	}

	// Simulate async completion: write output to file (like a real agent would).
	go func() {
		time.Sleep(50 * time.Millisecond)
		if result.Output != "" && opts.OutputFile != "" {
			_ = os.MkdirAll(filepath.Dir(opts.OutputFile), 0o755)
			_ = os.WriteFile(opts.OutputFile, []byte(result.Output), 0o644)
		}
		ch <- result
	}()

	return &agent.ForkSession{
		Result:     ch,
		OutputFile: opts.OutputFile,
	}, nil
}

// --- ForkManager ---

// ForkSpec describes a single fork to launch.
type ForkSpec struct {
	ID         string
	Prompt     string
	OutputFile string
}

// ForkManager manages parallel fork lifecycle.
// This is the interface under test — implementation comes after tests pass.
type ForkManager struct {
	backend agent.Backend
	mu      sync.Mutex
	active  map[string]*managedFork
	results map[string]agent.ForkResult
}

type managedFork struct {
	spec   ForkSpec
	fork   *agent.ForkSession
	done   chan struct{}
	result agent.ForkResult
}

// NewForkManager creates a ForkManager with the given backend.
func NewForkManager(backend agent.Backend) *ForkManager {
	return &ForkManager{
		backend: backend,
		active:  make(map[string]*managedFork),
		results: make(map[string]agent.ForkResult),
	}
}

// StartFork launches a fork asynchronously and returns immediately.
func (fm *ForkManager) StartFork(ctx context.Context, spec ForkSpec, opts agent.ForkOptions) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.active[spec.ID]; exists {
		return fmt.Errorf("fork %q already active", spec.ID)
	}

	opts.OutputFile = spec.OutputFile
	fs, err := fm.backend.Fork(ctx, spec.Prompt, opts)
	if err != nil {
		return err
	}

	mf := &managedFork{
		spec: spec,
		fork: fs,
		done: make(chan struct{}),
	}
	fm.active[spec.ID] = mf

	// Background goroutine collects result.
	go func() {
		fr := <-fs.Result
		mf.result = fr
		close(mf.done)

		fm.mu.Lock()
		delete(fm.active, spec.ID)
		fm.results[spec.ID] = fr
		fm.mu.Unlock()
	}()

	return nil
}

// WaitFork blocks until the specified fork completes.
func (fm *ForkManager) WaitFork(id string) (agent.ForkResult, error) {
	fm.mu.Lock()
	mf, ok := fm.active[id]
	fm.mu.Unlock()
	if !ok {
		// Check completed results.
		fm.mu.Lock()
		fr, done := fm.results[id]
		fm.mu.Unlock()
		if done {
			return fr, nil
		}
		return agent.ForkResult{}, fmt.Errorf("fork %q not found", id)
	}
	<-mf.done
	return mf.result, nil
}

// WaitAll blocks until all active forks complete and returns their results.
func (fm *ForkManager) WaitAll() map[string]agent.ForkResult {
	fm.mu.Lock()
	snap := make(map[string]*managedFork, len(fm.active))
	for k, v := range fm.active {
		snap[k] = v
	}
	fm.mu.Unlock()

	for _, mf := range snap {
		<-mf.done
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()
	out := make(map[string]agent.ForkResult, len(fm.results))
	for k, v := range fm.results {
		out[k] = v
	}
	return out
}

// ActiveCount returns the number of currently running forks.
func (fm *ForkManager) ActiveCount() int {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return len(fm.active)
}

// --- Tests ---

func TestForkManager_StartSingleFork(t *testing.T) {
	backend := &mockBackend{
		results: map[string]agent.ForkResult{
			"/tmp/fork1_output.txt": {Status: "completed", Output: "result 1"},
		},
	}
	fm := NewForkManager(backend)

	spec := ForkSpec{ID: "fork-1", Prompt: "edit foo.go", OutputFile: "/tmp/fork1_output.txt"}
	err := fm.StartFork(context.Background(), spec, agent.ForkOptions{
		Cwd: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("StartFork: %v", err)
	}

	fr, err := fm.WaitFork("fork-1")
	if err != nil {
		t.Fatalf("WaitFork: %v", err)
	}
	if fr.Status != "completed" {
		t.Errorf("expected completed, got %s", fr.Status)
	}
	if fr.Output != "result 1" {
		t.Errorf("expected 'result 1', got %q", fr.Output)
	}
}

func TestForkManager_ParallelForks(t *testing.T) {
	backend := &mockBackend{
		results: map[string]agent.ForkResult{
			"/tmp/fork_a.txt": {Status: "completed", Output: "A done"},
			"/tmp/fork_b.txt": {Status: "completed", Output: "B done"},
			"/tmp/fork_c.txt": {Status: "completed", Output: "C done"},
		},
	}
	fm := NewForkManager(backend)
	tempDir := t.TempDir()

	specs := []ForkSpec{
		{ID: "a", Prompt: "task A", OutputFile: "/tmp/fork_a.txt"},
		{ID: "b", Prompt: "task B", OutputFile: "/tmp/fork_b.txt"},
		{ID: "c", Prompt: "task C", OutputFile: "/tmp/fork_c.txt"},
	}

	for _, spec := range specs {
		if err := fm.StartFork(context.Background(), spec, agent.ForkOptions{Cwd: tempDir}); err != nil {
			t.Fatalf("StartFork %s: %v", spec.ID, err)
		}
	}

	// All 3 should be active.
	if fm.ActiveCount() != 3 {
		t.Errorf("expected 3 active forks, got %d", fm.ActiveCount())
	}

	results := fm.WaitAll()

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, id := range []string{"a", "b", "c"} {
		fr, ok := results[id]
		if !ok {
			t.Errorf("missing result for fork %s", id)
			continue
		}
		if fr.Status != "completed" {
			t.Errorf("fork %s: expected completed, got %s", id, fr.Status)
		}
	}

	// All forks should have completed.
	if fm.ActiveCount() != 0 {
		t.Errorf("expected 0 active forks after WaitAll, got %d", fm.ActiveCount())
	}
}

func TestForkManager_ForkFailure(t *testing.T) {
	backend := &mockBackend{
		results: map[string]agent.ForkResult{
			"/tmp/good.txt": {Status: "completed", Output: "ok"},
			"/tmp/bad.txt":  {Status: "failed", Error: "timeout exceeded"},
		},
	}
	fm := NewForkManager(backend)
	tempDir := t.TempDir()

	// Start a good and a bad fork.
	if err := fm.StartFork(context.Background(),
		ForkSpec{ID: "good", Prompt: "succeed", OutputFile: "/tmp/good.txt"},
		agent.ForkOptions{Cwd: tempDir},
	); err != nil {
		t.Fatal(err)
	}
	if err := fm.StartFork(context.Background(),
		ForkSpec{ID: "bad", Prompt: "fail", OutputFile: "/tmp/bad.txt"},
		agent.ForkOptions{Cwd: tempDir},
	); err != nil {
		t.Fatal(err)
	}

	results := fm.WaitAll()

	if results["good"].Status != "completed" {
		t.Errorf("good fork should be completed, got %s", results["good"].Status)
	}
	if results["bad"].Status != "failed" {
		t.Errorf("bad fork should be failed, got %s", results["bad"].Status)
	}
	if results["bad"].Error != "timeout exceeded" {
		t.Errorf("expected error 'timeout exceeded', got %q", results["bad"].Error)
	}
}

func TestForkManager_DuplicateID(t *testing.T) {
	backend := &mockBackend{
		results: map[string]agent.ForkResult{
			"/tmp/dup1.txt": {Status: "completed"},
			"/tmp/dup2.txt": {Status: "completed"},
		},
	}
	fm := NewForkManager(backend)
	tempDir := t.TempDir()

	opts := agent.ForkOptions{Cwd: tempDir}
	err := fm.StartFork(context.Background(),
		ForkSpec{ID: "dup", Prompt: "first", OutputFile: "/tmp/dup1.txt"}, opts)
	if err != nil {
		t.Fatal(err)
	}

	// Second fork with same ID should fail.
	err = fm.StartFork(context.Background(),
		ForkSpec{ID: "dup", Prompt: "second", OutputFile: "/tmp/dup2.txt"}, opts)
	if err == nil {
		t.Error("expected error for duplicate fork ID, got nil")
	}

	fm.WaitAll()
}

func TestForkManager_Cleanup(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, ".multicode", "fork_result.txt")

	backend := &mockBackend{
		results: map[string]agent.ForkResult{
			outputFile: {Status: "completed", Output: "done"},
		},
	}
	fm := NewForkManager(backend)

	// Create the output dir as the real fork would.
	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
		t.Fatal(err)
	}

	spec := ForkSpec{ID: "cleanup-test", Prompt: "test", OutputFile: outputFile}
	if err := fm.StartFork(context.Background(), spec, agent.ForkOptions{Cwd: tempDir}); err != nil {
		t.Fatal(err)
	}

	fr, err := fm.WaitFork("cleanup-test")
	if err != nil {
		t.Fatal(err)
	}
	if fr.Status != "completed" {
		t.Errorf("expected completed, got %s", fr.Status)
	}

	// Verify the output file exists (cleanup of fork's temp workdir is
	// the caller's responsibility, but we verify the output was produced).
	if _, statErr := os.Stat(outputFile); statErr != nil {
		t.Errorf("output file should exist after fork completes: %v", statErr)
	}
}
