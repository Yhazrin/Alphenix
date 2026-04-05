package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// --- Task Assignment Protocol ---

// TaskSpec describes a subtask to be assigned to a worker (sub-agent).
type TaskSpec struct {
	ID          string        // unique task identifier
	Description string        // human-readable task description
	Context     string        // additional context for the worker (code snippets, file paths, etc.)
	Timeout     time.Duration // per-task timeout (0 = use orchestrator default)
}

// WorkerContext is the structured input passed to a fork for execution.
// Built by BuildWorkerContext from a TaskSpec + orchestrator config.
type WorkerContext struct {
	Prompt    string        // directive-style prompt for the sub-agent
	TaskID    string        // task identifier for tracking
	Timeout   time.Duration // effective timeout
	OutputDir string        // directory for fork output file
}

// BuildWorkerContext converts a TaskSpec into a WorkerContext ready for ForkManager.
func BuildWorkerContext(spec TaskSpec, defaultTimeout time.Duration) WorkerContext {
	timeout := spec.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	var b strings.Builder
	b.WriteString("You are a sub-agent worker. Complete this specific task:\n\n")
	if spec.Context != "" {
		b.WriteString("Context:\n")
		b.WriteString(spec.Context)
		b.WriteString("\n\n")
	}
	b.WriteString(spec.Description)
	b.WriteString("\n\nReport your result clearly. Use DONE/ERROR markers for structured output.")

	return WorkerContext{
		Prompt:  b.String(),
		TaskID:  spec.ID,
		Timeout: timeout,
	}
}

// --- Worker State Machine ---

// WorkerState represents the lifecycle state of a worker.
type WorkerState string

const (
	WorkerStatePending   WorkerState = "pending"
	WorkerStateRunning   WorkerState = "running"
	WorkerStateCompleted WorkerState = "completed"
	WorkerStateFailed    WorkerState = "failed"
	WorkerStateTimeout   WorkerState = "timeout"
	WorkerStateRetrying  WorkerState = "retrying"
)

// WorkerStatus tracks the current state and metadata of a single worker.
type WorkerStatus struct {
	TaskID     string
	State      WorkerState
	Attempt    int       // current attempt number (1-based)
	MaxRetries int       // maximum retry attempts
	Output     string    // result output (populated on completion)
	Error      string    // error message (populated on failure)
	DurationMs int64     // total execution duration
	StartedAt  time.Time // when the current attempt started
	FinishedAt time.Time // when the worker finished
}

// --- Retry Policy ---

// RetryPolicy controls retry behavior for failed workers.
type RetryPolicy struct {
	MaxRetries    int           // max retry attempts (0 = no retries, default 1)
	BackoffBase   time.Duration // initial backoff delay (default 1s)
	BackoffMax    time.Duration // maximum backoff delay (default 30s)
	BackoffFactor float64       // exponential multiplier (default 2.0)
}

// DefaultRetryPolicy returns the default retry configuration.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:    1,
		BackoffBase:   1 * time.Second,
		BackoffMax:    30 * time.Second,
		BackoffFactor: 2.0,
	}
}

// BackoffDelay returns the delay before the given attempt (1-based).
// Attempt 1 always returns 0 (no delay before first try).
func (rp RetryPolicy) BackoffDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}
	delay := rp.BackoffBase
	if delay == 0 {
		delay = 1 * time.Second
	}
	for i := 2; i < attempt; i++ {
		delay = time.Duration(float64(delay) * rp.BackoffFactor)
		if rp.BackoffMax > 0 && delay > rp.BackoffMax {
			return rp.BackoffMax
		}
	}
	return delay
}

// --- Degradation Strategy ---

// DegradationStrategy controls behavior when some workers fail.
type DegradationStrategy int

const (
	// DegradationContinue collects partial results even if some workers fail.
	DegradationContinue DegradationStrategy = iota
	// DegradationAbort cancels remaining workers when any worker fails.
	DegradationAbort
)

// --- Result Aggregation ---

// WorkerOutput is the final result from a single worker.
type WorkerOutput struct {
	TaskID     string
	Status     WorkerState // completed, failed, timeout
	Output     string
	Error      string
	DurationMs int64
	Attempt    int // which attempt succeeded (or last attempt if failed)
}

