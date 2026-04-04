package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type TeamResponse struct {
	ID          string              `json:"id"`
	WorkspaceID string              `json:"workspace_id"`
	Name        string              `json:"name"`
	Description *string             `json:"description"`
	AvatarURL   *string             `json:"avatar_url"`
	LeadAgentID *string             `json:"lead_agent_id"`
	LeadAgent   *AgentResponse      `json:"lead_agent,omitempty"`
	Members     []TeamMemberResponse `json:"members"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
	CreatedBy   *string             `json:"created_by"`
	ArchivedAt  *string            `json:"archived_at"`
	ArchivedBy  *string            `json:"archived_by"`
}

type TeamMemberResponse struct {
	ID       string          `json:"id"`
	TeamID   string          `json:"team_id"`
	AgentID  string          `json:"agent_id"`
	Agent    *AgentResponse  `json:"agent,omitempty"`
	Role     string          `json:"role"`
	JoinedAt string         `json:"joined_at"`
}

type CreateTeamRequest struct {
	Name            string   `json:"name"`
	Description    string   `json:"description"`
	AvatarURL      *string `json:"avatar_url"`
	LeadAgentID    string  `json:"lead_agent_id"`
	MemberAgentIDs []string `json:"member_agent_ids"`
}

type UpdateTeamRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	AvatarURL    *string `json:"avatar_url"`
	LeadAgentID  *string `json:"lead_agent_id"`
}

type AddTeamMemberRequest struct {
	AgentID string `json:"agent_id"`
}

func teamToResponse(t db.Team) TeamResponse {
	return TeamResponse{
		ID:          uuidToString(t.ID),
		WorkspaceID: uuidToString(t.WorkspaceID),
		Name:        t.Name,
		Description: textToPtr(t.Description),
		AvatarURL:   textToPtr(t.AvatarUrl),
		LeadAgentID: uuidToPtr(t.LeadAgentID),
		CreatedAt:   timestampToString(t.CreatedAt),
		UpdatedAt:   timestampToString(t.UpdatedAt),
		CreatedBy:   uuidToPtr(t.CreatedBy),
		ArchivedAt:  timestampToPtr(t.ArchivedAt),
		ArchivedBy:  uuidToPtr(t.ArchivedBy),
		Members:     []TeamMemberResponse{},
	}
}

func teamMemberToResponse(m db.TeamMember) TeamMemberResponse {
	return TeamMemberResponse{
		ID:       uuidToString(m.ID),
		TeamID:   uuidToString(m.TeamID),
		AgentID:  uuidToString(m.AgentID),
		Role:     m.Role,
		JoinedAt: timestampToString(m.JoinedAt),
	}
}

func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	teams, err := h.Queries.ListTeams(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}

	// Load members for all teams
	memberRows, err := h.Queries.ListTeamMembersByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load team members")
		return
	}
	memberMap := map[string][]TeamMemberResponse{}
	for _, m := range memberRows {
		teamID := uuidToString(m.TeamID)
		memberMap[teamID] = append(memberMap[teamID], teamMemberToResponse(m))
	}

	// Load lead agents
	leadIDs := []string{}
	for _, t := range teams {
		if t.LeadAgentID.Valid {
			leadIDs = append(leadIDs, uuidToString(t.LeadAgentID))
		}
	}
	leadAgentMap := map[string]AgentResponse{}
	for _, id := range leadIDs {
		agent, err := h.Queries.GetAgent(r.Context(), parseUUID(id))
		if err == nil {
			leadAgentMap[id] = agentToResponse(agent)
		}
	}

	visible := make([]TeamResponse, 0, len(teams))
	for _, t := range teams {
		resp := teamToResponse(t)
		if members, ok := memberMap[resp.ID]; ok {
			resp.Members = members
		}
		if leadAgentID := resp.LeadAgentID; leadAgentID != nil {
			if lead, ok := leadAgentMap[*leadAgentID]; ok {
				resp.LeadAgent = &lead
			}
		}
		visible = append(visible, resp)
	}

	writeJSON(w, http.StatusOK, visible)
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	team, err := h.Queries.GetTeam(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	members, err := h.Queries.ListTeamMembers(r.Context(), team.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load team members")
		return
	}

	resp := teamToResponse(team)
	resp.Members = make([]TeamMemberResponse, len(members))
	for i, m := range members {
		resp.Members[i] = teamMemberToResponse(m)
	}

	if team.LeadAgentID.Valid {
		leadID := uuidToString(team.LeadAgentID)
		agent, err := h.Queries.GetAgent(r.Context(), team.LeadAgentID)
		if err == nil {
			lead := agentToResponse(agent)
			resp.LeadAgent = &lead
			_ = leadID // silence unused variable warning
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var leadAgentID pgtype.UUID
	if req.LeadAgentID != "" {
		leadAgentID = parseUUID(req.LeadAgentID)
	}

	team, err := h.Queries.CreateTeam(r.Context(), db.CreateTeamParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
		Description: strToText(req.Description),
		AvatarUrl:   ptrToText(req.AvatarURL),
		LeadAgentID: leadAgentID,
		CreatedBy:   parseUUID(userID),
	})
	if err != nil {
		slog.Warn("create team failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create team: "+err.Error())
		return
	}

	// Add members
	for _, agentIDStr := range req.MemberAgentIDs {
		h.Queries.AddTeamMember(r.Context(), db.AddTeamMemberParams{
			TeamID:  team.ID,
			AgentID: parseUUID(agentIDStr),
			Role:    "member",
		})
	}

	// If lead agent is specified, update their role
	if req.LeadAgentID != "" {
		h.Queries.UpdateTeamMemberRole(r.Context(), db.UpdateTeamMemberRoleParams{
			TeamID:  team.ID,
			AgentID: parseUUID(req.LeadAgentID),
			Role:    "lead",
		})
	}

	slog.Info("team created", append(logger.RequestAttrs(r), "team_id", uuidToString(team.ID), "name", team.Name)...)
	h.publish(protocol.EventAgentCreated, workspaceID, "member", userID, map[string]any{"team": teamToResponse(team)})
	writeJSON(w, http.StatusCreated, teamToResponse(team))
}

func (h *Handler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	team, err := h.Queries.GetTeam(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	var req UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := db.UpdateTeamParams{
		ID: team.ID,
	}

	if req.Name != nil {
		updates.Name = *req.Name
	}
	if req.Description != nil {
		updates.Description = strToText(*req.Description)
	}
	if req.AvatarURL != nil {
		updates.AvatarUrl = ptrToText(*req.AvatarURL)
	}
	if req.LeadAgentID != nil {
		leadAgentID := parseUUID(*req.LeadAgentID)
		updates.LeadAgentID = &leadAgentID
	}

	updated, err := h.Queries.UpdateTeam(r.Context(), updates)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update team")
		return
	}

	writeJSON(w, http.StatusOK, teamToResponse(updated))
}

func (h *Handler) ArchiveTeam(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	team, err := h.Queries.GetTeam(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	archived, err := h.Queries.ArchiveTeam(r.Context(), db.ArchiveTeamParams{
		ID:         team.ID,
		ArchivedBy: parseUUIDPtr(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive team")
		return
	}

	writeJSON(w, http.StatusOK, teamToResponse(archived))
}

func (h *Handler) RestoreTeam(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	team, err := h.Queries.GetTeam(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	restored, err := h.Queries.RestoreTeam(r.Context(), team.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore team")
		return
	}

	writeJSON(w, http.StatusOK, teamToResponse(restored))
}

func (h *Handler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")

	var req AddTeamMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, err := h.Queries.AddTeamMember(r.Context(), db.AddTeamMemberParams{
		TeamID:  parseUUID(teamID),
		AgentID: parseUUID(req.AgentID),
		Role:    "member",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add team member")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	agentID := chi.URLParam(r, "agentId")

	err := h.Queries.RemoveTeamMember(r.Context(), db.RemoveTeamMemberParams{
		TeamID:  parseUUID(teamID),
		AgentID: parseUUID(agentID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove team member")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SetTeamLead(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")

	var req AddTeamMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update all members to "member" first
	members, _ := h.Queries.ListTeamMembers(r.Context(), parseUUID(teamID))
	for _, m := range members {
		h.Queries.UpdateTeamMemberRole(r.Context(), db.UpdateTeamMemberRoleParams{
			TeamID:  m.TeamID,
			AgentID: m.AgentID,
			Role:    "member",
		})
	}

	// Set new lead
	err := h.Queries.UpdateTeamMemberRole(r.Context(), db.UpdateTeamMemberRoleParams{
		TeamID:  parseUUID(teamID),
		AgentID: parseUUID(req.AgentID),
		Role:    "lead",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set team lead")
		return
	}

	// Update team lead_agent_id
	h.Queries.UpdateTeamLead(r.Context(), db.UpdateTeamLeadParams{
		ID:          parseUUID(teamID),
		LeadAgentID: parseUUID(req.AgentID),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
