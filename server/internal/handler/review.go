package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type submitReviewRequest struct {
	Verdict  string `json:"verdict"`
	Feedback string `json:"feedback"`
}

// SubmitReview handles manual review of a task in in_review status.
// POST /api/tasks/:taskId/review
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	var req submitReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Verdict != "pass" && req.Verdict != "fail" && req.Verdict != "retry" {
		writeError(w, http.StatusBadRequest, "verdict must be pass, fail, or retry")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Load the task to find its workspace, then verify the user is an owner/admin.
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	workspaceID := uuidToString(issue.WorkspaceID)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin")
	if !ok {
		return
	}

	updatedTask, err := h.ReviewService.SubmitManualReview(r.Context(), parseUUID(taskID), member.UserID, req.Verdict, req.Feedback)
	if err != nil {
		slog.Warn("manual review failed", "task_id", taskID, "user_id", userID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("manual review submitted", "task_id", taskID, "user_id", userID, "verdict", req.Verdict)
	writeJSON(w, http.StatusOK, taskToResponse(*updatedTask))
}
