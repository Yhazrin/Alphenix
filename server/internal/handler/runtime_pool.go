package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// Runtime Join Token
// ---------------------------------------------------------------------------

type CreateRuntimeJoinTokenRequest struct {
	ExpiresInMinutes int `json:"expires_in_minutes"`
}

type CreateRuntimeJoinTokenResponse struct {
	Token       string `json:"token"`
	TokenPrefix string `json:"token_prefix"`
	ExpiresAt   string `json:"expires_at"`
}

func (h *Handler) CreateRuntimeJoinToken(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req CreateRuntimeJoinTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = CreateRuntimeJoinTokenRequest{}
	}

	expiresInMinutes := req.ExpiresInMinutes
	if expiresInMinutes <= 0 {
		expiresInMinutes = 30
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	rawToken := generateRandomToken(32)
	tokenHash := hashToken(rawToken)
	tokenPrefix := rawToken[:8]
	expiresAt := time.Now().Add(time.Duration(expiresInMinutes) * time.Minute)

	metadata, _ := json.Marshal(map[string]any{
		"created_by": userID,
	})

	var token db.RuntimeJoinToken
	err := h.DB.QueryRow(r.Context(), `
		INSERT INTO runtime_join_token (workspace_id, created_by, token_hash, token_prefix, expires_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, created_by, token_hash, token_prefix, expires_at, used_at, created_at, metadata
	`, parseUUID(workspaceID), parseUUID(userID), tokenHash, tokenPrefix, expiresAt, metadata).Scan(
		&token.ID, &token.WorkspaceID, &token.CreatedBy, &token.TokenHash, &token.TokenPrefix,
		&token.ExpiresAt, &token.UsedAt, &token.CreatedAt, &token.Metadata,
	)
	if err != nil {
		slog.Error("failed to create runtime join token", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, CreateRuntimeJoinTokenResponse{
		Token:       rawToken,
		TokenPrefix: tokenPrefix,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) ListRuntimeJoinTokens(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT id, token_prefix, expires_at, used_at, created_at
		FROM runtime_join_token
		WHERE workspace_id = $1
		ORDER BY created_at DESC
	`, parseUUID(workspaceID))
	if err != nil {
		slog.Error("failed to list runtime join tokens", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	defer rows.Close()

	var tokens []struct {
		ID          string `json:"id"`
		TokenPrefix string `json:"token_prefix"`
		ExpiresAt   string `json:"expires_at"`
		UsedAt      string `json:"used_at"`
		CreatedAt   string `json:"created_at"`
	}
	for rows.Next() {
		var t struct {
			ID          pgtype.UUID
			TokenPrefix string
			ExpiresAt   pgtype.Timestamptz
			UsedAt      pgtype.Timestamptz
			CreatedAt   pgtype.Timestamptz
		}
		if err := rows.Scan(&t.ID, &t.TokenPrefix, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt); err != nil {
			continue
		}
		tokens = append(tokens, struct {
			ID          string `json:"id"`
			TokenPrefix string `json:"token_prefix"`
			ExpiresAt   string `json:"expires_at"`
			UsedAt      string `json:"used_at"`
			CreatedAt   string `json:"created_at"`
		}{
			ID:          uuidToString(t.ID),
			TokenPrefix: t.TokenPrefix,
			ExpiresAt:   t.ExpiresAt.Time.Format(time.RFC3339),
			UsedAt:      nullTimeToString(t.UsedAt),
			CreatedAt:   t.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, tokens)
}

// ---------------------------------------------------------------------------
// Daemon Registration with Join Token
// ---------------------------------------------------------------------------

type RegisterRuntimeWithJoinTokenRequest struct {
	JoinToken   string         `json:"join_token"`
	DaemonID    string         `json:"daemon_id"`
	InstanceID  string         `json:"instance_id"`
	Name        string         `json:"name"`
	Provider    string         `json:"provider"`
	RuntimeMode string         `json:"runtime_mode"`
	DeviceInfo  string         `json:"device_info"`
	Metadata    map[string]any `json:"metadata"`
}

type RegisterRuntimeWithJoinTokenResponse struct {
	RuntimeID      string `json:"runtime_id"`
	ApprovalStatus string `json:"approval_status"`
	Status        string `json:"status"`
}

func (h *Handler) RegisterRuntimeWithJoinToken(w http.ResponseWriter, r *http.Request) {
	var req RegisterRuntimeWithJoinTokenRequest
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.JoinToken = strings.TrimSpace(req.JoinToken)
	req.DaemonID = strings.TrimSpace(req.DaemonID)
	req.InstanceID = strings.TrimSpace(req.InstanceID)
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	req.RuntimeMode = strings.TrimSpace(req.RuntimeMode)
	req.DeviceInfo = strings.TrimSpace(req.DeviceInfo)

	if req.JoinToken == "" {
		writeError(w, http.StatusBadRequest, "join_token is required")
		return
	}
	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	if req.InstanceID == "" {
		req.InstanceID = req.DaemonID
	}
	if req.Provider == "" {
		req.Provider = "unknown"
	}
	if req.Name == "" {
		req.Name = req.Provider
		if req.DeviceInfo != "" {
			req.Name = fmt.Sprintf("%s (%s)", req.Provider, req.DeviceInfo)
		}
	}
	if req.RuntimeMode == "" {
		req.RuntimeMode = "local_daemon"
	}

	tokenHash := hashToken(req.JoinToken)
	var token db.RuntimeJoinToken
	err := h.DB.QueryRow(r.Context(), `
		SELECT id, workspace_id, created_by, expires_at, used_at
		FROM runtime_join_token
		WHERE token_hash = $1
	`, tokenHash).Scan(&token.ID, &token.WorkspaceID, &token.CreatedBy, &token.ExpiresAt, &token.UsedAt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired join token")
		return
	}

	if token.UsedAt.Valid {
		writeError(w, http.StatusUnauthorized, "join token has already been used")
		return
	}
	if time.Now().After(token.ExpiresAt.Time) {
		writeError(w, http.StatusUnauthorized, "join token has expired")
		return
	}

	var metadata []byte
	if req.Metadata != nil {
		metadata, err = json.Marshal(req.Metadata)
		if err != nil {
			metadata, _ = json.Marshal(map[string]any{})
		}
	} else {
		metadata, _ = json.Marshal(map[string]any{})
	}

	var runtime db.AgentRuntime
	err = h.DB.QueryRow(r.Context(), `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, instance_id, name, runtime_mode, provider,
			status, device_info, metadata, owner_user_id, approval_status, visibility, trust_level, tags,
			last_seen_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'online', $7, $8, $9, 'pending', 'workspace', 'trusted_member', $10, now())
		ON CONFLICT (workspace_id, daemon_id, instance_id, provider)
			DO UPDATE SET name = EXCLUDED.name, status = 'online', last_seen_at = now()
		RETURNING id, workspace_id, daemon_id, instance_id, name, runtime_mode, provider, status, device_info,
			metadata, last_seen_at, created_at, updated_at, owner_user_id, approval_status, visibility,
			trust_level, drain_mode, paused, tags, max_concurrent_tasks_override, last_claimed_at,
			success_count_24h, failure_count_24h, avg_task_duration_ms
	`, token.WorkspaceID, req.DaemonID, req.InstanceID, req.Name, req.RuntimeMode, req.Provider,
		req.DeviceInfo, metadata, token.CreatedBy, metadata).Scan(
		&runtime.ID, &runtime.WorkspaceID, &runtime.DaemonID, &runtime.InstanceID, &runtime.Name,
		&runtime.RuntimeMode, &runtime.Provider, &runtime.Status, &runtime.DeviceInfo, &runtime.Metadata,
		&runtime.LastSeenAt, &runtime.CreatedAt, &runtime.UpdatedAt, &runtime.OwnerUserID, &runtime.ApprovalStatus,
		&runtime.Visibility, &runtime.TrustLevel, &runtime.DrainMode, &runtime.Paused, &runtime.Tags,
		&runtime.MaxConcurrentTasksOverride, &runtime.LastClaimedAt, &runtime.SuccessCount24h,
		&runtime.FailureCount24h, &runtime.AvgTaskDurationMs,
	)
	if err != nil {
		slog.Error("failed to create runtime", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to register runtime")
		return
	}

	// Mark token as used
	if _, err := h.DB.Exec(r.Context(), `UPDATE runtime_join_token SET used_at = now() WHERE id = $1`, token.ID); err != nil {
		slog.Warn("failed to mark runtime join token as used", "token_id", token.ID.String(), "error", err)
	}

	// Create audit log
	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO runtime_audit_log (workspace_id, runtime_id, actor_user_id, action, details)
		VALUES ($1, $2, $3, 'runtime_join_requested', $4)
	`, token.WorkspaceID, runtime.ID, token.CreatedBy, metadata); err != nil {
		slog.Warn("failed to create runtime audit log", "runtime_id", runtime.ID.String(), "action", "runtime_join_requested", "error", err)
	}

	writeJSON(w, http.StatusOK, RegisterRuntimeWithJoinTokenResponse{
		RuntimeID:      uuidToString(runtime.ID),
		ApprovalStatus: runtime.ApprovalStatus,
		Status:         runtime.Status,
	})
}

// ---------------------------------------------------------------------------
// Runtime Management
// ---------------------------------------------------------------------------

func (h *Handler) ApproveRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	workspaceID := r.Header.Get("X-Workspace-ID")
	userID := r.Header.Get("X-User-ID")

	var runtime db.AgentRuntime
	err := h.DB.QueryRow(r.Context(), `
		UPDATE agent_runtime
		SET approval_status = 'approved', updated_at = now()
		WHERE id = $1
		RETURNING id, workspace_id, daemon_id, instance_id, name, runtime_mode, provider, status, device_info,
			metadata, last_seen_at, created_at, updated_at, owner_user_id, approval_status, visibility,
			trust_level, drain_mode, paused, tags, max_concurrent_tasks_override, last_claimed_at,
			success_count_24h, failure_count_24h, avg_task_duration_ms
	`, parseUUID(runtimeID)).Scan(
		&runtime.ID, &runtime.WorkspaceID, &runtime.DaemonID, &runtime.InstanceID, &runtime.Name,
		&runtime.RuntimeMode, &runtime.Provider, &runtime.Status, &runtime.DeviceInfo, &runtime.Metadata,
		&runtime.LastSeenAt, &runtime.CreatedAt, &runtime.UpdatedAt, &runtime.OwnerUserID, &runtime.ApprovalStatus,
		&runtime.Visibility, &runtime.TrustLevel, &runtime.DrainMode, &runtime.Paused, &runtime.Tags,
		&runtime.MaxConcurrentTasksOverride, &runtime.LastClaimedAt, &runtime.SuccessCount24h,
		&runtime.FailureCount24h, &runtime.AvgTaskDurationMs,
	)
	if err != nil {
		slog.Error("failed to approve runtime", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to approve runtime")
		return
	}

	details, _ := json.Marshal(map[string]any{"approved_by": userID})
	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO runtime_audit_log (workspace_id, runtime_id, actor_user_id, action, details)
		VALUES ($1, $2, $3, 'runtime_approved', $4)
	`, parseUUID(workspaceID), runtime.ID, parseOptionalUUID(userID), details); err != nil {
		slog.Warn("failed to create runtime audit log", "runtime_id", runtime.ID.String(), "action", "runtime_approved", "error", err)
	}

	writeJSON(w, http.StatusOK, runtime)
}

func (h *Handler) RejectRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	workspaceID := r.Header.Get("X-Workspace-ID")
	userID := r.Header.Get("X-User-ID")

	var runtime db.AgentRuntime
	err := h.DB.QueryRow(r.Context(), `
		UPDATE agent_runtime
		SET approval_status = 'rejected', updated_at = now()
		WHERE id = $1
		RETURNING id, workspace_id, daemon_id, instance_id, name, runtime_mode, provider, status, device_info,
			metadata, last_seen_at, created_at, updated_at, owner_user_id, approval_status, visibility,
			trust_level, drain_mode, paused, tags, max_concurrent_tasks_override, last_claimed_at,
			success_count_24h, failure_count_24h, avg_task_duration_ms
	`, parseUUID(runtimeID)).Scan(
		&runtime.ID, &runtime.WorkspaceID, &runtime.DaemonID, &runtime.InstanceID, &runtime.Name,
		&runtime.RuntimeMode, &runtime.Provider, &runtime.Status, &runtime.DeviceInfo, &runtime.Metadata,
		&runtime.LastSeenAt, &runtime.CreatedAt, &runtime.UpdatedAt, &runtime.OwnerUserID, &runtime.ApprovalStatus,
		&runtime.Visibility, &runtime.TrustLevel, &runtime.DrainMode, &runtime.Paused, &runtime.Tags,
		&runtime.MaxConcurrentTasksOverride, &runtime.LastClaimedAt, &runtime.SuccessCount24h,
		&runtime.FailureCount24h, &runtime.AvgTaskDurationMs,
	)
	if err != nil {
		slog.Error("failed to reject runtime", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to reject runtime")
		return
	}

	details, _ := json.Marshal(map[string]any{"rejected_by": userID})
	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO runtime_audit_log (workspace_id, runtime_id, actor_user_id, action, details)
		VALUES ($1, $2, $3, 'runtime_rejected', $4)
	`, parseUUID(workspaceID), runtime.ID, parseOptionalUUID(userID), details); err != nil {
		slog.Warn("failed to create runtime audit log", "runtime_id", runtime.ID.String(), "action", "runtime_rejected", "error", err)
	}

	writeJSON(w, http.StatusOK, runtime)
}

