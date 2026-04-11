package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/alphenix/server/internal/logger"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

var slugSanitize = regexp.MustCompile(`[^a-z0-9]+`)

// ChannelResponse is JSON for a channel.
type ChannelResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	IsDefault   bool    `json:"is_default"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func channelToResponse(c db.Channel) ChannelResponse {
	return ChannelResponse{
		ID:          uuidToString(c.ID),
		WorkspaceID: uuidToString(c.WorkspaceID),
		Name:        c.Name,
		Slug:        c.Slug,
		Description: textToPtr(c.Description),
		IsDefault:   c.IsDefault,
		CreatedAt:   timestampToString(c.CreatedAt),
		UpdatedAt:   timestampToString(c.UpdatedAt),
	}
}

func channelSlugFromName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugSanitize.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "channel"
	}
	return s
}

// SeedDefaultChannelForWorkspace creates General channel and participants for a new workspace.
func (h *Handler) SeedDefaultChannelForWorkspace(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, createdBy pgtype.UUID) error {
	ch, err := qtx.CreateChannel(ctx, db.CreateChannelParams{
		WorkspaceID: workspaceID,
		Name:        "General",
		Slug:        "general",
		Description: pgtype.Text{},
		IsDefault:   true,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return err
	}
	return qtx.SeedChannelParticipantsFromWorkspace(ctx, db.SeedChannelParticipantsFromWorkspaceParams{
		ChannelID:   ch.ID,
		WorkspaceID: workspaceID,
	})
}

// addParticipantToDefaultChannel links a new agent or team to the workspace default channel.
func (h *Handler) addParticipantToDefaultChannel(ctx context.Context, workspaceID pgtype.UUID, participantType string, participantID pgtype.UUID) {
	ch, err := h.Queries.GetDefaultChannelByWorkspace(ctx, workspaceID)
	if err != nil {
		return
	}
	_ = h.Queries.AddChannelParticipant(ctx, db.AddChannelParticipantParams{
		ChannelID:       ch.ID,
		ParticipantType: participantType,
		ParticipantID:   participantID,
	})
}

func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	list, err := h.Queries.ListChannelsByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}
	out := make([]ChannelResponse, len(list))
	for i := range list {
		out[i] = channelToResponse(list[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": out})
}

type CreateChannelRequest struct {
	Name        string  `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
}

func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	slug := channelSlugFromName(req.Name)
	if req.Slug != nil && strings.TrimSpace(*req.Slug) != "" {
		slug = channelSlugFromName(*req.Slug)
	}
	workspaceID := parseUUID(resolveWorkspaceID(r))
	ch, err := h.Queries.CreateChannel(r.Context(), db.CreateChannelParams{
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Slug:        slug,
		Description: ptrToText(req.Description),
		IsDefault:   false,
		CreatedBy:   parseUUID(userID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "channel slug already exists in this workspace")
			return
		}
		slog.Warn("create channel failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create channel")
		return
	}
	// Creator joins the channel as member.
	_ = h.Queries.AddChannelParticipant(r.Context(), db.AddChannelParticipantParams{
		ChannelID:        ch.ID,
		ParticipantType:  "member",
		ParticipantID:    parseUUID(userID),
	})
	slog.Info("channel created", append(logger.RequestAttrs(r), "channel_id", uuidToString(ch.ID))...)
	writeJSON(w, http.StatusCreated, channelToResponse(ch))
}

func (h *Handler) GetChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "channelId")
	workspaceID := resolveWorkspaceID(r)
	ch, err := h.Queries.GetChannelInWorkspace(r.Context(), db.GetChannelInWorkspaceParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	writeJSON(w, http.StatusOK, channelToResponse(ch))
}

type AddChannelParticipantRequest struct {
	ParticipantType string `json:"participant_type"`
	ParticipantID   string `json:"participant_id"`
}

func (h *Handler) AddChannelParticipant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "channelId")
	workspaceID := resolveWorkspaceID(r)
	ch, err := h.Queries.GetChannelInWorkspace(r.Context(), db.GetChannelInWorkspaceParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	var req AddChannelParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pt := strings.TrimSpace(req.ParticipantType)
	if pt != "member" && pt != "agent" && pt != "team" {
		writeError(w, http.StatusBadRequest, "participant_type must be member, agent, or team")
		return
	}
	pid := strings.TrimSpace(req.ParticipantID)
	if pid == "" {
		writeError(w, http.StatusBadRequest, "participant_id is required")
		return
	}
	// Basic workspace ownership checks.
	wsUUID := parseUUID(workspaceID)
	pidUUID := parseUUID(pid)
	switch pt {
	case "member":
		if _, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{
			UserID:      pidUUID,
			WorkspaceID: wsUUID,
		}); err != nil {
			writeError(w, http.StatusBadRequest, "user is not a member of this workspace")
			return
		}
	case "agent":
		if _, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
			ID:          pidUUID,
			WorkspaceID: wsUUID,
		}); err != nil {
			writeError(w, http.StatusBadRequest, "agent not found in this workspace")
			return
		}
	case "team":
		tm, err := h.Queries.GetTeam(r.Context(), pidUUID)
		if err != nil || uuidToString(tm.WorkspaceID) != workspaceID {
			writeError(w, http.StatusBadRequest, "team not found in this workspace")
			return
		}
	}
	if err := h.Queries.AddChannelParticipant(r.Context(), db.AddChannelParticipantParams{
		ChannelID:        ch.ID,
		ParticipantType:  pt,
		ParticipantID:    pidUUID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add participant")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveChannelParticipant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "channelId")
	pt := chi.URLParam(r, "participantType")
	pid := chi.URLParam(r, "participantId")
	workspaceID := resolveWorkspaceID(r)
	ch, err := h.Queries.GetChannelInWorkspace(r.Context(), db.GetChannelInWorkspaceParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	if pt != "member" && pt != "agent" && pt != "team" {
		writeError(w, http.StatusBadRequest, "invalid participant type")
		return
	}
	if err := h.Queries.RemoveChannelParticipant(r.Context(), db.RemoveChannelParticipantParams{
		ChannelID:        ch.ID,
		ParticipantType:  pt,
		ParticipantID:    parseUUID(pid),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove participant")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ChannelParticipantResponse struct {
	ParticipantType string `json:"participant_type"`
	ParticipantID   string `json:"participant_id"`
	CreatedAt       string `json:"created_at"`
}

func (h *Handler) ListChannelParticipants(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "channelId")
	workspaceID := resolveWorkspaceID(r)
	ch, err := h.Queries.GetChannelInWorkspace(r.Context(), db.GetChannelInWorkspaceParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	parts, err := h.Queries.ListChannelParticipants(r.Context(), ch.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list participants")
		return
	}
	out := make([]ChannelParticipantResponse, len(parts))
	for i, p := range parts {
		out[i] = ChannelParticipantResponse{
			ParticipantType: p.ParticipantType,
			ParticipantID:   uuidToString(p.ParticipantID),
			CreatedAt:       timestampToString(p.CreatedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"participants": out})
}
