package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// TaskReportResponse is the JSON representation of a task execution report.
type TaskReportResponse struct {
	ID              string  `json:"id"`
	AgentID         string  `json:"agent_id"`
	AgentName       string  `json:"agent_name"`
	IssueID         string  `json:"issue_id"`
	IssueTitle      string  `json:"issue_title"`
	RuntimeID       string  `json:"runtime_id,omitempty"`
	RuntimeName     *string `json:"runtime_name,omitempty"`
	Status          string  `json:"status"`
	Priority        int32   `json:"priority"`
	DispatchedAt    *string `json:"dispatched_at,omitempty"`
	StartedAt       *string `json:"started_at,omitempty"`
	CompletedAt     *string `json:"completed_at,omitempty"`
	Result          []byte  `json:"result,omitempty"`
	Error           *string `json:"error,omitempty"`
	CreatedAt       string  `json:"created_at"`
	ReviewStatus    string  `json:"review_status"`
	MessageCount    int64   `json:"message_count"`
	CheckpointCount int64   `json:"checkpoint_count"`
}

// TaskTimelineEventResponse is a single entry in the task timeline.
type TaskTimelineEventResponse struct {
	EventType string  `json:"event_type"`
	ID        string  `json:"id"`
	Timestamp string  `json:"timestamp"`
	Title     string  `json:"title"`
	Detail    string  `json:"detail"`
	Meta      []byte  `json:"meta,omitempty"`
}

// GetTaskReport returns the execution report for a task.
// GET /api/tasks/{taskId}/report
func (h *Handler) GetTaskReport(w http.ResponseWriter, r *http.Request) {
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

	report, err := h.Queries.GetTaskReport(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Error("get task report failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load task report")
		return
	}

	resp := TaskReportResponse{
		ID:              uuidToString(report.ID),
		AgentID:         uuidToString(report.AgentID),
		AgentName:       report.AgentName,
		IssueID:         uuidToString(report.IssueID),
		IssueTitle:      report.IssueTitle,
		Status:          report.Status,
		Priority:        report.Priority,
		CreatedAt:       timestampToString(report.CreatedAt),
		ReviewStatus:    report.ReviewStatus,
		MessageCount:    report.MessageCount,
		CheckpointCount: report.CheckpointCount,
	}

	if report.RuntimeID.Valid {
		resp.RuntimeID = uuidToString(report.RuntimeID)
	}
	if report.RuntimeName.Valid {
		resp.RuntimeName = &report.RuntimeName.String
	}
	if report.DispatchedAt.Valid {
		s := timestampToString(report.DispatchedAt)
		resp.DispatchedAt = &s
	}
	if report.StartedAt.Valid {
		s := timestampToString(report.StartedAt)
		resp.StartedAt = &s
	}
	if report.CompletedAt.Valid {
		s := timestampToString(report.CompletedAt)
		resp.CompletedAt = &s
	}
	if report.Result != nil {
		resp.Result = report.Result
	}
	if report.Error.Valid {
		resp.Error = &report.Error.String
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetTaskTimeline returns the full timeline of events for a task.
// GET /api/tasks/{taskId}/timeline
func (h *Handler) GetTaskTimeline(w http.ResponseWriter, r *http.Request) {
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

	rows, err := h.Queries.GetTaskTimelineMessages(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Error("get task timeline failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load task timeline")
		return
	}

	events := make([]TaskTimelineEventResponse, len(rows))
	for i, row := range rows {
		events[i] = TaskTimelineEventResponse{
			EventType: row.EventType,
			ID:        row.TmID,
			Timestamp: timestampToString(row.Timestamp),
			Title:     row.Title,
			Detail:    row.Detail,
			Meta:      row.Meta,
		}
	}

	writeJSON(w, http.StatusOK, events)
}
