package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/logger"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// IssueResponse is the JSON response for an issue.
type IssueResponse struct {
	ID                 string                  `json:"id"`
	WorkspaceID        string                  `json:"workspace_id"`
	Number             int32                   `json:"number"`
	Identifier         string                  `json:"identifier"`
	Title              string                  `json:"title"`
	Description        *string                 `json:"description"`
	Status             string                  `json:"status"`
	Priority           string                  `json:"priority"`
	AssigneeType       *string                 `json:"assignee_type"`
	AssigneeID         *string                 `json:"assignee_id"`
	CreatorType        string                  `json:"creator_type"`
	CreatorID          string                  `json:"creator_id"`
	ParentIssueID      *string                 `json:"parent_issue_id"`
	RepoID             *string                 `json:"repo_id"`
	Position           float64                 `json:"position"`
	DueDate            *string                 `json:"due_date"`
	CreatedAt          string                  `json:"created_at"`
	UpdatedAt          string                  `json:"updated_at"`
	IssueKind          string                  `json:"issue_kind"`
	LatestTaskStatus   *string                 `json:"latest_task_status,omitempty"`
	Reactions          []IssueReactionResponse `json:"reactions,omitempty"`
	Attachments        []AttachmentResponse    `json:"attachments,omitempty"`
}

type agentTriggerSnapshot struct {
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
}

// defaultAgentTriggers returns the default trigger config for new agents:
// all three triggers explicitly enabled.
func defaultAgentTriggers() []byte {
	b, _ := json.Marshal([]agentTriggerSnapshot{
		{Type: "on_assign", Enabled: true},
		{Type: "on_comment", Enabled: true},
		{Type: "on_mention", Enabled: true},
	})
	return b
}

func issueToResponse(i db.Issue, issuePrefix string) IssueResponse {
	identifier := issuePrefix + "-" + strconv.Itoa(int(i.Number))
	return IssueResponse{
		ID:            uuidToString(i.ID),
		WorkspaceID:   uuidToString(i.WorkspaceID),
		Number:        i.Number,
		Identifier:    identifier,
		Title:         i.Title,
		Description:   textToPtr(i.Description),
		Status:        i.Status,
		Priority:      i.Priority,
		AssigneeType:  textToPtr(i.AssigneeType),
		AssigneeID:    uuidToPtr(i.AssigneeID),
		CreatorType:   i.CreatorType,
		CreatorID:     uuidToString(i.CreatorID),
		ParentIssueID: uuidToPtr(i.ParentIssueID),
		RepoID:        uuidToPtr(i.RepoID),
		IssueKind:     i.IssueKind,
		Position:      i.Position,
		DueDate:       timestampToPtr(i.DueDate),
		CreatedAt:     timestampToString(i.CreatedAt),
		UpdatedAt:     timestampToString(i.UpdatedAt),
	}
}

func (h *Handler) ListIssues(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	workspaceID := resolveWorkspaceID(r)

	limit := 100
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			if v < 0 {
				v = 0
			}
			if v > 200 {
				v = 200
			}
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}

	// Parse optional filter params
	var statusFilter pgtype.Text
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = pgtype.Text{String: s, Valid: true}
	}
	var priorityFilter pgtype.Text
	if p := r.URL.Query().Get("priority"); p != "" {
		priorityFilter = pgtype.Text{String: p, Valid: true}
	}
	var assigneeFilter pgtype.UUID
	if a := r.URL.Query().Get("assignee_id"); a != "" {
		assigneeFilter = parseUUID(a)
	}

	rows, err := h.Queries.ListIssuesWithTaskStatus(ctx, db.ListIssuesWithTaskStatusParams{
		WorkspaceID: parseUUID(workspaceID),
		Limit:       int32(limit),
		Offset:      int32(offset),
		Status:      statusFilter,
		Priority:    priorityFilter,
		AssigneeID:  assigneeFilter,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issues")
		return
	}

	prefix := h.getIssuePrefix(ctx, parseUUID(workspaceID))
	resp := make([]IssueResponse, len(rows))
	for i, row := range rows {
		r := issueToResponse(db.Issue{
			ID:            row.ID,
			WorkspaceID:   row.WorkspaceID,
			Title:         row.Title,
			Description:   row.Description,
			Status:        row.Status,
			Priority:      row.Priority,
			AssigneeType:  row.AssigneeType,
			AssigneeID:    row.AssigneeID,
			CreatorType:   row.CreatorType,
			CreatorID:     row.CreatorID,
			ParentIssueID: row.ParentIssueID,
			Position:      row.Position,
			DueDate:       row.DueDate,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			Number:        row.Number,
			RepoID:        row.RepoID,
		}, prefix)
		if row.LatestTaskStatus != "" {
			s := row.LatestTaskStatus
			r.LatestTaskStatus = &s
		}
		resp[i] = r
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"issues": resp,
		"total":  len(resp),
	})
}

