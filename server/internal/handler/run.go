package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/service"
	"github.com/multica-ai/alphenix/server/pkg/agent"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// CreateRunRequest is the HTTP request body for creating a run.
type CreateRunRequest struct {
	IssueID        string `json:"issue_id"`
	AgentID        string `json:"agent_id"`
	TaskID         string `json:"task_id,omitempty"`
	ParentRunID    string `json:"parent_run_id,omitempty"`
	TeamID         string `json:"team_id,omitempty"`
	SystemPrompt   string `json:"system_prompt"`
	ModelName      string `json:"model_name"`
	PermissionMode string `json:"permission_mode"`
}

// CreateRun creates a new agent run.
func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req CreateRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.RunOrchestrator.CreateRun(r.Context(), service.CreateRunRequest{
		WorkspaceID:    workspaceID,
		IssueID:        req.IssueID,
		AgentID:        req.AgentID,
		TaskID:         req.TaskID,
		ParentRunID:    req.ParentRunID,
		TeamID:         req.TeamID,
		SystemPrompt:   req.SystemPrompt,
		ModelName:      req.ModelName,
		PermissionMode: req.PermissionMode,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, run)
}

// GetRun returns a single run by ID.
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	run, err := h.RunOrchestrator.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	if uuidToString(run.WorkspaceID) != resolveWorkspaceID(r) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// ListRuns returns runs for the current workspace with pagination.
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	limit := int32(50)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = int32(v)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	runs, err := h.RunOrchestrator.ListRuns(r.Context(), workspaceID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, runs)
}

// ListRunsByIssue returns all runs for a given issue.
func (h *Handler) ListRunsByIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "issueId")
	runs, err := h.RunOrchestrator.ListRunsByIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, runs)
}

// StartRun transitions a run from pending to executing.
func (h *Handler) StartRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	run, err := h.RunOrchestrator.StartRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// CancelRun marks a run as cancelled.
func (h *Handler) CancelRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	run, err := h.RunOrchestrator.CancelRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// RetryRun creates a new run based on an existing run (parent_run_id = original).
func (h *Handler) RetryRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	// Validate the run exists and belongs to the workspace.
	run, err := h.RunOrchestrator.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if uuidToString(run.WorkspaceID) != resolveWorkspaceID(r) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	newRun, err := h.RunOrchestrator.RetryRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, newRun)
}

// CompleteRun marks a run as completed.
func (h *Handler) CompleteRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	run, err := h.RunOrchestrator.CompleteRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// GetRunSteps returns all steps for a run.
func (h *Handler) GetRunSteps(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	steps, err := h.RunOrchestrator.GetRunSteps(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, steps)
}

// GetRunTodos returns all todos for a run.
func (h *Handler) GetRunTodos(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	todos, err := h.RunOrchestrator.GetRunTodos(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, todos)
}

// GetRunArtifacts returns all artifacts for a run.
func (h *Handler) GetRunArtifacts(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	artifacts, err := h.RunOrchestrator.GetRunArtifacts(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, artifacts)
}

// RecordStepRequest is the HTTP request body for recording a step.
type RecordStepRequest struct {
	StepType   string `json:"step_type"`
	ToolName   string `json:"tool_name"`
	CallID     string `json:"call_id,omitempty"`
	ToolInput  []byte `json:"tool_input"`
	ToolOutput string `json:"tool_output,omitempty"`
	IsError    bool   `json:"is_error"`
}

// RecordStep records a step within a run.
func (h *Handler) RecordStep(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	var req RecordStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.StepType == "" {
		req.StepType = "tool_use"
	}

	step, err := h.RunOrchestrator.RecordStep(r.Context(), runID, req.StepType, req.ToolName, req.CallID, req.ToolInput, req.ToolOutput, req.IsError)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, step)
}

// CreateTodoRequest is the HTTP request body for creating a todo.
type CreateTodoRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// CreateRunTodo creates a new todo for a run.
func (h *Handler) CreateRunTodo(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	var req CreateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	todo, err := h.RunOrchestrator.CreateTodo(r.Context(), runID, req.Title, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, todo)
}

