package handler

import (
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// --- Agent Messaging ---

type SendMessageRequest struct {
	ToAgentID   string  `json:"to_agent_id"`
	Content     string  `json:"content"`
	MessageType string  `json:"message_type"`
	TaskID      *string `json:"task_id"`
	ReplyToID   *string `json:"reply_to_id"`
}

type AgentMessageResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	FromAgentID string  `json:"from_agent_id"`
	ToAgentID   string  `json:"to_agent_id"`
	TaskID      string  `json:"task_id,omitempty"`
	Content     string  `json:"content"`
	MessageType string  `json:"message_type"`
	ReplyToID   string  `json:"reply_to_id,omitempty"`
	ReadAt      *string `json:"read_at"`
	CreatedAt   string  `json:"created_at"`
}

func agentMessageToResponse(m db.AgentMessage) AgentMessageResponse {
	return AgentMessageResponse{
		ID:          uuidToString(m.ID),
		WorkspaceID: uuidToString(m.WorkspaceID),
		FromAgentID: uuidToString(m.FromAgentID),
		ToAgentID:   uuidToString(m.ToAgentID),
		TaskID:      uuidToString(m.TaskID),
		Content:     m.Content,
		MessageType: m.MessageType,
		ReplyToID:   uuidToString(m.ReplyToID),
		ReadAt:      timestampToPtr(m.ReadAt),
		CreatedAt:   timestampToString(m.CreatedAt),
	}
}

