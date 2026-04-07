package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/alphenix/server/internal/service"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// ---------------------------------------------------------------------------
// Fork Lifecycle (called by daemon when ForkManager callbacks fire)
// ---------------------------------------------------------------------------

// DaemonForkStarted creates a child run when a sub-agent fork begins.
// POST /api/daemon/tasks/{taskId}/forks/{forkId}/start
func (h *Handler) DaemonForkStarted(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	forkID := chi.URLParam(r, "forkId")

	var req struct {
		Prompt string `json:"prompt,omitempty"`
		Role   string `json:"role,omitempty"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body is optional for fork started — proceed with empty fields.
		req = struct {
			Prompt string `json:"prompt,omitempty"`
			Role   string `json:"role,omitempty"`
		}{}
	}

	// Look up the parent run by task_id.
	parentRun, err := h.Queries.GetRunByTask(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Warn("fork started: no run for task", "task_id", taskID, "fork_id", forkID, "error", err)
		writeError(w, http.StatusNotFound, "parent run not found")
		return
	}

	workspaceID := uuidToString(parentRun.WorkspaceID)

	// Create a child run with parent_run_id set to the parent run.
	// Use forkID as the task_id on the child run so we can find it later.
	childRun, err := h.RunOrchestrator.CreateRun(r.Context(), service.CreateRunRequest{
		WorkspaceID:    workspaceID,
		IssueID:        uuidToString(parentRun.IssueID),
		AgentID:        uuidToString(parentRun.AgentID),
		TaskID:         forkID,
		ParentRunID:    uuidToString(parentRun.ID),
		SystemPrompt:   "", // sub-agent inherits from parent
		ModelName:      parentRun.ModelName,
		PermissionMode: parentRun.PermissionMode,
	})
	if err != nil {
		slog.Error("fork started: failed to create child run", "task_id", taskID, "fork_id", forkID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create child run")
		return
	}

	// Immediately start the child run (it's already executing).
	if _, err := h.RunOrchestrator.StartRun(r.Context(), uuidToString(childRun.ID)); err != nil {
		slog.Error("fork started: failed to start child run", "run_id", uuidToString(childRun.ID), "error", err)
		// Non-fatal — the run exists but is in pending phase.
	}

	slog.Info("fork started", "task_id", taskID, "fork_id", forkID, "parent_run_id", uuidToString(parentRun.ID), "child_run_id", uuidToString(childRun.ID))

	// Broadcast fork started event so frontend can show sub-agent activity.
	h.RunOrchestrator.BroadcastRunEvent(r.Context(), uuidToString(parentRun.ID), workspaceID, protocol.EventForkStarted, map[string]any{
		"fork_id":        forkID,
		"parent_task_id": taskID,
		"parent_run_id":  uuidToString(parentRun.ID),
		"child_run_id":   uuidToString(childRun.ID),
		"role":           req.Role,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"child_run_id": uuidToString(childRun.ID),
	})
}

// DaemonForkCompleted marks a child run as completed when a sub-agent fork finishes.
// POST /api/daemon/tasks/{taskId}/forks/{forkId}/complete
func (h *Handler) DaemonForkCompleted(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	forkID := chi.URLParam(r, "forkId")

	var req struct {
		Output     string `json:"output,omitempty"`
		DurationMs int64  `json:"duration_ms,omitempty"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = struct {
			Output     string `json:"output,omitempty"`
			DurationMs int64  `json:"duration_ms,omitempty"`
		}{}
	}

	// Look up the parent run to get workspace context.
	parentRun, err := h.Queries.GetRunByTask(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Warn("fork completed: no run for task", "task_id", taskID, "fork_id", forkID, "error", err)
		writeError(w, http.StatusNotFound, "parent run not found")
		return
	}

	workspaceID := uuidToString(parentRun.WorkspaceID)

	// Find the child run by forkID (stored as task_id on child run).
	childRun, err := h.Queries.GetRunByTask(r.Context(), parseUUID(forkID))
	if err != nil {
		slog.Warn("fork completed: no child run for fork", "fork_id", forkID, "error", err)
		// Still broadcast the event even if we can't update the run.
		h.RunOrchestrator.BroadcastRunEvent(r.Context(), uuidToString(parentRun.ID), workspaceID, protocol.EventForkCompleted, map[string]any{
			"fork_id":        forkID,
			"parent_task_id": taskID,
			"parent_run_id":  uuidToString(parentRun.ID),
			"duration_ms":    req.DurationMs,
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	// Mark the child run as completed.
	if _, err := h.RunOrchestrator.CompleteRun(r.Context(), uuidToString(childRun.ID)); err != nil {
		slog.Error("fork completed: failed to complete child run", "run_id", uuidToString(childRun.ID), "error", err)
	}

	slog.Info("fork completed", "task_id", taskID, "fork_id", forkID, "child_run_id", uuidToString(childRun.ID), "duration_ms", req.DurationMs)

	h.RunOrchestrator.BroadcastRunEvent(r.Context(), uuidToString(parentRun.ID), workspaceID, protocol.EventForkCompleted, map[string]any{
		"fork_id":        forkID,
		"parent_task_id": taskID,
		"parent_run_id":  uuidToString(parentRun.ID),
		"child_run_id":   uuidToString(childRun.ID),
		"duration_ms":    req.DurationMs,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DaemonForkFailed marks a child run as failed when a sub-agent fork errors.
// POST /api/daemon/tasks/{taskId}/forks/{forkId}/fail
func (h *Handler) DaemonForkFailed(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	forkID := chi.URLParam(r, "forkId")

	var req struct {
		Error string `json:"error,omitempty"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = struct {
			Error string `json:"error,omitempty"`
		}{}
	}

	// Look up the parent run to get workspace context.
	parentRun, err := h.Queries.GetRunByTask(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Warn("fork failed: no run for task", "task_id", taskID, "fork_id", forkID, "error", err)
		writeError(w, http.StatusNotFound, "parent run not found")
		return
	}

	workspaceID := uuidToString(parentRun.WorkspaceID)

	// Find the child run by forkID.
	childRun, err := h.Queries.GetRunByTask(r.Context(), parseUUID(forkID))
	if err != nil {
		slog.Warn("fork failed: no child run for fork", "fork_id", forkID, "error", err)
		h.RunOrchestrator.BroadcastRunEvent(r.Context(), uuidToString(parentRun.ID), workspaceID, protocol.EventForkFailed, map[string]any{
			"fork_id":        forkID,
			"parent_task_id": taskID,
			"parent_run_id":  uuidToString(parentRun.ID),
			"error":          req.Error,
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	// Mark the child run as failed.
	if _, err := h.RunOrchestrator.FailRun(r.Context(), uuidToString(childRun.ID), req.Error); err != nil {
		slog.Error("fork failed: failed to fail child run", "run_id", uuidToString(childRun.ID), "error", err)
	}

	slog.Info("fork failed", "task_id", taskID, "fork_id", forkID, "child_run_id", uuidToString(childRun.ID), "fork_error", req.Error)

	h.RunOrchestrator.BroadcastRunEvent(r.Context(), uuidToString(parentRun.ID), workspaceID, protocol.EventForkFailed, map[string]any{
		"fork_id":        forkID,
		"parent_task_id": taskID,
		"parent_run_id":  uuidToString(parentRun.ID),
		"child_run_id":   uuidToString(childRun.ID),
		"error":          req.Error,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
