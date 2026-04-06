package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ForkSpec describes a single fork to launch in a multi-fork batch.
type ForkSpec struct {
	ID        string        // unique identifier for this fork
	Prompt    string        // directive-style prompt for the sub-agent
	Role      AgentRole     // executor, coordinator, reviewer
	Cwd       string        // working directory (if empty, uses ForkManager's base dir)
	Timeout   time.Duration // per-fork timeout
	OutputDir string        // directory for output file (if empty, auto-created in worktree)
}

// ForkOutput collects the result of a single fork after completion.
type ForkOutput struct {
	Spec       ForkSpec
	Status     string // "completed", "failed", "aborted", "timeout"
	Output     string
	Error      string
	DurationMs int64
	Worktree   string // path to the isolated worktree (for cleanup)
}

// ForkManager manages parallel fork execution with worktree isolation.
// It provides StartFork/WaitFork/WaitAll semantics for multi-agent orchestration.
type ForkManager struct {
	backend Backend
	baseDir string // parent working directory
	opts    ForkManagerOptions

	mu     sync.Mutex
	forks  map[string]*managedFork
	closed bool
}

// ForkManagerOptions configures a ForkManager.
type ForkManagerOptions struct {
	// DefaultTimeout is used when a ForkSpec doesn't specify its own timeout.
	DefaultTimeout time.Duration

	// Model overrides the model for all forks (optional).
	Model string

	// ToolPermissions applied to all forks (nil = inherit default).
	ToolPermissions *ToolPermissions

	// ToolHooks for fork-level observability (optional).
	ToolHooks ToolHooks

	// OnForkStarted is called when a fork begins (optional, for event broadcasting).
	OnForkStarted func(forkID string, spec ForkSpec)

	// OnForkCompleted is called when a fork succeeds (optional).
	OnForkCompleted func(forkID string, output ForkOutput)

	// OnForkFailed is called when a fork fails (optional).
	OnForkFailed func(forkID string, output ForkOutput)
}

// managedFork tracks a single running fork.
type managedFork struct {
	spec     ForkSpec
	session  *ForkSession
	worktree string // isolated temp directory
	output   chan ForkOutput
}

// NewForkManager creates a ForkManager for parallel sub-agent execution.
func NewForkManager(backend Backend, baseDir string, opts ForkManagerOptions) *ForkManager {
	return &ForkManager{
		backend: backend,
		baseDir: baseDir,
		opts:    opts,
		forks:   make(map[string]*managedFork),
	}
}

// StartFork launches a new fork in an isolated worktree.
// Returns the fork ID (same as spec.ID) for use with WaitFork.
func (fm *ForkManager) StartFork(ctx context.Context, spec ForkSpec) (string, error) {
	fm.mu.Lock()
	if fm.closed {
		fm.mu.Unlock()
		return "", fmt.Errorf("fork manager is closed")
	}
	if _, exists := fm.forks[spec.ID]; exists {
		fm.mu.Unlock()
		return "", fmt.Errorf("fork %q already started", spec.ID)
	}
	fm.mu.Unlock()

	// Create isolated worktree.
	worktree, err := fm.createWorktree(spec)
	if err != nil {
		return "", fmt.Errorf("create worktree for fork %q: %w", spec.ID, err)
	}

	// Determine output file path.
	outputFile := filepath.Join(worktree, ".alphenix", "fork_output.txt")
	if spec.OutputDir != "" {
		outputFile = filepath.Join(spec.OutputDir, "fork_output.txt")
	}
	if mkErr := os.MkdirAll(filepath.Dir(outputFile), 0o755); mkErr != nil {
		os.RemoveAll(worktree)
		return "", fmt.Errorf("create output dir for fork %q: %w", spec.ID, mkErr)
	}

	// Determine timeout.
	timeout := spec.Timeout
	if timeout == 0 {
		timeout = fm.opts.DefaultTimeout
	}
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	// Build fork options.
	forkOpts := ForkOptions{
		Cwd:             worktree,
		Model:           fm.opts.Model,
		Timeout:         timeout,
		ToolPermissions: fm.opts.ToolPermissions,
		ToolHooks:       fm.opts.ToolHooks,
		OutputFile:      outputFile,
	}

	// Launch fork.
	session, err := fm.backend.Fork(ctx, spec.Prompt, forkOpts)
	if err != nil {
		os.RemoveAll(worktree)
		return "", fmt.Errorf("launch fork %q: %w", spec.ID, err)
	}

	mf := &managedFork{
		spec:     spec,
		session:  session,
		worktree: worktree,
		output:   make(chan ForkOutput, 1),
	}

	fm.mu.Lock()
	fm.forks[spec.ID] = mf
	fm.mu.Unlock()

	// Notify started.
	if fm.opts.OnForkStarted != nil {
		fm.opts.OnForkStarted(spec.ID, spec)
	}

	// Collect result in background.
	go fm.collectResult(spec.ID, mf)

	return spec.ID, nil
}