// AggregatedResult collects outputs from all workers.
type AggregatedResult struct {
	Results   []WorkerOutput // ordered by completion time
	Succeeded int
	Failed    int
	Total     int
}

// Summary returns a markdown-formatted summary for the coordinator.
func (ar AggregatedResult) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Sub-Agent Results (%d total: %d succeeded, %d failed)\n\n",
		ar.Total, ar.Succeeded, ar.Failed)
	for _, r := range ar.Results {
		fmt.Fprintf(&b, "### %s (status: %s, attempt: %d, %dms)\n\n",
			r.TaskID, r.Status, r.Attempt, r.DurationMs)
		if r.Output != "" {
			b.WriteString(r.Output)
		} else if r.Error != "" {
			fmt.Fprintf(&b, "Error: %s", r.Error)
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

// --- Orchestrator ---

// OrchestratorConfig configures the Orchestrator.
type OrchestratorConfig struct {
	DefaultTimeout  time.Duration
	RetryPolicy     RetryPolicy
	Degradation     DegradationStrategy
	OnWorkerStateChange func(taskID string, oldState, newState WorkerState)
}

// Orchestrator manages parallel sub-agent execution with retry, timeout,
// and result aggregation. It wraps ForkManager and adds:
// - Worker state machine (pending → running → completed/failed/timeout)
// - Retry with exponential backoff
// - Result aggregation by completion order
// - Degradation strategy (continue or abort on partial failure)
type Orchestrator struct {
	backend Backend
	baseDir string
	config  OrchestratorConfig

	mu      sync.Mutex
	workers map[string]*managedWorker
	closed  bool
}

type managedWorker struct {
	mu         sync.Mutex
	spec       TaskSpec
	status     WorkerStatus
	forkOutput chan WorkerOutput
}

// NewOrchestrator creates an Orchestrator for parallel task execution.
func NewOrchestrator(backend Backend, baseDir string, config OrchestratorConfig) *Orchestrator {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 120 * time.Second
	}
	if config.RetryPolicy.MaxRetries == 0 && config.RetryPolicy.BackoffBase == 0 {
		config.RetryPolicy = DefaultRetryPolicy()
	}
	return &Orchestrator{
		backend: backend,
		baseDir: baseDir,
		config:  config,
		workers: make(map[string]*managedWorker),
	}
}

// SubmitTasks launches workers for the given task specs.
func (o *Orchestrator) SubmitTasks(ctx context.Context, tasks []TaskSpec) (int, error) {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return 0, fmt.Errorf("orchestrator is closed")
	}
	o.mu.Unlock()

	queued := 0
	for _, task := range tasks {
		if err := o.submitOne(ctx, task); err != nil {
			return queued, fmt.Errorf("submit task %q: %w", task.ID, err)
		}
		queued++
	}
	return queued, nil
}

func (o *Orchestrator) submitOne(ctx context.Context, spec TaskSpec) error {
	o.mu.Lock()
	if _, exists := o.workers[spec.ID]; exists {
		o.mu.Unlock()
		return fmt.Errorf("task %q already submitted", spec.ID)
	}

	mw := &managedWorker{
		spec: spec,
		status: WorkerStatus{
			TaskID:     spec.ID,
			State:      WorkerStatePending,
			MaxRetries: o.config.RetryPolicy.MaxRetries,
		},
		forkOutput: make(chan WorkerOutput, 1),
	}
	o.workers[spec.ID] = mw
	o.mu.Unlock()

	go o.runWorker(ctx, mw)
	return nil
}

