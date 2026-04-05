package daemon

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// fileModifyingTools is the set of tool names whose results may modify files
// and warrant a post-tool snapshot.
var fileModifyingTools = map[string]bool{
	"write":       true,
	"edit":        true,
	"bash":        true,
	"notebook":    true,
	"todowrite":   false, // in-memory only
	"webfetch":    false,
	"websearch":   false,
	"read":        false,
	"glob":        false,
	"grep":        false,
}

// SnapshotService captures git diffs and persists them as task checkpoints.
type SnapshotService struct {
	client *Client
	sem    chan struct{} // semaphore: cap 3 concurrent git diff goroutines
	logger *slog.Logger
}

// NewSnapshotService creates a new SnapshotService.
func NewSnapshotService(client *Client, logger *slog.Logger) *SnapshotService {
	return &SnapshotService{
		client: client,
		sem:    make(chan struct{}, 3),
		logger: logger,
	}
}

// CaptureTool captures a post-tool-use snapshot asynchronously.
// Only file-modifying tools trigger a capture; others are silently skipped.
func (s *SnapshotService) CaptureTool(ctx context.Context, taskID, workDir, toolName string) {
	if !fileModifyingTools[toolName] {
		return
	}
	// Non-blocking semaphore acquire.
	select {
	case s.sem <- struct{}{}:
	default:
		s.logger.Debug("snapshot: semaphore full, skipping tool snapshot", "task_id", taskID, "tool", toolName)
		return
	}
	go func() {
		defer func() { <-s.sem }()
		s.capture(ctx, taskID, workDir, "tool:"+toolName, false)
	}()
}

// CaptureDone captures a synchronous snapshot at task completion or failure.
// Always creates a checkpoint even if the diff is empty.
func (s *SnapshotService) CaptureDone(ctx context.Context, taskID, workDir, label string) {
	s.capture(ctx, taskID, workDir, label, true)
}

func (s *SnapshotService) capture(ctx context.Context, taskID, workDir, label string, forceEmpty bool) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	files := gitDiff(ctx, workDir)
	if len(files) == 0 && !forceEmpty {
		return
	}

	if err := s.client.SaveCheckpoint(ctx, taskID, label, files); err != nil {
		s.logger.Warn("snapshot: save checkpoint failed", "task_id", taskID, "label", label, "error", err)
	}
}

// gitDiff returns the list of changed files from `git diff --name-only HEAD`.
func gitDiff(ctx context.Context, workDir string) []string {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