// WaitFork blocks until the specified fork completes and returns its output.
func (fm *ForkManager) WaitFork(forkID string) (ForkOutput, error) {
	fm.mu.Lock()
	mf, exists := fm.forks[forkID]
	fm.mu.Unlock()

	if !exists {
		return ForkOutput{}, fmt.Errorf("fork %q not found", forkID)
	}

	output := <-mf.output
	return output, nil
}

// WaitAll blocks until all started forks complete and returns their outputs.
// Outputs are returned in the order they complete (not launch order).
func (fm *ForkManager) WaitAll() []ForkOutput {
	fm.mu.Lock()
	forks := make([]*managedFork, 0, len(fm.forks))
	for _, mf := range fm.forks {
		forks = append(forks, mf)
	}
	fm.mu.Unlock()

	results := make([]ForkOutput, 0, len(forks))
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for _, mf := range forks {
		wg.Add(1)
		go func(m *managedFork) {
			defer wg.Done()
			output := <-m.output
			resultsMu.Lock()
			results = append(results, output)
			resultsMu.Unlock()
		}(mf)
	}

	wg.Wait()
	return results
}

// Close cleans up all worktrees and marks the manager as closed.
// No new forks can be started after Close.
func (fm *ForkManager) Close() {
	fm.mu.Lock()
	fm.closed = true
	forks := make(map[string]*managedFork, len(fm.forks))
	for k, v := range fm.forks {
		forks[k] = v
	}
	fm.mu.Unlock()

	for _, mf := range forks {
		if mf.worktree != "" {
			os.RemoveAll(mf.worktree)
		}
	}
}

// ActiveCount returns the number of forks that have been started but not yet completed.
func (fm *ForkManager) ActiveCount() int {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	count := 0
	for _, mf := range fm.forks {
		select {
		case <-mf.output:
			// already completed
		default:
			count++
		}
	}
	return count
}

// createWorktree creates an isolated temporary directory for a fork.
// It copies essential project files (git repo metadata) so the sub-agent
// can operate independently without conflicting with other forks.
func (fm *ForkManager) createWorktree(spec ForkSpec) (string, error) {
	baseDir := fm.baseDir
	if spec.Cwd != "" {
		baseDir = spec.Cwd
	}

	// Create temp directory.
	worktree, err := os.MkdirTemp("", "alphenix-fork-"+spec.ID+"-*")
	if err != nil {
		return "", fmt.Errorf("mkdirtemp: %w", err)
	}

	// Copy .git directory if present — needed for git operations in the fork.
	gitDir := filepath.Join(baseDir, ".git")
	if _, statErr := os.Stat(gitDir); statErr == nil {
		if copyErr := copyDir(gitDir, filepath.Join(worktree, ".git")); copyErr != nil {
			os.RemoveAll(worktree)
			return "", fmt.Errorf("copy .git: %w", copyErr)
		}
	}

	// Create .alphenix output directory.
	if mkErr := os.MkdirAll(filepath.Join(worktree, ".alphenix"), 0o755); mkErr != nil {
		os.RemoveAll(worktree)
		return "", fmt.Errorf("create .alphenix dir: %w", mkErr)
	}

	return worktree, nil
}

// collectResult drains the fork's result channel and sends the output.
func (fm *ForkManager) collectResult(forkID string, mf *managedFork) {
	fr := <-mf.session.Result

	// Read output file if result has no output ("Don't peek" — only after completion).
	output := fr.Output
	if output == "" && mf.session.OutputFile != "" {
		if data, readErr := os.ReadFile(mf.session.OutputFile); readErr == nil {
			output = string(data)
		}
	}

	forkOutput := ForkOutput{
		Spec:       mf.spec,
		Status:     fr.Status,
		Output:     output,
		Error:      fr.Error,
		DurationMs: fr.DurationMs,
		Worktree:   mf.worktree,
	}

	// Notify lifecycle.
	switch fr.Status {
	case "completed":
		if fm.opts.OnForkCompleted != nil {
			fm.opts.OnForkCompleted(forkID, forkOutput)
		}
	default:
		if fm.opts.OnForkFailed != nil {
			fm.opts.OnForkFailed(forkID, forkOutput)
		}
	}

	mf.output <- forkOutput
	close(mf.output)
}

// copyDir recursively copies a directory tree. Best-effort for git metadata.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		// Handle symlinks (common in .git)
		if info.Mode()&os.ModeSymlink != 0 {
			link, linkErr := os.Readlink(path)
			if linkErr != nil {
				return linkErr
			}
			return os.Symlink(link, target)
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
