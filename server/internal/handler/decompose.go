package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/logger"
	"github.com/multica-ai/alphenix/server/internal/service"
	"github.com/multica-ai/alphenix/server/pkg/agent"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// SubtaskPreview is a single subtask proposed by the Architect Agent.
type SubtaskPreview struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Deliverable  string   `json:"deliverable"`
	DependsOn    []int    `json:"depends_on"` // indices into the subtasks array
	AssigneeType *string  `json:"assignee_type,omitempty"`
	AssigneeID   *string  `json:"assignee_id,omitempty"`
}

// DecomposePreview is the structured output from the Architect Agent.
type DecomposePreview struct {
	Subtasks    []SubtaskPreview `json:"subtasks"`
	PlanSummary string           `json:"plan_summary"`
	Risks       []string         `json:"risks"`
}

// DecomposeResponse is the response when starting a decomposition.
type DecomposeResponse struct {
	RunID    string `json:"run_id"`
	Status   string `json:"status"` // "running", "completed", "failed"
	Preview  *DecomposePreview `json:"preview,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ConfirmDecomposeRequest is the request body for confirming a decomposition.
type ConfirmDecomposeRequest struct {
	Subtasks []SubtaskPreview `json:"subtasks"`
}

// decomposeSystemPrompt instructs the Architect Agent how to decompose a goal.
const decomposeSystemPrompt = `You are an Architect Agent. Your job is to break down a high-level goal into concrete, executable subtasks.

You MUST respond with a single JSON object (no markdown fences, no extra text) matching this schema:
{
  "subtasks": [
    {
      "title": "short task title",
      "description": "detailed description of what needs to be done",
      "deliverable": "what the task produces",
      "depends_on": [0],         // indices of tasks this depends on (empty array if none)
      "assignee_type": "agent"   // "agent" or null
    }
  ],
  "plan_summary": "one-paragraph summary of the overall plan",
  "risks": ["risk 1", "risk 2"]
}

Rules:
- Each subtask should be independently executable (except for declared dependencies)
- Keep subtasks small — ideally completable in one agent run
- Use dependency ordering to enable parallel execution
- Do NOT include the current task index in depends_on (no self-references)
- If a subtask should be done by an agent, set assignee_type to "agent"
- Respond with ONLY the JSON object, no markdown, no explanation`

// Decompose kicks off an Architect Agent to decompose a goal issue into subtasks.
// POST /api/issues/{id}/decompose
func (h *Handler) Decompose(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, id)
	if !ok {
		return
	}

	if issue.IssueKind != "goal" {
		writeError(w, http.StatusBadRequest, "only goal issues can be decomposed")
		return
	}

	workspaceID := uuidToString(issue.WorkspaceID)

	// Find an agent in the workspace to act as the Architect.
	// For MVP: use the first available agent with a runtime configured.
	agents, err := h.Queries.ListAgents(r.Context(), parseUUID(workspaceID))
	if err != nil || len(agents) == 0 {
		writeError(w, http.StatusBadRequest, "no agents available in workspace for decomposition")
		return
	}

	var architect *db.Agent
	for i := range agents {
		if agents[i].RuntimeID.Valid {
			architect = &agents[i]
			break
		}
	}
	if architect == nil {
		writeError(w, http.StatusBadRequest, "no agent with a configured runtime found in workspace")
		return
	}

	// Look up the runtime to get the provider.
	runtime, err := h.Queries.GetAgentRuntime(r.Context(), architect.RuntimeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve agent runtime")
		return
	}

	// Resolve executable path from runtime metadata or fall back to provider name.
	executablePath := runtime.Provider // default: look up via PATH
	if len(runtime.Metadata) > 0 {
		var meta map[string]any
		if json.Unmarshal(runtime.Metadata, &meta) == nil {
			if ep, ok := meta["executable_path"].(string); ok && ep != "" {
				executablePath = ep
			}
		}
	}

	// Build the decomposition prompt.
	desc := ""
	if issue.Description.Valid {
		desc = issue.Description.String
	}
	prompt := fmt.Sprintf("Decompose this goal into subtasks:\n\nTitle: %s\nDescription: %s",
		issue.Title, desc)

	// Create a run to track the decomposition.
	run, err := h.RunOrchestrator.CreateRun(r.Context(), service.CreateRunRequest{
		WorkspaceID:    workspaceID,
		IssueID:        uuidToString(issue.ID),
		AgentID:        uuidToString(architect.ID),
		SystemPrompt:   decomposeSystemPrompt,
		ModelName:      "claude-sonnet-4-20250514",
		PermissionMode: "bypassPermissions",
	})
	if err != nil {
		slog.Error("decompose: failed to create run", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create decomposition run")
		return
	}

	// Execute asynchronously — return the run ID immediately.
	go func() {
		ctx := r.Context()

		backend, err := agent.New(runtime.Provider, agent.Config{
			ExecutablePath: executablePath,
		})
		if err != nil {
			slog.Error("decompose: failed to create backend", "error", err)
			h.RunOrchestrator.FailRun(ctx, run.ID.String())
			return
		}

		// Use a temp directory as the agent's working directory.
		workDir := filepath.Join(os.TempDir(), "alphenix-decompose", run.ID.String())
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			slog.Error("decompose: failed to create workdir", "error", err)
			h.RunOrchestrator.FailRun(ctx, run.ID.String())
			return
		}

		result, err := h.RunOrchestrator.ExecuteRun(ctx, service.ExecuteRunRequest{
			RunID:        run.ID.String(),
			Cwd:          workDir,
			Model:        "claude-sonnet-4-20250514",
			SystemPrompt: decomposeSystemPrompt,
			Prompt:       prompt,
			Backend:      backend,
			Timeout:      5 * time.Minute,
			MaxTurns:     10,
		})
		if err != nil {
			slog.Error("decompose: execute failed", "error", err, "run_id", run.ID.String())
			return
		}

		// Store the agent output as a run artifact for later retrieval.
		if result.Output != "" {
			if _, err := h.RunOrchestrator.CreateArtifact(ctx, run.ID.String(), "", "decompose_output", "plan.json", result.Output, "application/json"); err != nil {
				slog.Error("decompose: failed to store output artifact", "error", err, "run_id", run.ID.String())
			}
		}

		slog.Info("decompose: completed", "run_id", run.ID.String(), "status", result.Status, "output_len", len(result.Output))
	}()

	writeJSON(w, http.StatusAccepted, DecomposeResponse{
		RunID:  run.ID.String(),
		Status: "running",
	})
}

// GetDecomposeResult returns the current status of a decomposition run.
// GET /api/issues/{id}/decompose/{runId}
func (h *Handler) GetDecomposeResult(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, ok := h.loadIssueForUser(w, r, id)
	if !ok {
		return
	}

	runID := chi.URLParam(r, "runId")
	run, err := h.RunOrchestrator.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	resp := DecomposeResponse{
		RunID:  run.ID.String(),
		Status: run.Status,
	}

	if run.Status == "completed" {
		// Retrieve the decompose output from run artifacts.
		artifacts, err := h.Queries.ListRunArtifactsByType(r.Context(), db.ListRunArtifactsByTypeParams{
			RunID:        parseUUID(runID),
			ArtifactType: "decompose_output",
		})
		if err == nil && len(artifacts) > 0 {
			var preview DecomposePreview
			if err := json.Unmarshal([]byte(artifacts[0].Content), &preview); err == nil {
				resp.Preview = &preview
			} else {
				slog.Warn("decompose: failed to parse agent output as JSON", "run_id", runID, "error", err)
				resp.Status = "failed"
				resp.Error = "agent returned invalid JSON"
			}
		}
	}

	if run.Status == "failed" {
		resp.Error = "decomposition failed"
	}

	writeJSON(w, http.StatusOK, resp)
}

// ConfirmDecompose creates sub-issues from a completed decomposition.
// POST /api/issues/{id}/decompose/confirm
func (h *Handler) ConfirmDecompose(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, id)
	if !ok {
		return
	}

	if issue.IssueKind != "goal" {
		writeError(w, http.StatusBadRequest, "only goal issues can be decomposed")
		return
	}

	var req ConfirmDecomposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Subtasks) == 0 {
		writeError(w, http.StatusBadRequest, "subtasks is required and must not be empty")
		return
	}

	workspaceID := uuidToString(issue.WorkspaceID)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)

	// Use a transaction for atomicity.
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	// Create sub-issues and track their IDs for dependency wiring.
	createdIDs := make([]pgtype.UUID, len(req.Subtasks))
	var createdIssues []IssueResponse

	for i, st := range req.Subtasks {
		issueNumber, err := qtx.IncrementIssueCounter(r.Context(), parseUUID(workspaceID))
		if err != nil {
			slog.Warn("increment issue counter failed", "error", err, "workspace_id", workspaceID)
			writeError(w, http.StatusInternalServerError, "failed to create sub-issue")
			return
		}

		var assigneeType pgtype.Text
		var assigneeID pgtype.UUID
		if st.AssigneeType != nil {
			assigneeType = pgtype.Text{String: *st.AssigneeType, Valid: true}
		}

		subIssue, err := qtx.CreateIssue(r.Context(), db.CreateIssueParams{
			WorkspaceID:   parseUUID(workspaceID),
			Title:         st.Title,
			Description:   pgtype.Text{String: st.Description, Valid: st.Description != ""},
			Status:        "todo",
			Priority:      "none",
			AssigneeType:  assigneeType,
			AssigneeID:    assigneeID,
			CreatorType:   "member",
			CreatorID:     parseUUID(userID),
			ParentIssueID: issue.ID,
			Position:      float64(i),
			Number:        issueNumber,
		})
		if err != nil {
			slog.Warn("create sub-issue failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to create sub-issue")
			return
		}

		createdIDs[i] = subIssue.ID
		createdIssues = append(createdIssues, issueToResponse(subIssue, prefix))
	}

	// Wire dependencies.
	for i, st := range req.Subtasks {
		for _, depIdx := range st.DependsOn {
			if depIdx < 0 || depIdx >= len(createdIDs) || depIdx == i {
				continue
			}
			_, err := qtx.CreateIssueDependency(r.Context(), db.CreateIssueDependencyParams{
				IssueID:          createdIDs[i],
				DependsOnIssueID: createdIDs[depIdx],
				Type:             "blocked_by",
			})
			if err != nil {
				slog.Warn("create dependency failed", "error", err, "subtask", i, "dep", depIdx)
				// Non-fatal: continue creating remaining dependencies.
			}
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	// Enqueue any unblocked tasks.
	h.TaskService.TryEnqueueReadySubIssues(r.Context(), issue.ID)

	slog.Info("decompose: confirmed",
		append(logger.RequestAttrs(r),
			"goal_id", uuidToString(issue.ID),
			"subtasks_created", len(createdIssues),
			"workspace_id", workspaceID)...,
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"issues": createdIssues,
		"total":  len(createdIssues),
	})
}