func (h *Handler) SendAgentMessage(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.loadAgentForUser(w, r, agentID); !ok {
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if req.ToAgentID == "" {
		writeError(w, http.StatusBadRequest, "to_agent_id is required")
		return
	}
	if req.MessageType == "" {
		req.MessageType = "info"
	}

	msg, err := h.CollaborationService.SendMessage(
		r.Context(),
		parseUUID(workspaceID),
		parseUUID(agentID),
		parseUUID(req.ToAgentID),
		req.Content,
		req.MessageType,
		optionalUUID(req.TaskID),
		optionalUUID(req.ReplyToID),
	)
	if err != nil {
		slog.Warn("send agent message failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to send message")
		return
	}

	resp := agentMessageToResponse(msg)
	h.publish(protocol.EventAgentMessage, workspaceID, "agent", agentID, map[string]any{"message": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) ListAgentMessages(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	if _, ok := h.loadAgentForUser(w, r, agentID); !ok {
		return
	}

	unreadOnly := r.URL.Query().Get("unread") == "true"

	var messages []db.AgentMessage
	var err error
	if taskID := r.URL.Query().Get("task_id"); taskID != "" {
		messages, err = h.Queries.ListAgentMessagesByTask(r.Context(), parseUUID(taskID))
	} else {
		messages, err = h.Queries.ListAgentMessagesForAgent(r.Context(), db.ListAgentMessagesForAgentParams{
			ToAgentID: parseUUID(agentID),
			CreatedAt: pgtype.Timestamptz{Time: time.Now().Add(-7 * 24 * time.Hour), Valid: true},
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}

	resp := make([]AgentMessageResponse, 0, len(messages))
	for _, m := range messages {
		if unreadOnly && m.ReadAt.Valid {
			continue
		}
		resp = append(resp, agentMessageToResponse(m))
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) MarkAgentMessagesRead(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	if _, ok := h.loadAgentForUser(w, r, agentID); !ok {
		return
	}

	if err := h.CollaborationService.MarkMessagesRead(r.Context(), parseUUID(agentID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark messages as read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Task Dependencies ---

type AddDependencyRequest struct {
	DependsOnTaskID string `json:"depends_on_task_id"`
}

type TaskDependencyResponse struct {
	TaskID      string `json:"task_id"`
	DependsOnID string `json:"depends_on_id"`
	CreatedAt   string `json:"created_at"`
}

func (h *Handler) AddTaskDependency(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	workspaceID := resolveWorkspaceID(r)

	var req AddDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DependsOnTaskID == "" {
		writeError(w, http.StatusBadRequest, "depends_on_task_id is required")
		return
	}

	dep, err := h.CollaborationService.AddDependency(
		r.Context(),
		parseUUID(workspaceID),
		parseUUID(taskID),
		parseUUID(req.DependsOnTaskID),
	)
	if err != nil {
		slog.Warn("add dependency failed", append(logger.RequestAttrs(r), "error", err, "task_id", taskID)...)
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, TaskDependencyResponse{
		TaskID:      uuidToString(dep.TaskID),
		DependsOnID: uuidToString(dep.DependsOnTaskID),
		CreatedAt:   timestampToString(dep.CreatedAt),
	})
}

func (h *Handler) RemoveTaskDependency(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	workspaceID := resolveWorkspaceID(r)

	var req AddDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DependsOnTaskID == "" {
		writeError(w, http.StatusBadRequest, "depends_on_task_id is required")
		return
	}

	if err := h.CollaborationService.RemoveDependency(
		r.Context(),
		parseUUID(workspaceID),
		parseUUID(taskID),
		parseUUID(req.DependsOnTaskID),
	); err != nil {
		slog.Warn("remove dependency failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to remove dependency")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ListTaskDependencies(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	deps, err := h.CollaborationService.GetDependencyInfo(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list dependencies")
		return
	}

	writeJSON(w, http.StatusOK, deps)
}

func (h *Handler) GetReadyTasks(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	tasks, err := h.CollaborationService.GetReadyTasks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get ready tasks")
		return
	}

	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t)
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Task Checkpoints ---

type SaveCheckpointRequest struct {
	Label        string            `json:"label"`
	State        map[string]any    `json:"state"`
	FilesChanged []string          `json:"files_changed"`
}

type CheckpointResponse struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	WorkspaceID  string `json:"workspace_id"`
	Label        string `json:"label"`
	State        any    `json:"state,omitempty"`
	FilesChanged any    `json:"files_changed,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func checkpointToResponse(cp db.TaskCheckpoint) CheckpointResponse {
	var state any
	json.Unmarshal(cp.State, &state)
	var files any
	json.Unmarshal(cp.FilesChanged, &files)
	return CheckpointResponse{
		ID:           uuidToString(cp.ID),
		TaskID:       uuidToString(cp.TaskID),
		WorkspaceID:  uuidToString(cp.WorkspaceID),
		Label:        cp.Label,
		State:        state,
		FilesChanged: files,
		CreatedAt:    timestampToString(cp.CreatedAt),
	}
}

func (h *Handler) SaveTaskCheckpoint(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	workspaceID := resolveWorkspaceID(r)

	var req SaveCheckpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Label == "" {
		writeError(w, http.StatusBadRequest, "label is required")
		return
	}

	cp, err := h.CollaborationService.SaveCheckpoint(
		r.Context(),
		parseUUID(workspaceID),
		parseUUID(taskID),
		req.Label,
		req.State,
		req.FilesChanged,
	)
	if err != nil {
		slog.Warn("save checkpoint failed", append(logger.RequestAttrs(r), "error", err, "task_id", taskID)...)
		writeError(w, http.StatusInternalServerError, "failed to save checkpoint")
		return
	}

	writeJSON(w, http.StatusCreated, checkpointToResponse(cp))
}

func (h *Handler) ListTaskCheckpoints(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	checkpoints, err := h.Queries.ListTaskCheckpoints(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list checkpoints")
		return
	}

	resp := make([]CheckpointResponse, len(checkpoints))
	for i, cp := range checkpoints {
		resp[i] = checkpointToResponse(cp)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetLatestCheckpoint(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	cp, err := h.CollaborationService.GetLatestCheckpoint(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "no checkpoint found")
		return
	}

	writeJSON(w, http.StatusOK, cp)
}

// --- Agent Memory ---

type StoreMemoryRequest struct {
	Content   string            `json:"content"`
	Embedding []float64         `json:"embedding"`
	Metadata  map[string]any    `json:"metadata"`
	ExpiresAt *string           `json:"expires_at"`
}

type MemoryResponse struct {
	ID          string  `json:"id"`
	AgentID     string  `json:"agent_id"`
	Content     string  `json:"content"`
	Metadata    any     `json:"metadata,omitempty"`
	Similarity  float64 `json:"similarity,omitempty"`
	CreatedAt   string  `json:"created_at"`
	ExpiresAt   *string `json:"expires_at"`
}

func memoryToResponse(m db.AgentMemory) MemoryResponse {
	var meta any
	json.Unmarshal(m.Metadata, &meta)
	resp := MemoryResponse{
		ID:         uuidToString(m.ID),
		AgentID:    uuidToString(m.AgentID),
		Content:    m.Content,
		Metadata:   meta,
		Similarity: m.Similarity,
		CreatedAt:  timestampToString(m.CreatedAt),
	}
	if m.ExpiresAt.Valid {
		s := m.ExpiresAt.Time.Format(time.RFC3339)
		resp.ExpiresAt = &s
	}
	return resp
}

func (h *Handler) StoreAgentMemory(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.loadAgentForUser(w, r, agentID); !ok {
		return
	}

	var req StoreMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	var embeddingBytes []byte
	if len(req.Embedding) > 0 {
		embeddingBytes = float64SliceToVector(req.Embedding)
	}

	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err == nil {
			expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	mem, err := h.CollaborationService.StoreMemory(
		r.Context(),
		parseUUID(workspaceID),
		parseUUID(agentID),
		req.Content,
		embeddingBytes,
		req.Metadata,
		expiresAt,
	)
	if err != nil {
		slog.Warn("store memory failed", append(logger.RequestAttrs(r), "error", err, "agent_id", agentID)...)
		writeError(w, http.StatusInternalServerError, "failed to store memory")
		return
	}

	writeJSON(w, http.StatusCreated, memoryToResponse(mem))
}

type RecallMemoryRequest struct {
	Embedding []float64 `json:"embedding"`
	Limit     int32     `json:"limit"`
}

func (h *Handler) RecallAgentMemory(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	if _, ok := h.loadAgentForUser(w, r, agentID); !ok {
		return
	}

	var req RecallMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Embedding) == 0 {
		writeError(w, http.StatusBadRequest, "embedding is required")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	memories, err := h.CollaborationService.RecallMemory(
		r.Context(),
		parseUUID(agentID),
		float64SliceToVector(req.Embedding),
		req.Limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to recall memories")
		return
	}

	resp := make([]MemoryResponse, len(memories))
	for i, m := range memories {
		resp[i] = memoryToResponse(m)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListAgentMemory(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	if _, ok := h.loadAgentForUser(w, r, agentID); !ok {
		return
	}

	memories, err := h.Queries.ListAgentMemory(r.Context(), db.ListAgentMemoryParams{
		AgentID: parseUUID(agentID),
		Limit:   50,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list memories")
		return
	}

	resp := make([]MemoryResponse, len(memories))
	for i, m := range memories {
		resp[i] = memoryToResponse(m)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteAgentMemory(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	if err := h.Queries.DeleteAgentMemory(r.Context(), parseUUID(memoryID)); err != nil {
		slog.Warn("delete memory failed", append(logger.RequestAttrs(r), "error", err, "memory_id", memoryID)...)
		writeError(w, http.StatusInternalServerError, "failed to delete memory")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RecallWorkspaceMemory searches across all agents in a workspace.
type RecallWorkspaceMemoryRequest struct {
	Embedding []float64 `json:"embedding"`
	Limit     int32     `json:"limit"`
}

func (h *Handler) RecallWorkspaceMemory(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	var req RecallWorkspaceMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Embedding) == 0 {
		writeError(w, http.StatusBadRequest, "embedding is required")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	memories, err := h.CollaborationService.RecallWorkspaceMemory(
		r.Context(),
		parseUUID(workspaceID),
		float64SliceToVector(req.Embedding),
		req.Limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to recall workspace memories")
		return
	}

	resp := make([]MemoryResponse, len(memories))
	for i, m := range memories {
		resp[i] = memoryToResponse(m)
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Helpers ---

func optionalUUID(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	return parseUUID(*s)
}

func float64SliceToVector(vals []float64) []byte {
	b := make([]byte, 0, len(vals)*4)
	for _, v := range vals {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], math.Float32bits(float32(v)))
		b = append(b, buf[:]...)
	}
	return b
}