func (h *Handler) GetIssue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, id)
	if !ok {
		return
	}
	prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)
	resp := issueToResponse(issue, prefix)

	// Fetch issue reactions.
	reactions, err := h.Queries.ListIssueReactions(r.Context(), issue.ID)
	if err == nil && len(reactions) > 0 {
		resp.Reactions = make([]IssueReactionResponse, len(reactions))
		for i, rx := range reactions {
			resp.Reactions[i] = issueReactionToResponse(rx)
		}
	}

	// Fetch issue-level attachments.
	attachments, err := h.Queries.ListAttachmentsByIssue(r.Context(), db.ListAttachmentsByIssueParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
	})
	if err == nil && len(attachments) > 0 {
		resp.Attachments = make([]AttachmentResponse, len(attachments))
		for i, a := range attachments {
			resp.Attachments[i] = h.attachmentToResponse(a)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type CreateIssueRequest struct {
	Title              string  `json:"title"`
	Description        *string `json:"description"`
	Status             string  `json:"status"`
	Priority           string  `json:"priority"`
	AssigneeType       *string `json:"assignee_type"`
	AssigneeID         *string `json:"assignee_id"`
	ParentIssueID      *string `json:"parent_issue_id"`
	RepoID             *string `json:"repo_id"`
	DueDate            *string `json:"due_date"`
	IssueKind          string  `json:"issue_kind"`
}

func (h *Handler) CreateIssue(w http.ResponseWriter, r *http.Request) {
	var req CreateIssueRequest
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if len(req.Title) > 500 {
		writeError(w, http.StatusBadRequest, "title must be 500 characters or fewer")
		return
	}

	workspaceID := resolveWorkspaceID(r)

	// Get creator from context (set by auth middleware)
	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	status := req.Status
	if status == "" {
		status = "backlog"
	}
	priority := req.Priority
	if priority == "" {
		priority = "none"
	}

	var assigneeType pgtype.Text
	var assigneeID pgtype.UUID
	if req.AssigneeType != nil {
		assigneeType = pgtype.Text{String: *req.AssigneeType, Valid: true}
	}
	if req.AssigneeID != nil {
		assigneeID = parseUUID(*req.AssigneeID)
	}

	// Enforce agent visibility: private agents can only be assigned by owner/admin.
	if req.AssigneeType != nil && *req.AssigneeType == "agent" && req.AssigneeID != nil {
		if ok, msg := h.canAssignAgent(r.Context(), r, *req.AssigneeID, workspaceID); !ok {
			writeError(w, http.StatusForbidden, msg)
			return
		}
	}

	var parentIssueID pgtype.UUID
	if req.ParentIssueID != nil {
		parentIssueID = parseUUID(*req.ParentIssueID)
	}

	var dueDate pgtype.Timestamptz
	if req.DueDate != nil && *req.DueDate != "" {
		t, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid due_date format, expected RFC3339")
			return
		}
		dueDate = pgtype.Timestamptz{Time: t, Valid: true}
	}

	var repoID pgtype.UUID
	if req.RepoID != nil {
		repoID = parseUUID(*req.RepoID)
	}

	// Use a transaction to atomically increment the workspace issue counter
	// and create the issue with the assigned number.
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)
	issueNumber, err := qtx.IncrementIssueCounter(r.Context(), parseUUID(workspaceID))
	if err != nil {
		slog.Warn("increment issue counter failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}

	// Determine creator identity: agent (via X-Agent-ID header) or member.
	creatorType, actualCreatorID := h.resolveActor(r, creatorID, workspaceID)

	issue, err := qtx.CreateIssue(r.Context(), db.CreateIssueParams{
		WorkspaceID:        parseUUID(workspaceID),
		Title:              req.Title,
		Description:        ptrToText(req.Description),
		Status:             status,
		Priority:           priority,
		AssigneeType:       assigneeType,
		AssigneeID:         assigneeID,
		CreatorType:        creatorType,
		CreatorID:          parseUUID(actualCreatorID),
		ParentIssueID:      parentIssueID,
		Position:           0,
		DueDate:            dueDate,
		Number:             issueNumber,
		RepoID:             repoID,
		IssueKind:          req.IssueKind,
	})
	if err != nil {
		slog.Warn("create issue failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}

	prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)
	resp := issueToResponse(issue, prefix)
	slog.Info("issue created", append(logger.RequestAttrs(r), "issue_id", uuidToString(issue.ID), "title", issue.Title, "status", issue.Status, "workspace_id", workspaceID)...)
	h.publish(protocol.EventIssueCreated, workspaceID, creatorType, actualCreatorID, map[string]any{"issue": resp})

	// Only ready issues in todo are enqueued for agents.
	if issue.AssigneeType.Valid && issue.AssigneeID.Valid {
		slog.Debug("CreateIssue: assignee set, checking trigger", "issue_id", uuidToString(issue.ID))
		if h.shouldEnqueueAgentTask(r.Context(), issue) {
			slog.Debug("CreateIssue: trigger enabled, enqueuing task", "issue_id", uuidToString(issue.ID))
			h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
		} else {
			slog.Debug("CreateIssue: trigger disabled", "issue_id", uuidToString(issue.ID))
		}
	}

	writeJSON(w, http.StatusCreated, resp)
}

type UpdateIssueRequest struct {
	Title              *string  `json:"title"`
	Description        *string  `json:"description"`
	Status             *string  `json:"status"`
	Priority           *string  `json:"priority"`
	AssigneeType       *string  `json:"assignee_type"`
	AssigneeID         *string  `json:"assignee_id"`
	Position           *float64 `json:"position"`
	RepoID			*string	`json:"repo_id"`
	DueDate            *string  `json:"due_date"`
}

func (h *Handler) UpdateIssue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	prevIssue, ok := h.loadIssueForUser(w, r, id)
	if !ok {
		return
	}
	userID := requestUserID(r)
	workspaceID := uuidToString(prevIssue.WorkspaceID)

	// Read body as raw bytes so we can detect which fields were explicitly sent.
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req UpdateIssueRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Track which fields were explicitly present in JSON (even if null)
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawFields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Pre-fill nullable fields (bare sqlc.narg) with current values
	params := db.UpdateIssueParams{
		ID:           prevIssue.ID,
		AssigneeType: prevIssue.AssigneeType,
		AssigneeID:   prevIssue.AssigneeID,
		DueDate:      prevIssue.DueDate,
		RepoID:       prevIssue.RepoID,
	}

	// COALESCE fields — only set when explicitly provided
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Priority != nil {
		params.Priority = pgtype.Text{String: *req.Priority, Valid: true}
	}
	if req.Position != nil {
		params.Position = pgtype.Float8{Float64: *req.Position, Valid: true}
	}
	// Nullable fields — only override when explicitly present in JSON
	if _, ok := rawFields["assignee_type"]; ok {
		if req.AssigneeType != nil {
			params.AssigneeType = pgtype.Text{String: *req.AssigneeType, Valid: true}
		} else {
			params.AssigneeType = pgtype.Text{Valid: false} // explicit null = unassign
		}
	}
	if _, ok := rawFields["assignee_id"]; ok {
		if req.AssigneeID != nil {
			params.AssigneeID = parseUUID(*req.AssigneeID)
		} else {
			params.AssigneeID = pgtype.UUID{Valid: false} // explicit null = unassign
		}
	}
	if _, ok := rawFields["due_date"]; ok {
		if req.DueDate != nil && *req.DueDate != "" {
			t, err := time.Parse(time.RFC3339, *req.DueDate)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid due_date format, expected RFC3339")
				return
			}
			params.DueDate = pgtype.Timestamptz{Time: t, Valid: true}
		} else {
			params.DueDate = pgtype.Timestamptz{Valid: false} // explicit null = clear date
		}
	}
	if _, ok := rawFields["repo_id"]; ok {
		if req.RepoID != nil {
			params.RepoID = parseUUID(*req.RepoID)
		} else {
			params.RepoID = pgtype.UUID{Valid: false} // explicit null = unassign
		}
	}

	// Enforce agent visibility: private agents can only be assigned by owner/admin.
	if req.AssigneeType != nil && *req.AssigneeType == "agent" && req.AssigneeID != nil {
		if ok, msg := h.canAssignAgent(r.Context(), r, *req.AssigneeID, workspaceID); !ok {
			writeError(w, http.StatusForbidden, msg)
			return
		}
	}

	issue, err := h.Queries.UpdateIssue(r.Context(), params)
	if err != nil {
		slog.Warn("update issue failed", append(logger.RequestAttrs(r), "error", err, "issue_id", id, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to update issue")
		return
	}

	prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)
	resp := issueToResponse(issue, prefix)
	slog.Info("issue updated", append(logger.RequestAttrs(r), "issue_id", id, "workspace_id", workspaceID)...)

	assigneeChanged := (req.AssigneeType != nil || req.AssigneeID != nil) &&
		(prevIssue.AssigneeType.String != issue.AssigneeType.String || uuidToString(prevIssue.AssigneeID) != uuidToString(issue.AssigneeID))
	statusChanged := req.Status != nil && prevIssue.Status != issue.Status
	priorityChanged := req.Priority != nil && prevIssue.Priority != issue.Priority
	descriptionChanged := req.Description != nil && textToPtr(prevIssue.Description) != resp.Description
	titleChanged := req.Title != nil && prevIssue.Title != issue.Title
	prevDueDate := timestampToPtr(prevIssue.DueDate)
	dueDateChanged := prevDueDate != resp.DueDate && (prevDueDate == nil) != (resp.DueDate == nil) ||
		(prevDueDate != nil && resp.DueDate != nil && *prevDueDate != *resp.DueDate)

	// Determine actor identity: agent (via X-Agent-ID header) or member.
	actorType, actorID := h.resolveActor(r, userID, workspaceID)

	h.publish(protocol.EventIssueUpdated, workspaceID, actorType, actorID, map[string]any{
		"issue":               resp,
		"assignee_changed":    assigneeChanged,
		"status_changed":      statusChanged,
		"priority_changed":    priorityChanged,
		"due_date_changed":    dueDateChanged,
		"description_changed": descriptionChanged,
		"title_changed":       titleChanged,
		"prev_title":          prevIssue.Title,
		"prev_assignee_type":  textToPtr(prevIssue.AssigneeType),
		"prev_assignee_id":    uuidToPtr(prevIssue.AssigneeID),
		"prev_status":         prevIssue.Status,
		"prev_priority":       prevIssue.Priority,
		"prev_due_date":       prevDueDate,
		"prev_description":    textToPtr(prevIssue.Description),
		"creator_type":        prevIssue.CreatorType,
		"creator_id":          uuidToString(prevIssue.CreatorID),
	})

	// Reconcile task queue when assignee changes (not on status changes —
	// agents manage issue status themselves via the CLI).
	if assigneeChanged {
		h.TaskService.CancelTasksForIssue(r.Context(), issue.ID)

		if h.shouldEnqueueAgentTask(r.Context(), issue) {
			h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// canAssignAgent checks whether the requesting user is allowed to assign issues
// to the given agent. Private agents can only be assigned by their owner or
// workspace admins/owners.
func (h *Handler) canAssignAgent(ctx context.Context, r *http.Request, agentID, workspaceID string) (bool, string) {
	agent, err := h.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
		ID:          parseUUID(agentID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return false, "agent not found"
	}
	if agent.ArchivedAt.Valid {
		return false, "cannot assign to archived agent"
	}
	if agent.Visibility != "private" {
		return true, ""
	}
	userID := requestUserID(r)
	if uuidToString(agent.OwnerID) == userID {
		return true, ""
	}
	member, err := h.getWorkspaceMember(ctx, userID, workspaceID)
	if err != nil {
		return false, "cannot assign to private agent"
	}
	if roleAllowed(member.Role, "owner", "admin") {
		return true, ""
	}
	return false, "cannot assign to private agent"
}

// shouldEnqueueAgentTask returns true when an issue assignment should trigger
// the assigned agent. No status gate — assignment is an explicit human action,
// so it should trigger regardless of issue status (e.g. assigning an agent to
// a done issue to fix a discovered problem).
func (h *Handler) shouldEnqueueAgentTask(ctx context.Context, issue db.Issue) bool {
	return h.isAgentTriggerEnabled(ctx, issue, "on_assign")
}

// shouldEnqueueOnComment returns true if a member comment on this issue should
// trigger the assigned agent. Fires for any non-terminal status — comments are
// conversational and can happen at any stage of active work.
func (h *Handler) shouldEnqueueOnComment(ctx context.Context, issue db.Issue) bool {
	// Don't trigger on terminal statuses (done, cancelled).
	if issue.Status == "done" || issue.Status == "cancelled" {
		return false
	}
	if !h.isAgentTriggerEnabled(ctx, issue, "on_comment") {
		return false
	}
	// Coalescing queue: allow enqueue when a task is running (so the agent
	// picks up new comments on the next cycle) but skip if a pending task
	// already exists (natural dedup for rapid-fire comments).
	hasPending, err := h.Queries.HasPendingTaskForIssue(ctx, issue.ID)
	if err != nil || hasPending {
		return false
	}
	return true
}

// isAgentTriggerEnabled checks if an issue is assigned to an agent with a
// specific trigger type enabled. Returns true if the agent has no triggers
// configured (default-enabled behavior for backwards compatibility).
func (h *Handler) isAgentTriggerEnabled(ctx context.Context, issue db.Issue, triggerType string) bool {
	if !issue.AssigneeType.Valid {
		slog.Debug("isAgentTriggerEnabled: AssigneeType not valid", "issue_id", uuidToString(issue.ID))
		return false
	}
	if issue.AssigneeType.String != "agent" {
		slog.Debug("isAgentTriggerEnabled: AssigneeType not agent", "issue_id", uuidToString(issue.ID), "assignee_type", issue.AssigneeType.String)
		return false
	}
	if !issue.AssigneeID.Valid {
		slog.Debug("isAgentTriggerEnabled: AssigneeID not valid", "issue_id", uuidToString(issue.ID))
		return false
	}

	agent, err := h.Queries.GetAgent(ctx, issue.AssigneeID)
	if err != nil {
		slog.Debug("isAgentTriggerEnabled: GetAgent failed", "issue_id", uuidToString(issue.ID), "error", err)
		return false
	}
	if !agent.RuntimeID.Valid {
		slog.Debug("isAgentTriggerEnabled: agent.RuntimeID not valid", "issue_id", uuidToString(issue.ID), "agent_id", uuidToString(issue.AssigneeID))
		return false
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("isAgentTriggerEnabled: agent is archived", "issue_id", uuidToString(issue.ID), "agent_id", uuidToString(issue.AssigneeID))
		return false
	}

	enabled := agentHasTriggerEnabled(agent.Triggers, triggerType)
	slog.Debug("isAgentTriggerEnabled", "issue_id", uuidToString(issue.ID), "trigger", triggerType, "enabled", enabled)
	return enabled
}

// isAgentMentionTriggerEnabled checks if a specific agent has the on_mention
// trigger enabled. Unlike isAgentTriggerEnabled, this takes an explicit agent
// ID rather than deriving it from the issue assignee.
func (h *Handler) isAgentMentionTriggerEnabled(ctx context.Context, agentID pgtype.UUID) bool {
	agent, err := h.Queries.GetAgent(ctx, agentID)
	if err != nil || !agent.RuntimeID.Valid {
		return false
	}

	return agentHasTriggerEnabled(agent.Triggers, "on_mention")
}

// agentHasTriggerEnabled checks if a trigger type is enabled in the agent's
// trigger config. Returns true (default-enabled) when the triggers list is
// empty or does not contain the requested type — for backwards compatibility
// with agents created before explicit trigger config was introduced.
func agentHasTriggerEnabled(raw []byte, triggerType string) bool {
	if raw == nil || len(raw) == 0 {
		return true
	}

	var triggers []agentTriggerSnapshot
	if err := json.Unmarshal(raw, &triggers); err != nil {
		return false
	}
	if len(triggers) == 0 {
		return true // Empty array = default-enabled (backwards compat)
	}
	for _, trigger := range triggers {
		if trigger.Type == triggerType {
			return trigger.Enabled
		}
	}
	return true // Trigger type not configured = enabled by default
}

func (h *Handler) DeleteIssue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, id)
	if !ok {
		return
	}

	h.TaskService.CancelTasksForIssue(r.Context(), issue.ID)

	// Collect all attachment URLs (issue-level + comment-level) before CASCADE delete.
	attachmentURLs, err := h.Queries.ListAttachmentURLsByIssueOrComments(r.Context(), issue.ID)
	if err != nil {
		slog.Warn("failed to list attachment URLs for issue cleanup", "issue_id", uuidToString(issue.ID), "error", err)
		attachmentURLs = nil
	}

	err = h.Queries.DeleteIssue(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete issue")
		return
	}

	h.deleteS3Objects(r.Context(), attachmentURLs)
	userID := requestUserID(r)
	actorType, actorID := h.resolveActor(r, userID, uuidToString(issue.WorkspaceID))
	h.publish(protocol.EventIssueDeleted, uuidToString(issue.WorkspaceID), actorType, actorID, map[string]any{"issue_id": id})
	slog.Info("issue deleted", append(logger.RequestAttrs(r), "issue_id", id, "workspace_id", uuidToString(issue.WorkspaceID))...)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Batch operations
// ---------------------------------------------------------------------------

type BatchUpdateIssuesRequest struct {
	IssueIDs []string           `json:"issue_ids"`
	Updates  UpdateIssueRequest `json:"updates"`
}

func (h *Handler) BatchUpdateIssues(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req BatchUpdateIssuesRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IssueIDs) == 0 {
		writeError(w, http.StatusBadRequest, "issue_ids is required")
		return
	}
	if len(req.IssueIDs) > 500 {
		writeError(w, http.StatusBadRequest, "too many issue IDs (max 500)")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Detect which fields in "updates" were explicitly set (including null).
	var rawTop map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawTop); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var rawUpdates map[string]json.RawMessage
	if raw, exists := rawTop["updates"]; exists {
		if err := json.Unmarshal(raw, &rawUpdates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	workspaceID := resolveWorkspaceID(r)

	// Batch fetch all issues in a single query instead of N+1.
	prevIssues, err := h.batchGetIssuesByIDs(r.Context(), req.IssueIDs, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch issues")
		return
	}

	// Index by ID for quick lookup.
	prevByID := make(map[string]db.Issue, len(prevIssues))
	for _, issue := range prevIssues {
		prevByID[uuidToString(issue.ID)] = issue
	}

	updated := 0
	prefix := h.getIssuePrefix(r.Context(), parseUUID(workspaceID))

	// Wrap batch updates in a transaction for atomicity.
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context())
	txQueries := h.Queries.WithTx(tx)

	type pendingEvent struct {
		workspaceID, actorType, actorID string
		payload                         map[string]any
	}
	var events []pendingEvent

	for _, issueID := range req.IssueIDs {
		prevIssue, ok := prevByID[issueID]
		if !ok {
			continue
		}

		params := db.UpdateIssueParams{
			ID:           prevIssue.ID,
			AssigneeType: prevIssue.AssigneeType,
			AssigneeID:   prevIssue.AssigneeID,
			DueDate:      prevIssue.DueDate,
		}

		if req.Updates.Title != nil {
			params.Title = pgtype.Text{String: *req.Updates.Title, Valid: true}
		}
		if req.Updates.Description != nil {
			params.Description = pgtype.Text{String: *req.Updates.Description, Valid: true}
		}
		if req.Updates.Status != nil {
			params.Status = pgtype.Text{String: *req.Updates.Status, Valid: true}
		}
		if req.Updates.Priority != nil {
			params.Priority = pgtype.Text{String: *req.Updates.Priority, Valid: true}
		}
		if req.Updates.Position != nil {
			params.Position = pgtype.Float8{Float64: *req.Updates.Position, Valid: true}
		}
		if _, ok := rawUpdates["assignee_type"]; ok {
			if req.Updates.AssigneeType != nil {
				params.AssigneeType = pgtype.Text{String: *req.Updates.AssigneeType, Valid: true}
			} else {
				params.AssigneeType = pgtype.Text{Valid: false}
			}
		}
		if _, ok := rawUpdates["assignee_id"]; ok {
			if req.Updates.AssigneeID != nil {
				params.AssigneeID = parseUUID(*req.Updates.AssigneeID)
			} else {
				params.AssigneeID = pgtype.UUID{Valid: false}
			}
		}
		if _, ok := rawUpdates["due_date"]; ok {
			if req.Updates.DueDate != nil && *req.Updates.DueDate != "" {
				t, err := time.Parse(time.RFC3339, *req.Updates.DueDate)
				if err != nil {
					continue
				}
				params.DueDate = pgtype.Timestamptz{Time: t, Valid: true}
			} else {
				params.DueDate = pgtype.Timestamptz{Valid: false}
			}
		}

		// Enforce agent visibility for batch assignment.
		if req.Updates.AssigneeType != nil && *req.Updates.AssigneeType == "agent" && req.Updates.AssigneeID != nil {
			if ok, _ := h.canAssignAgent(r.Context(), r, *req.Updates.AssigneeID, workspaceID); !ok {
				continue
			}
		}

		issue, err := txQueries.UpdateIssue(r.Context(), params)
		if err != nil {
			slog.Warn("batch update issue failed", "issue_id", issueID, "error", err)
			continue
		}

		resp := issueToResponse(issue, prefix)
		actorType, actorID := h.resolveActor(r, userID, workspaceID)

		assigneeChanged := (req.Updates.AssigneeType != nil || req.Updates.AssigneeID != nil) &&
			(prevIssue.AssigneeType.String != issue.AssigneeType.String || uuidToString(prevIssue.AssigneeID) != uuidToString(issue.AssigneeID))
		statusChanged := req.Updates.Status != nil && prevIssue.Status != issue.Status
		priorityChanged := req.Updates.Priority != nil && prevIssue.Priority != issue.Priority

		events = append(events, pendingEvent{
			workspaceID: workspaceID,
			actorType:   actorType,
			actorID:     actorID,
			payload: map[string]any{
				"issue":            resp,
				"assignee_changed": assigneeChanged,
				"status_changed":   statusChanged,
				"priority_changed": priorityChanged,
			},
		})

		if assigneeChanged {
			h.TaskService.CancelTasksForIssue(r.Context(), issue.ID)
			if h.shouldEnqueueAgentTask(r.Context(), issue) {
				h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
			}
		}

		updated++
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	for _, e := range events {
		h.publish(protocol.EventIssueUpdated, e.workspaceID, e.actorType, e.actorID, e.payload)
	}

	slog.Info("batch update issues", append(logger.RequestAttrs(r), "count", updated)...)
	writeJSON(w, http.StatusOK, map[string]any{"updated": updated})
}

type BatchDeleteIssuesRequest struct {
	IssueIDs []string `json:"issue_ids"`
}

func (h *Handler) BatchDeleteIssues(w http.ResponseWriter, r *http.Request) {
	var req BatchDeleteIssuesRequest
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IssueIDs) == 0 {
		writeError(w, http.StatusBadRequest, "issue_ids is required")
		return
	}
	if len(req.IssueIDs) > 500 {
		writeError(w, http.StatusBadRequest, "too many issue IDs (max 500)")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	workspaceID := resolveWorkspaceID(r)

	// Batch fetch all issues in a single query instead of N+1.
	issues, err := h.batchGetIssuesByIDs(r.Context(), req.IssueIDs, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch issues")
		return
	}

	// Cancel tasks for all fetched issues.
	for _, issue := range issues {
		h.TaskService.CancelTasksForIssue(r.Context(), issue.ID)
	}

	// Collect attachment URLs before delete.
	for _, issue := range issues {
		attachmentURLs, err := h.Queries.ListAttachmentURLsByIssueOrComments(r.Context(), issue.ID)
		if err != nil {
			slog.Warn("failed to list attachment URLs for batch issue cleanup", "issue_id", uuidToString(issue.ID), "error", err)
			continue
		}
		h.deleteS3Objects(r.Context(), attachmentURLs)
	}

	// Batch delete all issues in a single query.
	if err := h.batchDeleteIssues(r.Context(), req.IssueIDs, workspaceID); err != nil {
		slog.Warn("batch delete issues failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete issues")
		return
	}

	// Publish delete events for each issue.
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	for _, issue := range issues {
		h.publish(protocol.EventIssueDeleted, workspaceID, actorType, actorID, map[string]any{"issue_id": uuidToString(issue.ID)})
	}

	deleted := len(issues)
	slog.Info("batch delete issues", append(logger.RequestAttrs(r), "count", deleted)...)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

// ── Issue identifier helpers ──────────────────────────────────────────────────

type identifierParts struct {
	prefix string
	number int32
}

// splitIdentifier parses "PREFIX-NUMBER" into its components.
// Returns nil if the string is not in that format.
func splitIdentifier(id string) *identifierParts {
	idx := -1
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '-' {
			idx = i
			break
		}
	}
	if idx <= 0 || idx >= len(id)-1 {
		return nil
	}
	numStr := id[idx+1:]
	num := 0
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return nil
		}
		num = num*10 + int(c-'0')
	}
	if num <= 0 {
		return nil
	}
	return &identifierParts{prefix: id[:idx], number: int32(num)}
}

// getIssuePrefix fetches the issue_prefix for a workspace, using an in-memory
// cache to avoid repeated DB lookups. Falls back to generating a prefix from
// the workspace name if the stored prefix is empty.
func (h *Handler) getIssuePrefix(ctx context.Context, workspaceID pgtype.UUID) string {
	key := uuidToString(workspaceID)
	if cached, ok := h.prefixCache.Load(key); ok {
		return cached.(string)
	}
	ws, err := h.Queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return ""
	}
	prefix := ws.IssuePrefix
	if prefix == "" {
		prefix = generateIssuePrefix(ws.Name)
	}
	h.prefixCache.Store(key, prefix)
	return prefix
}

// resolveIssueByIdentifier tries to look up an issue by "PREFIX-NUMBER" format.
// The prefix must match the workspace's actual issue_prefix, preventing ambiguous
// resolution (e.g. MUL-42 vs XXX-42 both resolving to number 42).
func (h *Handler) resolveIssueByIdentifier(ctx context.Context, id, workspaceID string) (db.Issue, bool) {
	parts := splitIdentifier(id)
	if parts == nil {
		return db.Issue{}, false
	}
	if workspaceID == "" {
		return db.Issue{}, false
	}
	wsUUID := parseUUID(workspaceID)
	expectedPrefix := h.getIssuePrefix(ctx, wsUUID)
	if expectedPrefix != "" && !strings.EqualFold(parts.prefix, expectedPrefix) {
		return db.Issue{}, false
	}
	issue, err := h.Queries.GetIssueByNumber(ctx, db.GetIssueByNumberParams{
		WorkspaceID: wsUUID,
		Number:      parts.number,
	})
	if err != nil {
		return db.Issue{}, false
	}
	return issue, true
}

// loadIssueForUser resolves an issue by ID or "PREFIX-NUMBER" identifier,
// verifying the caller is authenticated and the issue belongs to their workspace.
func (h *Handler) loadIssueForUser(w http.ResponseWriter, r *http.Request, issueID string) (db.Issue, bool) {
	if _, ok := requireUserID(w, r); !ok {
		return db.Issue{}, false
	}

	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.Issue{}, false
	}

	if issue, ok := h.resolveIssueByIdentifier(r.Context(), issueID, workspaceID); ok {
		return issue, true
	}

	issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
		ID:          parseUUID(issueID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return db.Issue{}, false
	}
	return issue, true
}

// batchGetIssuesByIDs fetches multiple issues in a single query, returning only
// those that belong to the given workspace. Avoids N+1 queries in batch operations.
func (h *Handler) batchGetIssuesByIDs(ctx context.Context, issueIDs []string, workspaceID string) ([]db.Issue, error) {
	if h.DB == nil || len(issueIDs) == 0 {
		return nil, nil
	}
	uuids := make([]pgtype.UUID, len(issueIDs))
	for i, id := range issueIDs {
		uuids[i] = parseUUID(id)
	}
	rows, err := h.DB.Query(ctx,
		`SELECT id, workspace_id, title, description, status, priority,
		        assignee_type, assignee_id, creator_type, creator_id,
		        parent_issue_id, acceptance_criteria, context_refs,
		        position, due_date, created_at, updated_at, number, repo_id
		 FROM issue WHERE id = ANY($1::uuid[]) AND workspace_id = $2`,
		uuids, parseUUID(workspaceID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []db.Issue
	for rows.Next() {
		var i db.Issue
		err := rows.Scan(
			&i.ID, &i.WorkspaceID, &i.Title, &i.Description, &i.Status, &i.Priority,
			&i.AssigneeType, &i.AssigneeID, &i.CreatorType, &i.CreatorID,
			&i.ParentIssueID, &i.AcceptanceCriteria, &i.ContextRefs,
			&i.Position, &i.DueDate, &i.CreatedAt, &i.UpdatedAt, &i.Number,
				&i.RepoID,
		)
		if err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

// batchDeleteIssues deletes multiple issues in a single SQL statement.
func (h *Handler) batchDeleteIssues(ctx context.Context, issueIDs []string, workspaceID string) error {
	if h.DB == nil || len(issueIDs) == 0 {
		return nil
	}
	uuids := make([]pgtype.UUID, len(issueIDs))
	for i, id := range issueIDs {
		uuids[i] = parseUUID(id)
	}
	_, err := h.DB.Exec(ctx,
		`DELETE FROM issue WHERE id = ANY($1::uuid[]) AND workspace_id = $2`,
		uuids, parseUUID(workspaceID),
	)
	return err
}