func (o *Orchestrator) runWorker(ctx context.Context, mw *managedWorker) {
	maxAttempts := 1 + o.config.RetryPolicy.MaxRetries

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			delay := o.config.RetryPolicy.BackoffDelay(attempt)
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					o.finishWorker(mw, WorkerStateFailed, "", ctx.Err().Error(), 0, attempt)
					return
				}
			}
			o.transitionState(mw, WorkerStateRetrying)
		}

		mw.mu.Lock()
		mw.status.Attempt = attempt
		mw.status.StartedAt = time.Now()
		mw.mu.Unlock()
		o.transitionState(mw, WorkerStateRunning)

		wctx := BuildWorkerContext(mw.spec, o.config.DefaultTimeout)

		fm := NewForkManager(o.backend, o.baseDir, ForkManagerOptions{
			DefaultTimeout: wctx.Timeout,
		})

		forkID := fmt.Sprintf("%s-attempt-%d", mw.spec.ID, attempt)
		_, startErr := fm.StartFork(ctx, ForkSpec{
			ID:      forkID,
			Prompt:  wctx.Prompt,
			Timeout: wctx.Timeout,
		})

		if startErr != nil {
			fm.Close()
			if attempt == maxAttempts {
				o.finishWorker(mw, WorkerStateFailed, "", startErr.Error(), 0, attempt)
				return
			}
			continue
		}

		output, waitErr := fm.WaitFork(forkID)
		fm.Close()

		if waitErr != nil {
			if attempt == maxAttempts {
				o.finishWorker(mw, WorkerStateFailed, "", waitErr.Error(), 0, attempt)
				return
			}
			continue
		}

		switch output.Status {
		case "completed":
			o.finishWorker(mw, WorkerStateCompleted, output.Output, "", output.DurationMs, attempt)
			return
		case "timeout":
			o.finishWorker(mw, WorkerStateTimeout, output.Output, output.Error, output.DurationMs, attempt)
			return
		default:
			if attempt == maxAttempts {
				o.finishWorker(mw, WorkerStateFailed, output.Output, output.Error, output.DurationMs, attempt)
				return
			}
		}
	}
}

func (o *Orchestrator) finishWorker(mw *managedWorker, state WorkerState, output, errMsg string, durationMs int64, attempt int) {
	o.transitionState(mw, state)

	mw.mu.Lock()
	mw.status.Output = output
	mw.status.Error = errMsg
	mw.status.DurationMs = durationMs
	mw.status.FinishedAt = time.Now()
	mw.mu.Unlock()

	mw.forkOutput <- WorkerOutput{
		TaskID:     mw.spec.ID,
		Status:     state,
		Output:     output,
		Error:      errMsg,
		DurationMs: durationMs,
		Attempt:    attempt,
	}
	close(mw.forkOutput)
}

func (o *Orchestrator) transitionState(mw *managedWorker, newState WorkerState) {
	mw.mu.Lock()
	oldState := mw.status.State
	mw.status.State = newState
	mw.mu.Unlock()

	if o.config.OnWorkerStateChange != nil {
		o.config.OnWorkerStateChange(mw.spec.ID, oldState, newState)
	}
}

// WaitAll blocks until all submitted workers complete and returns aggregated results.
func (o *Orchestrator) WaitAll() AggregatedResult {
	o.mu.Lock()
	workers := make([]*managedWorker, 0, len(o.workers))
	for _, mw := range o.workers {
		workers = append(workers, mw)
	}
	o.mu.Unlock()

	var (
		resultMu sync.Mutex
		results  []WorkerOutput
		wg       sync.WaitGroup
	)

	for _, mw := range workers {
		wg.Add(1)
		go func(m *managedWorker) {
			defer wg.Done()
			output := <-m.forkOutput
			resultMu.Lock()
			results = append(results, output)
			resultMu.Unlock()
		}(mw)
	}

	wg.Wait()

	succeeded, failed := 0, 0
	for _, r := range results {
		if r.Status == WorkerStateCompleted {
			succeeded++
		} else {
			failed++
		}
	}

	return AggregatedResult{
		Results:   results,
		Succeeded: succeeded,
		Failed:    failed,
		Total:     len(results),
	}
}

// WorkerStatus returns the current status of a worker.
func (o *Orchestrator) WorkerStatus(taskID string) (WorkerStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	mw, exists := o.workers[taskID]
	if !exists {
		return WorkerStatus{}, fmt.Errorf("task %q not found", taskID)
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.status, nil
}

// ActiveWorkers returns the count of workers in running or retrying state.
func (o *Orchestrator) ActiveWorkers() int {
	o.mu.Lock()
	defer o.mu.Unlock()

	count := 0
	for _, mw := range o.workers {
		mw.mu.Lock()
		state := mw.status.State
		mw.mu.Unlock()
		if state == WorkerStateRunning || state == WorkerStateRetrying {
			count++
		}
	}
	return count
}

// Close marks the orchestrator as closed. No new tasks can be submitted.
func (o *Orchestrator) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
}