func (h *Handler) PauseRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	workspaceID := r.Header.Get("X-Workspace-ID")
	userID := r.Header.Get("X-User-ID")

	var runtime db.AgentRuntime
	err := h.DB.QueryRow(r.Context(), `
		UPDATE agent_runtime
		SET paused = true, updated_at = now()
		WHERE id = $1
		RETURNING id, workspace_id, daemon_id, instance_id, name, runtime_mode, provider, status, device_info,
			metadata, last_seen_at, created_at, updated_at, owner_user_id, approval_status, visibility,
			trust_level, drain_mode, paused, tags, max_concurrent_tasks_override, last_claimed_at,
			success_count_24h, failure_count_24h, avg_task_duration_ms
	`, parseUUID(runtimeID)).Scan(
		&runtime.ID, &runtime.WorkspaceID, &runtime.DaemonID, &runtime.InstanceID, &runtime.Name,
		&runtime.RuntimeMode, &runtime.Provider, &runtime.Status, &runtime.DeviceInfo, &runtime.Metadata,
		&runtime.LastSeenAt, &runtime.CreatedAt, &runtime.UpdatedAt, &runtime.OwnerUserID, &runtime.ApprovalStatus,
		&runtime.Visibility, &runtime.TrustLevel, &runtime.DrainMode, &runtime.Paused, &runtime.Tags,
		&runtime.MaxConcurrentTasksOverride, &runtime.LastClaimedAt, &runtime.SuccessCount24h,
		&runtime.FailureCount24h, &runtime.AvgTaskDurationMs,
	)
	if err != nil {
		slog.Error("failed to pause runtime", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to pause runtime")
		return
	}

	details, _ := json.Marshal(map[string]any{"paused_by": userID})
	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO runtime_audit_log (workspace_id, runtime_id, actor_user_id, action, details)
		VALUES ($1, $2, $3, 'runtime_paused', $4)
	`, parseUUID(workspaceID), runtime.ID, parseOptionalUUID(userID), details); err != nil {
		slog.Warn("failed to create runtime audit log", "runtime_id", runtime.ID.String(), "action", "runtime_paused", "error", err)
	}

	writeJSON(w, http.StatusOK, runtime)
}

func (h *Handler) ResumeRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	workspaceID := r.Header.Get("X-Workspace-ID")
	userID := r.Header.Get("X-User-ID")

	var runtime db.AgentRuntime
	err := h.DB.QueryRow(r.Context(), `
		UPDATE agent_runtime
		SET paused = false, updated_at = now()
		WHERE id = $1
		RETURNING id, workspace_id, daemon_id, instance_id, name, runtime_mode, provider, status, device_info,
			metadata, last_seen_at, created_at, updated_at, owner_user_id, approval_status, visibility,
			trust_level, drain_mode, paused, tags, max_concurrent_tasks_override, last_claimed_at,
			success_count_24h, failure_count_24h, avg_task_duration_ms
	`, parseUUID(runtimeID)).Scan(
		&runtime.ID, &runtime.WorkspaceID, &runtime.DaemonID, &runtime.InstanceID, &runtime.Name,
		&runtime.RuntimeMode, &runtime.Provider, &runtime.Status, &runtime.DeviceInfo, &runtime.Metadata,
		&runtime.LastSeenAt, &runtime.CreatedAt, &runtime.UpdatedAt, &runtime.OwnerUserID, &runtime.ApprovalStatus,
		&runtime.Visibility, &runtime.TrustLevel, &runtime.DrainMode, &runtime.Paused, &runtime.Tags,
		&runtime.MaxConcurrentTasksOverride, &runtime.LastClaimedAt, &runtime.SuccessCount24h,
		&runtime.FailureCount24h, &runtime.AvgTaskDurationMs,
	)
	if err != nil {
		slog.Error("failed to resume runtime", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to resume runtime")
		return
	}

	details, _ := json.Marshal(map[string]any{"resumed_by": userID})
	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO runtime_audit_log (workspace_id, runtime_id, actor_user_id, action, details)
		VALUES ($1, $2, $3, 'runtime_resumed', $4)
	`, parseUUID(workspaceID), runtime.ID, parseOptionalUUID(userID), details); err != nil {
		slog.Warn("failed to create runtime audit log", "runtime_id", runtime.ID.String(), "action", "runtime_resumed", "error", err)
	}

	writeJSON(w, http.StatusOK, runtime)
}

