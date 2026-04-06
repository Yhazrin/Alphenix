package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// RuntimePolicyResponse is the JSON representation of a runtime assignment policy.
type RuntimePolicyResponse struct {
	ID                  string   `json:"id"`
	WorkspaceID         string   `json:"workspace_id"`
	AgentID             string   `json:"agent_id"`
	TeamID              *string  `json:"team_id,omitempty"`
	RequiredTags        []string `json:"required_tags"`
	ForbiddenTags       []string `json:"forbidden_tags"`
	PreferredRuntimeIds []string `json:"preferred_runtime_ids"`
	FallbackRuntimeIds  []string `json:"fallback_runtime_ids"`
	MaxQueueDepth       int32    `json:"max_queue_depth"`
	IsActive            bool     `json:"is_active"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

func runtimePolicyToResponse(p db.RuntimeAssignmentPolicy) RuntimePolicyResponse {
	resp := RuntimePolicyResponse{
		ID:                  uuidToString(p.ID),
		WorkspaceID:         uuidToString(p.WorkspaceID),
		AgentID:             uuidToString(p.AgentID),
		RequiredTags:        parseJSONStringSlice(p.RequiredTags),
		ForbiddenTags:       parseJSONStringSlice(p.ForbiddenTags),
		PreferredRuntimeIds: parseJSONStringSlice(p.PreferredRuntimeIds),
		FallbackRuntimeIds:  parseJSONStringSlice(p.FallbackRuntimeIds),
		MaxQueueDepth:       p.MaxQueueDepth,
		IsActive:            p.IsActive,
		CreatedAt:           timestampToString(p.CreatedAt),
		UpdatedAt:           timestampToString(p.UpdatedAt),
	}
	if p.TeamID.Valid {
		s := uuidToString(p.TeamID)
		resp.TeamID = &s
	}
	return resp
}

func parseJSONStringSlice(data []byte) []string {
	if data == nil {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return []string{}
	}
	if result == nil {
		return []string{}
	}
	return result
}

// ListRuntimePolicies returns all runtime assignment policies for the workspace.
// GET /api/runtime-policies
func (h *Handler) ListRuntimePolicies(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	rows, err := h.Queries.ListRuntimePolicies(r.Context(), parseUUID(workspaceID))
	if err != nil {
		slog.Error("list runtime policies failed", "workspace_id", workspaceID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list runtime policies")
		return
	}

	resp := make([]RuntimePolicyResponse, len(rows))
	for i, row := range rows {
		resp[i] = runtimePolicyToResponse(row)
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetRuntimePolicy returns a single runtime assignment policy.
// GET /api/runtime-policies/{id}
func (h *Handler) GetRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	policy, err := h.Queries.GetRuntimePolicy(r.Context(), db.GetRuntimePolicyParams{
		ID:          parseUUID(policyID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "runtime policy not found")
		return
	}

	writeJSON(w, http.StatusOK, runtimePolicyToResponse(policy))
}

// GetRuntimePolicyByAgent returns the runtime policy for a specific agent.
// GET /api/runtime-policies/by-agent/{agentId}
func (h *Handler) GetRuntimePolicyByAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentId")
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	policy, err := h.Queries.GetRuntimePolicyByAgent(r.Context(), db.GetRuntimePolicyByAgentParams{
		AgentID:     parseUUID(agentID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "runtime policy not found")
		return
	}

	writeJSON(w, http.StatusOK, runtimePolicyToResponse(policy))
}

type createRuntimePolicyRequest struct {
	AgentID             string   `json:"agent_id"`
	TeamID              *string  `json:"team_id"`
	RequiredTags        []string `json:"required_tags"`
	ForbiddenTags       []string `json:"forbidden_tags"`
	PreferredRuntimeIds []string `json:"preferred_runtime_ids"`
	FallbackRuntimeIds  []string `json:"fallback_runtime_ids"`
	MaxQueueDepth       int32    `json:"max_queue_depth"`
	IsActive            *bool    `json:"is_active"`
}

// CreateRuntimePolicy creates a new runtime assignment policy.
// POST /api/runtime-policies
func (h *Handler) CreateRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	var req createRuntimePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	// Verify agent exists in workspace.
	agent, err := h.Queries.GetAgent(r.Context(), parseUUID(req.AgentID))
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if uuidToString(agent.WorkspaceID) != workspaceID {
		writeError(w, http.StatusForbidden, "agent does not belong to this workspace")
		return
	}

	requiredTags, _ := json.Marshal(req.RequiredTags)
	forbiddenTags, _ := json.Marshal(req.ForbiddenTags)
	preferredIds, _ := json.Marshal(req.PreferredRuntimeIds)
	fallbackIds, _ := json.Marshal(req.FallbackRuntimeIds)

	params := db.CreateRuntimePolicyParams{
		WorkspaceID:         parseUUID(workspaceID),
		AgentID:             parseUUID(req.AgentID),
		RequiredTags:        requiredTags,
		ForbiddenTags:       forbiddenTags,
		PreferredRuntimeIds: preferredIds,
		FallbackRuntimeIds:  fallbackIds,
		MaxQueueDepth:       req.MaxQueueDepth,
		IsActive:            true,
	}
	if req.TeamID != nil {
		params.TeamID = parseUUID(*req.TeamID)
	}
	if req.IsActive != nil {
		params.IsActive = *req.IsActive
	}

	policy, err := h.Queries.CreateRuntimePolicy(r.Context(), params)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "a runtime policy already exists for this agent")
			return
		}
		slog.Error("create runtime policy failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create runtime policy")
		return
	}

	writeJSON(w, http.StatusCreated, runtimePolicyToResponse(policy))
}

type updateRuntimePolicyRequest struct {
	RequiredTags        []string `json:"required_tags"`
	ForbiddenTags       []string `json:"forbidden_tags"`
	PreferredRuntimeIds []string `json:"preferred_runtime_ids"`
	FallbackRuntimeIds  []string `json:"fallback_runtime_ids"`
	MaxQueueDepth       *int32   `json:"max_queue_depth"`
	IsActive            *bool    `json:"is_active"`
}

// UpdateRuntimePolicy patches an existing runtime assignment policy.
// PATCH /api/runtime-policies/{id}
func (h *Handler) UpdateRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	// Verify policy exists in workspace.
	existing, err := h.Queries.GetRuntimePolicy(r.Context(), db.GetRuntimePolicyParams{
		ID:          parseUUID(policyID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "runtime policy not found")
		return
	}

	var req updateRuntimePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateRuntimePolicyParams{
		ID: existing.ID,
	}

	if req.RequiredTags != nil {
		data, _ := json.Marshal(req.RequiredTags)
		params.RequiredTags = data
	}
	if req.ForbiddenTags != nil {
		data, _ := json.Marshal(req.ForbiddenTags)
		params.ForbiddenTags = data
	}
	if req.PreferredRuntimeIds != nil {
		data, _ := json.Marshal(req.PreferredRuntimeIds)
		params.PreferredRuntimeIds = data
	}
	if req.FallbackRuntimeIds != nil {
		data, _ := json.Marshal(req.FallbackRuntimeIds)
		params.FallbackRuntimeIds = data
	}
	if req.MaxQueueDepth != nil {
		params.MaxQueueDepth = pgtype.Int4{Int32: *req.MaxQueueDepth, Valid: true}
	}
	if req.IsActive != nil {
		params.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}

	policy, err := h.Queries.UpdateRuntimePolicy(r.Context(), params)
	if err != nil {
		slog.Error("update runtime policy failed", "policy_id", policyID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update runtime policy")
		return
	}

	writeJSON(w, http.StatusOK, runtimePolicyToResponse(policy))
}

// DeleteRuntimePolicy removes a runtime assignment policy.
// DELETE /api/runtime-policies/{id}
func (h *Handler) DeleteRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	err := h.Queries.DeleteRuntimePolicy(r.Context(), db.DeleteRuntimePolicyParams{
		ID:          parseUUID(policyID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		slog.Error("delete runtime policy failed", "policy_id", policyID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete runtime policy")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