// UpdateTodoRequest is the HTTP request body for updating a todo.
type UpdateTodoRequest struct {
	Status  string `json:"status"`
	Blocker string `json:"blocker,omitempty"`
}

// UpdateRunTodo updates a todo's status.
func (h *Handler) UpdateRunTodo(w http.ResponseWriter, r *http.Request) {
	todoID := chi.URLParam(r, "todoId")

	var req UpdateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	todo, err := h.RunOrchestrator.UpdateTodo(r.Context(), todoID, req.Status, req.Blocker)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, todo)
}

// ExecuteRunRequest is the HTTP request body for executing a run autonomously.
type ExecuteRunRequest struct {
	Provider       string `json:"provider"`        // agent type: "claude", "codex", "opencode"
	ExecutablePath string `json:"executable_path"` // path to agent CLI binary
	Cwd            string `json:"cwd"`             // working directory
	Model          string `json:"model,omitempty"`
	SystemPrompt   string `json:"system_prompt"`
	Prompt         string `json:"prompt"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	MaxTurns       int    `json:"max_turns,omitempty"`
}

// ExecuteRun starts autonomous execution of a run via an agent backend.
func (h *Handler) ExecuteRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	var req ExecuteRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate the run exists and belongs to the workspace.
	run, err := h.RunOrchestrator.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if uuidToString(run.WorkspaceID) != resolveWorkspaceID(r) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	// Create agent backend.
	backend, err := agent.New(req.Provider, agent.Config{
		ExecutablePath: req.ExecutablePath,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent provider: "+err.Error())
		return
	}

	timeout := 10 * time.Minute
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	result, err := h.RunOrchestrator.ExecuteRun(r.Context(), service.ExecuteRunRequest{
		RunID:        runID,
		Cwd:          req.Cwd,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Prompt:       req.Prompt,
		Backend:      backend,
		Timeout:      timeout,
		MaxTurns:     req.MaxTurns,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListRunEvents returns run events for replay / reconnection catchup.
// GET /api/runs/{runId}/events?after=<seq>&limit=<n>
// - after: cursor seq (exclusive). Events with seq > after are returned. Default 0.
// - limit: max events to return. Default 100, max 500.
func (h *Handler) ListRunEvents(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	ctx := r.Context()

	// Verify the run exists and belongs to the caller's workspace.
	run, err := h.Queries.GetRun(ctx, parseUUID(runID))
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if uuidToString(run.WorkspaceID) != resolveWorkspaceID(r) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	// Parse cursor params.
	var afterSeq int64
	if s := r.URL.Query().Get("after"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			afterSeq = n
		}
	}

	limit := int32(100)
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 32); err == nil && n > 0 && n <= 500 {
			limit = int32(n)
		}
	}

	var events []map[string]any
	if afterSeq > 0 {
		rows, err := h.Queries.ListRunEvents(ctx, db.ListRunEventsParams{
			RunID: parseUUID(runID),
			Seq:   pgtype.Int8{Int64: afterSeq, Valid: true},
			Limit: limit,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list events")
			return
		}
		events = make([]map[string]any, 0, len(rows))
		for _, ev := range rows {
			events = append(events, runEventToMap(ev))
		}
	} else {
		rows, err := h.Queries.ListRunEventsAll(ctx, db.ListRunEventsAllParams{
			RunID: parseUUID(runID),
			Limit: limit,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list events")
			return
		}
		events = make([]map[string]any, 0, len(rows))
		for _, ev := range rows {
			events = append(events, runEventToMap(ev))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
	})
}

// runEventToMap converts a RunEvent DB row to the API response shape.
func runEventToMap(ev db.RunEvent) map[string]any {
	m := map[string]any{
		"id":         uuidToString(ev.ID),
		"run_id":     uuidToString(ev.RunID),
		"seq":        ev.Seq,
		"event_type": ev.EventType,
		"created_at": ev.CreatedAt.Time.Format(time.RFC3339Nano),
	}
	// Unmarshal payload JSONB into the map directly so the client gets the
	// same shape as the WS broadcast payload.
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err == nil {
		m["payload"] = payload
	} else {
		m["payload"] = map[string]any{}
	}
	return m
}
