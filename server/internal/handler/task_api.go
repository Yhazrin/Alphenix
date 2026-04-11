package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// GetTask returns a single agent task by ID.
// GET /api/tasks/{taskId}
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if _, ok := h.workspaceMember(w, r, uuidToString(issue.WorkspaceID)); !ok {
		return
	}
	resp := taskToResponse(task)
	resp.WorkspaceID = uuidToString(issue.WorkspaceID)
	writeJSON(w, http.StatusOK, resp)
}

// RetryTask resets an in_review task to queued for another execution run.
// POST /api/tasks/{taskId}/retry
func (h *Handler) RetryTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(issue.WorkspaceID), "workspace not found", "owner", "admin", "member"); !ok {
		return
	}
	retried, err := h.ReviewService.RetryTask(r.Context(), task.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := taskToResponse(*retried)
	resp.WorkspaceID = uuidToString(issue.WorkspaceID)
	writeJSON(w, http.StatusOK, resp)
}

type chainTaskRequest struct {
	TargetAgentID string `json:"target_agent_id"`
	Reason        string `json:"reason"`
}

// ChainTask creates a queued task for another agent, linked to the source task.
// POST /api/tasks/{taskId}/chain
func (h *Handler) ChainTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	src, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), src.IssueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	wsID := uuidToString(issue.WorkspaceID)
	if _, ok := h.workspaceMember(w, r, wsID); !ok {
		return
	}
	var req chainTaskRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TargetAgentID) == "" {
		writeError(w, http.StatusBadRequest, "target_agent_id is required")
		return
	}
	targetAgent, err := h.Queries.GetAgent(r.Context(), parseUUID(req.TargetAgentID))
	if err != nil || uuidToString(targetAgent.WorkspaceID) != wsID {
		writeError(w, http.StatusBadRequest, "invalid target agent")
		return
	}
	if !targetAgent.RuntimeID.Valid {
		writeError(w, http.StatusBadRequest, "target agent has no runtime")
		return
	}
	chainReason := pgtype.Text{String: req.Reason, Valid: strings.TrimSpace(req.Reason) != ""}
	newTask, err := h.Queries.CreateAgentTask(r.Context(), db.CreateAgentTaskParams{
		AgentID:           targetAgent.ID,
		RuntimeID:         targetAgent.RuntimeID,
		IssueID:           src.IssueID,
		Priority:          src.Priority,
		TriggerCommentID:  pgtype.UUID{Valid: false},
		ChainSourceTaskID: src.ID,
		ChainReason:       chainReason,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chained task")
		return
	}
	h.publish(protocol.EventTaskChained, wsID, "system", "", map[string]any{
		"task_id":         uuidToString(newTask.ID),
		"source_task_id":  uuidToString(src.ID),
		"target_agent_id": uuidToString(targetAgent.ID),
		"issue_id":        uuidToString(src.IssueID),
	})
	resp := taskToResponse(newTask)
	resp.WorkspaceID = wsID
	writeJSON(w, http.StatusCreated, resp)
}

// SearchIssues returns issues whose title contains the query string (open issues only, capped).
// GET /api/issues/search?q=...
func (h *Handler) SearchIssues(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	wsUUID := parseUUID(workspaceID)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"issues": []IssueResponse{}, "total": 0})
		return
	}
	issues, err := h.Queries.ListOpenIssues(r.Context(), db.ListOpenIssuesParams{
		WorkspaceID: wsUUID,
		Priority:    pgtype.Text{},
		AssigneeID:  pgtype.UUID{},
		ChannelID:   pgtype.UUID{},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search issues")
		return
	}
	prefix := h.getIssuePrefix(r.Context(), wsUUID)
	const maxResults = 50
	matched := make([]IssueResponse, 0, maxResults)
	for _, issue := range issues {
		if strings.Contains(strings.ToLower(issue.Title), q) {
			matched = append(matched, issueToResponse(issue, prefix))
			if len(matched) >= maxResults {
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issues": matched,
		"total":  len(matched),
	})
}
