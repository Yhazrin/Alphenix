package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// WorkspaceRepoResponse is the JSON representation of a workspace repo.
type WorkspaceRepoResponse struct {
	ID            string `json:"id"`
	WorkspaceID   string `json:"workspace_id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
	Description   string `json:"description"`
	IsDefault     bool   `json:"is_default"`
	Config        any    `json:"config"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func workspaceRepoToResponse(r db.WorkspaceRepo) WorkspaceRepoResponse {
	var config any
	if r.Config != nil {
		json.Unmarshal(r.Config, &config)
	}
	if config == nil {
		config = map[string]any{}
	}

	var desc string
	if r.Description.Valid {
		desc = r.Description.String
	}

	return WorkspaceRepoResponse{
		ID:            uuidToString(r.ID),
		WorkspaceID:   uuidToString(r.WorkspaceID),
		Name:          r.Name,
		URL:           r.Url,
		DefaultBranch: r.DefaultBranch,
		Description:   desc,
		IsDefault:     r.IsDefault,
		Config:        config,
		CreatedAt:     r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     r.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
}

// ListWorkspaceRepos returns all repos in a workspace.
func (h *Handler) ListWorkspaceRepos(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")
	_, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}

	repos, err := h.Queries.ListWorkspaceRepos(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workspace repos")
		return
	}

	result := make([]WorkspaceRepoResponse, len(repos))
	for i, repo := range repos {
		result[i] = workspaceRepoToResponse(repo)
	}
	writeJSON(w, http.StatusOK, result)
}

// GetWorkspaceRepo returns a single repo by ID.
func (h *Handler) GetWorkspaceRepo(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	repo, err := h.Queries.GetWorkspaceRepo(r.Context(), parseUUID(repoID))
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}

	workspaceID := uuidToString(repo.WorkspaceID)
	_, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, workspaceRepoToResponse(repo))
}

// CreateWorkspaceRepoRequest is the request body for creating a workspace repo.
type CreateWorkspaceRepoRequest struct {
	Name          string          `json:"name"`
	URL           string          `json:"url"`
	DefaultBranch string          `json:"default_branch"`
	Description   string          `json:"description"`
	IsDefault     bool            `json:"is_default"`
	Config        json.RawMessage `json:"config"`
}

// CreateWorkspaceRepo creates a new repo in a workspace.
func (h *Handler) CreateWorkspaceRepo(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "repo not found", "owner", "admin"); !ok {
		return
	}

	var req CreateWorkspaceRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	config := req.Config
	if config == nil {
		config = json.RawMessage("{}")
	}

	repo, err := h.Queries.CreateWorkspaceRepo(r.Context(), db.CreateWorkspaceRepoParams{
		WorkspaceID:   parseUUID(workspaceID),
		Name:          req.Name,
		Url:           req.URL,
		DefaultBranch: req.DefaultBranch,
		Description:   pgtype.Text{String: req.Description, Valid: req.Description != ""},
		IsDefault:     req.IsDefault,
		Config:        config,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workspace repo")
		return
	}

	writeJSON(w, http.StatusCreated, workspaceRepoToResponse(repo))
}

// UpdateWorkspaceRepoRequest is the request body for updating a workspace repo.
type UpdateWorkspaceRepoRequest struct {
	Name          *string          `json:"name"`
	URL           *string          `json:"url"`
	DefaultBranch *string          `json:"default_branch"`
	Description   *string          `json:"description"`
	IsDefault     *bool            `json:"is_default"`
	Config        *json.RawMessage `json:"config"`
}

// UpdateWorkspaceRepo updates an existing repo.
func (h *Handler) UpdateWorkspaceRepo(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	existing, err := h.Queries.GetWorkspaceRepo(r.Context(), parseUUID(repoID))
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}

	workspaceID := uuidToString(existing.WorkspaceID)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "repo not found", "owner", "admin"); !ok {
		return
	}

	var req UpdateWorkspaceRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateWorkspaceRepoParams{
		ID: parseUUID(repoID),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.URL != nil {
		params.Url = pgtype.Text{String: *req.URL, Valid: true}
	}
	if req.DefaultBranch != nil {
		params.DefaultBranch = pgtype.Text{String: *req.DefaultBranch, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.IsDefault != nil {
		params.IsDefault = pgtype.Bool{Bool: *req.IsDefault, Valid: true}
	}
	if req.Config != nil {
		params.Config = *req.Config
	}

	repo, err := h.Queries.UpdateWorkspaceRepo(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update workspace repo")
		return
	}

	writeJSON(w, http.StatusOK, workspaceRepoToResponse(repo))
}

// DeleteWorkspaceRepo deletes a repo.
func (h *Handler) DeleteWorkspaceRepo(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	existing, err := h.Queries.GetWorkspaceRepo(r.Context(), parseUUID(repoID))
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}

	workspaceID := uuidToString(existing.WorkspaceID)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "repo not found", "owner", "admin"); !ok {
		return
	}

	if err := h.Queries.DeleteWorkspaceRepo(r.Context(), parseUUID(repoID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete workspace repo")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListIssuesByRepoID returns all issues for a given repo.
func (h *Handler) ListIssuesByRepoID(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	repo, err := h.Queries.GetWorkspaceRepo(r.Context(), parseUUID(repoID))
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}

	workspaceID := uuidToString(repo.WorkspaceID)
	_, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}

	issues, err := h.Queries.ListIssuesByRepoID(r.Context(), parseUUID(repoID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issues by repo")
		return
	}

	prefix := h.getIssuePrefix(r.Context(), repo.WorkspaceID)
	resp := make([]IssueResponse, len(issues))
	for i, issue := range issues {
		resp[i] = issueToResponse(issue, prefix)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"issues": resp,
		"total":  len(resp),
	})
}
