package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// MCPServerResponse is the JSON representation of an MCP server.
type MCPServerResponse struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspace_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Transport       string `json:"transport"`
	URL             string `json:"url"`
	Command         string `json:"command"`
	Args            any    `json:"args"`
	Env             any    `json:"env"`
	Status          string `json:"status"`
	LastError       string `json:"last_error"`
	LastConnectedAt *string `json:"last_connected_at"`
	Config          any    `json:"config"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func mcpServerToResponse(s db.McpServer) MCPServerResponse {
	var args any
	if s.Args != nil {
		json.Unmarshal(s.Args, &args)
	}
	if args == nil {
		args = []any{}
	}

	var env any
	if s.Env != nil {
		json.Unmarshal(s.Env, &env)
	}
	if env == nil {
		env = map[string]any{}
	}

	var config any
	if s.Config != nil {
		json.Unmarshal(s.Config, &config)
	}
	if config == nil {
		config = map[string]any{}
	}

	return MCPServerResponse{
		ID:              uuidToString(s.ID),
		WorkspaceID:     uuidToString(s.WorkspaceID),
		Name:            s.Name,
		Description:     s.Description,
		Transport:       s.Transport,
		URL:             s.Url,
		Command:         s.Command,
		Args:            args,
		Env:             env,
		Status:          s.Status,
		LastError:       s.LastError,
		LastConnectedAt: timestampToPtr(s.LastConnectedAt),
		Config:          config,
		CreatedAt:       s.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       s.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
}

// ListMCPServers returns all MCP servers in the workspace.
func (h *Handler) ListMCPServers(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "workspaceID")
	_, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}

	servers, err := h.Queries.ListMCPServersByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list MCP servers")
		return
	}

	result := make([]MCPServerResponse, len(servers))
	for i, s := range servers {
		result[i] = mcpServerToResponse(s)
	}
	writeJSON(w, http.StatusOK, result)
}

// GetMCPServer returns a single MCP server by ID.
func (h *Handler) GetMCPServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.Queries.GetMCPServer(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "MCP server not found")
		return
	}

	workspaceID := uuidToString(server.WorkspaceID)
	_, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, mcpServerToResponse(server))
}

// CreateMCPServerRequest is the request body for creating an MCP server.
type CreateMCPServerRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Transport   string          `json:"transport"`
	URL         string          `json:"url"`
	Command     string          `json:"command"`
	Args        json.RawMessage `json:"args"`
	Env         json.RawMessage `json:"env"`
	Config      json.RawMessage `json:"config"`
}

// CreateMCPServer creates a new MCP server entry.
func (h *Handler) CreateMCPServer(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "workspaceID")
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "MCP server not found", "owner", "admin"); !ok {
		return
	}

	var req CreateMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Transport == "" {
		req.Transport = "stdio"
	}

	args := []byte(req.Args)
	if args == nil {
		args = json.RawMessage("[]")
	}
	env := []byte(req.Env)
	if env == nil {
		env = json.RawMessage("{}")
	}
	config := []byte(req.Config)
	if config == nil {
		config = json.RawMessage("{}")
	}

	server, err := h.Queries.CreateMCPServer(r.Context(), db.CreateMCPServerParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		Url:         req.URL,
		Command:     req.Command,
		Args:        args,
		Env:         env,
		Config:      config,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create MCP server")
		return
	}

	writeJSON(w, http.StatusCreated, mcpServerToResponse(server))
}

// UpdateMCPServerRequest is the request body for updating an MCP server.
type UpdateMCPServerRequest struct {
	Name        *string          `json:"name"`
	Description *string          `json:"description"`
	Transport   *string          `json:"transport"`
	URL         *string          `json:"url"`
	Command     *string          `json:"command"`
	Args        *json.RawMessage `json:"args"`
	Env         *json.RawMessage `json:"env"`
	Config      *json.RawMessage `json:"config"`
}

// UpdateMCPServer updates an existing MCP server.
func (h *Handler) UpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.Queries.GetMCPServer(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "MCP server not found")
		return
	}

	workspaceID := uuidToString(existing.WorkspaceID)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "MCP server not found", "owner", "admin"); !ok {
		return
	}

	var req UpdateMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateMCPServerParams{
		ID: parseUUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Transport != nil {
		params.Transport = pgtype.Text{String: *req.Transport, Valid: true}
	}
	if req.URL != nil {
		params.Url = pgtype.Text{String: *req.URL, Valid: true}
	}
	if req.Command != nil {
		params.Command = pgtype.Text{String: *req.Command, Valid: true}
	}
	if req.Args != nil {
		params.Args = *req.Args
	}
	if req.Env != nil {
		params.Env = *req.Env
	}
	if req.Config != nil {
		params.Config = *req.Config
	}

	server, err := h.Queries.UpdateMCPServer(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update MCP server")
		return
	}

	writeJSON(w, http.StatusOK, mcpServerToResponse(server))
}

// DeleteMCPServer deletes an MCP server.
func (h *Handler) DeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.Queries.GetMCPServer(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "MCP server not found")
		return
	}

	workspaceID := uuidToString(existing.WorkspaceID)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "MCP server not found", "owner", "admin"); !ok {
		return
	}

	if err := h.Queries.DeleteMCPServer(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete MCP server")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