func (h *Handler) RevokeRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	workspaceID := r.Header.Get("X-Workspace-ID")
	userID := r.Header.Get("X-User-ID")

	var runtime db.AgentRuntime
	err := h.DB.QueryRow(r.Context(), `
		UPDATE agent_runtime
		SET approval_status = 'revoked', status = 'offline', updated_at = now()
		WHERE id = $1
		RETURNING id, workspace_id, daemon_id, instance_id, name, runtime_mode, provider, status, device_info,
			metadata, last_seen_at, created_at, updated_at, owner_user_id, approval_status, visibility,
			trust_level, drain_mode, paused, tags, max_concurrent_tasks_override, last_claimed_at,
			success_count_24h, failure_count_24h, avg_task_duration_ms
	`, parseUUID(runtimeID)).Scan(
		&runtime.ID, &runtime.WorkspaceID, &runtime.DaemonID, &runtime.InstanceID, &runtime.Name,
		&runtime.RuntimeMode, &runtime.Provider, &runtime.Status, &runtime.DeviceInfo, &runtime.Metadata,
		&runtime.LastSeenAt, &runtime.CreatedAt, &runtime.UpdatedAt, &runtime.OwnerUserID, &runtime.ApprovalStatus,
		&runtime.Visibility, &runtime.TrustLevel, &runtime.DrainMode, &runtime.Paused, &runtime.Tags,
		&runtime.MaxConcurrentTasksOverride, &runtime.LastClaimedAt, &runtime.SuccessCount24h,
		&runtime.FailureCount24h, &runtime.AvgTaskDurationMs,
	)
	if err != nil {
		slog.Error("failed to revoke runtime", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to revoke runtime")
		return
	}

	details, _ := json.Marshal(map[string]any{"revoked_by": userID})
	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO runtime_audit_log (workspace_id, runtime_id, actor_user_id, action, details)
		VALUES ($1, $2, $3, 'runtime_revoked', $4)
	`, parseUUID(workspaceID), runtime.ID, parseOptionalUUID(userID), details); err != nil {
		slog.Warn("failed to create runtime audit log", "runtime_id", runtime.ID.String(), "action", "runtime_revoked", "error", err)
	}

	writeJSON(w, http.StatusOK, runtime)
}

func (h *Handler) DrainRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	workspaceID := r.Header.Get("X-Workspace-ID")
	userID := r.Header.Get("X-User-ID")

	var runtime db.AgentRuntime
	err := h.DB.QueryRow(r.Context(), `
		UPDATE agent_runtime
		SET drain_mode = true, updated_at = now()
		WHERE id = $1
		RETURNING id, workspace_id, daemon_id, instance_id, name, runtime_mode, provider, status, device_info,
			metadata, last_seen_at, created_at, updated_at, owner_user_id, approval_status, visibility,
			trust_level, drain_mode, paused, tags, max_concurrent_tasks_override, last_claimed_at,
			success_count_24h, failure_count_24h, avg_task_duration_ms
	`, parseUUID(runtimeID)).Scan(
		&runtime.ID, &runtime.WorkspaceID, &runtime.DaemonID, &runtime.InstanceID, &runtime.Name,
		&runtime.RuntimeMode, &runtime.Provider, &runtime.Status, &runtime.DeviceInfo, &runtime.Metadata,
		&runtime.LastSeenAt, &runtime.CreatedAt, &runtime.UpdatedAt, &runtime.OwnerUserID, &runtime.ApprovalStatus,
		&runtime.Visibility, &runtime.TrustLevel, &runtime.DrainMode, &runtime.Paused, &runtime.Tags,
		&runtime.MaxConcurrentTasksOverride, &runtime.LastClaimedAt, &runtime.SuccessCount24h,
		&runtime.FailureCount24h, &runtime.AvgTaskDurationMs,
	)
	if err != nil {
		slog.Error("failed to drain runtime", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to drain runtime")
		return
	}

	details, _ := json.Marshal(map[string]any{"drained_by": userID})
	if _, err := h.DB.Exec(r.Context(), `
		INSERT INTO runtime_audit_log (workspace_id, runtime_id, actor_user_id, action, details)
		VALUES ($1, $2, $3, 'runtime_drained', $4)
	`, parseUUID(workspaceID), runtime.ID, parseOptionalUUID(userID), details); err != nil {
		slog.Warn("failed to create runtime audit log", "runtime_id", runtime.ID.String(), "action", "runtime_drained", "error", err)
	}

	writeJSON(w, http.StatusOK, runtime)
}

func (h *Handler) GetRuntimeAuditLogs(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT id, workspace_id, runtime_id, actor_user_id, action, details, created_at
		FROM runtime_audit_log
		WHERE runtime_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, parseUUID(runtimeID))
	if err != nil {
		slog.Error("failed to get runtime audit logs", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to get audit logs")
		return
	}
	defer rows.Close()

	var logs []struct {
		ID          string `json:"id"`
		WorkspaceID string `json:"workspace_id"`
		RuntimeID   string `json:"runtime_id"`
		ActorUserID string `json:"actor_user_id"`
		Action      string `json:"action"`
		Details     string `json:"details"`
		CreatedAt   string `json:"created_at"`
	}
	for rows.Next() {
		var log struct {
			ID          pgtype.UUID
			WorkspaceID pgtype.UUID
			RuntimeID   pgtype.UUID
			ActorUserID pgtype.UUID
			Action      string
			Details     []byte
			CreatedAt   pgtype.Timestamptz
		}
		if err := rows.Scan(&log.ID, &log.WorkspaceID, &log.RuntimeID, &log.ActorUserID, &log.Action, &log.Details, &log.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspace_id"`
			RuntimeID   string `json:"runtime_id"`
			ActorUserID string `json:"actor_user_id"`
			Action      string `json:"action"`
			Details     string `json:"details"`
			CreatedAt   string `json:"created_at"`
		}{
			ID:          uuidToString(log.ID),
			WorkspaceID: uuidToString(log.WorkspaceID),
			RuntimeID:   uuidToString(log.RuntimeID),
			ActorUserID: uuidToString(log.ActorUserID),
			Action:      log.Action,
			Details:     string(log.Details),
			CreatedAt:   log.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, logs)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	randBytes := make([]byte, length)
	if _, err := rand.Read(randBytes); err != nil {
		// Fall back to a less secure pseudo-random source only if crypto/rand fails.
		// This should virtually never happen.
		for i := range b {
			b[i] = charset[int(time.Now().UnixNano())%len(charset)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[randBytes[i]%byte(len(charset))]
	}
	return string(b)
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func nullTimeToString(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

func parseOptionalUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{Valid: false}
	}
	return parseUUID(s)
}
