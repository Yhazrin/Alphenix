package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/memory"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
	pgvector_go "github.com/pgvector/pgvector-go"
)

// CollaborationService orchestrates multi-agent collaboration patterns:
// inter-agent messaging, DAG task dependencies, shared context preparation,
// long-term memory, and task checkpoints.
type CollaborationService struct {
	Queries *db.Queries
	Hub     *realtime.Hub
	Bus     *events.Bus
}

func NewCollaborationService(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *CollaborationService {
	return &CollaborationService{Queries: q, Hub: hub, Bus: bus}
}

// --- Inter-Agent Messaging ---

// SendMessage sends a message from one agent to another, broadcasts via WS,
// and returns the created message.
func (s *CollaborationService) SendMessage(ctx context.Context, workspaceID, fromAgentID, toAgentID pgtype.UUID, content, messageType string, taskID, replyToID pgtype.UUID) (db.AgentMessage, error) {
	msg, err := s.Queries.CreateAgentMessage(ctx, db.CreateAgentMessageParams{
		WorkspaceID: workspaceID,
		FromAgentID: fromAgentID,
		ToAgentID:   toAgentID,
		TaskID:      taskID,
		Content:     content,
		Metadata:    []byte("{}"),
		MessageType: messageType,
		ReplyToID:   replyToID,
	})
	if err != nil {
		return db.AgentMessage{}, fmt.Errorf("create agent message: %w", err)
	}

	slog.Info("agent message sent",
		"message_id", util.UUIDToString(msg.ID),
		"from", util.UUIDToString(fromAgentID),
		"to", util.UUIDToString(toAgentID),
		"type", messageType,
	)

	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentMessage,
		WorkspaceID: util.UUIDToString(workspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(fromAgentID),
		Payload: protocol.AgentMessagePayload{
			MessageID:   util.UUIDToString(msg.ID),
			FromAgentID: util.UUIDToString(fromAgentID),
			ToAgentID:   util.UUIDToString(toAgentID),
			TaskID:      util.UUIDToString(taskID),
			Content:     content,
			MessageType: messageType,
			ReplyToID:   util.UUIDToString(replyToID),
		},
	})

	return msg, nil
}

// GetPendingMessages returns unread messages for an agent, suitable for
// injection into the agent's next prompt.
func (s *CollaborationService) GetPendingMessages(ctx context.Context, agentID pgtype.UUID) ([]db.AgentMessage, error) {
	messages, err := s.Queries.ListAgentMessagesForAgent(ctx, db.ListAgentMessagesForAgentParams{
		ToAgentID: agentID,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
	})
	if err != nil {
		return nil, err
	}

	var unread []db.AgentMessage
	for _, m := range messages {
		if !m.ReadAt.Valid {
			unread = append(unread, m)
		}
	}
	return unread, nil
}

// MarkMessagesRead marks all unread messages for an agent as read.
func (s *CollaborationService) MarkMessagesRead(ctx context.Context, agentID pgtype.UUID) error {
	return s.Queries.MarkAllAgentMessagesRead(ctx, agentID)
}

// --- DAG Task Dependencies ---

// AddDependency creates a task-level dependency: taskID cannot run until
// dependsOnTaskID completes. Returns error if this would create a cycle.
func (s *CollaborationService) AddDependency(ctx context.Context, workspaceID, taskID, dependsOnTaskID pgtype.UUID) (db.TaskDependency, error) {
	if taskID == dependsOnTaskID {
		return db.TaskDependency{}, fmt.Errorf("task cannot depend on itself")
	}

	// Cycle detection: check if dependsOnTaskID transitively depends on taskID.
	if s.wouldCreateCycle(ctx, taskID, dependsOnTaskID) {
		return db.TaskDependency{}, fmt.Errorf("dependency would create a cycle")
	}

	dep, err := s.Queries.CreateTaskDependency(ctx, db.CreateTaskDependencyParams{
		WorkspaceID:     workspaceID,
		TaskID:          taskID,
		DependsOnTaskID: dependsOnTaskID,
	})
	if err != nil {
		return db.TaskDependency{}, fmt.Errorf("create dependency: %w", err)
	}

	slog.Info("task dependency created",
		"task_id", util.UUIDToString(taskID),
		"depends_on", util.UUIDToString(dependsOnTaskID),
	)

	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDependencyCreated,
		WorkspaceID: util.UUIDToString(workspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskDependencyPayload{
			TaskID:          util.UUIDToString(taskID),
			DependsOnTaskID: util.UUIDToString(dependsOnTaskID),
		},
	})

	return dep, nil
}

// RemoveDependency removes a specific task dependency.
func (s *CollaborationService) RemoveDependency(ctx context.Context, workspaceID, taskID, dependsOnTaskID pgtype.UUID) error {
	err := s.Queries.DeleteTaskDependency(ctx, db.DeleteTaskDependencyParams{
		TaskID:          taskID,
		DependsOnTaskID: dependsOnTaskID,
	})
	if err != nil {
		return err
	}

	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDependencyDeleted,
		WorkspaceID: util.UUIDToString(workspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskDependencyPayload{
			TaskID:          util.UUIDToString(taskID),
			DependsOnTaskID: util.UUIDToString(dependsOnTaskID),
		},
	})
	return nil
}

// GetReadyTasks returns queued tasks whose dependencies are all satisfied.
func (s *CollaborationService) GetReadyTasks(ctx context.Context, agentID pgtype.UUID) ([]db.AgentTaskQueue, error) {
	return s.Queries.ListReadyTasks(ctx, agentID)
}

// GetDependencyInfo returns dependency details for prompt injection.
func (s *CollaborationService) GetDependencyInfo(ctx context.Context, taskID pgtype.UUID) ([]protocol.TaskDependencyInfo, error) {
	deps, err := s.Queries.GetTaskDependencies(ctx, taskID)
	if err != nil {
		return nil, err
	}

	result := make([]protocol.TaskDependencyInfo, 0, len(deps))
	for _, d := range deps {
		status := "pending"
		if depTask, err := s.Queries.GetAgentTask(ctx, d.DependsOnTaskID); err == nil {
			status = depTask.Status
		}
		result = append(result, protocol.TaskDependencyInfo{
			TaskID:           util.UUIDToString(d.TaskID),
			DependsOnID:      util.UUIDToString(d.DependsOnTaskID),
			DependencyStatus: status,
		})
	}
	return result, nil
}

// wouldCreateCycle checks if adding taskID <- dependsOnTaskID would create
// a transitive cycle in the DAG. Uses iterative DFS from dependsOnTaskID.
func (s *CollaborationService) wouldCreateCycle(ctx context.Context, taskID, dependsOnTaskID pgtype.UUID) bool {
	visited := map[string]struct{}{}
	queue := []pgtype.UUID{dependsOnTaskID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		currentStr := util.UUIDToString(current)
		if currentStr == util.UUIDToString(taskID) {
			return true
		}
		if _, seen := visited[currentStr]; seen {
			continue
		}
		visited[currentStr] = struct{}{}

		deps, err := s.Queries.GetTaskDependencies(ctx, current)
		if err != nil {
			continue
		}
		for _, d := range deps {
			queue = append(queue, d.DependsOnTaskID)
		}
	}
	return false
}

// --- Long-Term Memory ---

// StoreMemory persists an observation/pattern for an agent with an embedding.
func (s *CollaborationService) StoreMemory(ctx context.Context, workspaceID, agentID pgtype.UUID, content string, embedding pgvector_go.Vector, metadata map[string]any, expiresAt pgtype.Timestamptz) (db.AgentMemory, error) {
	metaBytes, _ := json.Marshal(metadata)
	if metaBytes == nil {
		metaBytes = []byte("{}")
	}

	mem, err := s.Queries.CreateAgentMemory(ctx, db.CreateAgentMemoryParams{
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		Content:     content,
		Embedding:   embedding,
		Metadata:    metaBytes,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return db.AgentMemory{}, fmt.Errorf("store memory: %w", err)
	}

	slog.Debug("memory stored",
		"memory_id", util.UUIDToString(mem.ID),
		"agent_id", util.UUIDToString(agentID),
		"content_len", len(content),
	)

	s.Bus.Publish(events.Event{
		Type:        protocol.EventMemoryStored,
		WorkspaceID: util.UUIDToString(workspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(agentID),
		Payload: map[string]any{
			"memory_id": util.UUIDToString(mem.ID),
			"agent_id":  util.UUIDToString(agentID),
			"content":   content,
		},
	})

	return mem, nil
}

// RecallMemory searches an agent's memories by embedding similarity.
func (s *CollaborationService) RecallMemory(ctx context.Context, agentID pgtype.UUID, embedding pgvector_go.Vector, limit int32) ([]memory.SearchResult, error) {
	rows, err := s.Queries.SearchAgentMemory(ctx, db.SearchAgentMemoryParams{
		Embedding: embedding,
		AgentID:   agentID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	return agentMemoryRowsToSearchResults(rows), nil
}

// RecallWorkspaceMemory searches across all agents in a workspace by embedding similarity.
func (s *CollaborationService) RecallWorkspaceMemory(ctx context.Context, workspaceID pgtype.UUID, embedding pgvector_go.Vector, limit int32) ([]memory.SearchResult, error) {
	rows, err := s.Queries.SearchWorkspaceMemory(ctx, db.SearchWorkspaceMemoryParams{
		Embedding:   embedding,
		WorkspaceID: workspaceID,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	return workspaceMemoryRowsToSearchResults(rows), nil
}

// RecentWorkspaceMemory returns the most recent memories across all agents in a workspace.
// Used as a fallback when no embedding is available (e.g. at task claim time).
func (s *CollaborationService) RecentWorkspaceMemory(ctx context.Context, workspaceID pgtype.UUID, limit int32) ([]memory.SearchResult, error) {
	rows, err := s.Queries.ListRecentWorkspaceMemory(ctx, db.ListRecentWorkspaceMemoryParams{
		WorkspaceID: workspaceID,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	return recentMemoryRowsToSearchResults(rows), nil
}

// CleanupExpiredMemory removes expired memory entries.
func (s *CollaborationService) CleanupExpiredMemory(ctx context.Context) error {
	return s.Queries.DeleteExpiredMemory(ctx)
}

// HybridRecallWorkspaceMemory combines BM25 full-text search and vector
// semantic search using Reciprocal Rank Fusion. queryText provides the BM25
// input (e.g. issue title + description), embedding provides the vector input.
// Falls back to vector-only, BM25-only, or recent-only if a channel is unavailable.
func (s *CollaborationService) HybridRecallWorkspaceMemory(ctx context.Context, workspaceID pgtype.UUID, queryText string, embedding pgvector_go.Vector, limit int32) ([]protocol.MemoryRecall, error) {
	candidateLimit := limit * 3 // fetch more candidates for fusion

	// BM25 channel.
	var bm25Results []memory.SearchResult
	if queryText != "" {
		expanded := memory.ExpandQuery(queryText)
		if expanded != "" {
			rows, err := s.Queries.SearchWorkspaceMemoryBM25(ctx, db.SearchWorkspaceMemoryBM25Params{
				SearchQuery: expanded,
				WorkspaceID: workspaceID,
				LimitCount:  candidateLimit,
			})
			if err == nil {
				for i, r := range rows {
					bm25Results = append(bm25Results, memory.SearchResult{
						Memory: bm25RowToAgentMemory(r),
						Score:  float64(r.Bm25Score),
						Rank:   i + 1,
					})
				}
			} else {
				slog.Debug("hybrid memory: BM25 search failed", "error", err)
			}
		}
	}

	// Vector channel.
	var vectorResults []memory.SearchResult
	if len(embedding.Slice()) > 0 {
		rows, err := s.Queries.SearchWorkspaceMemory(ctx, db.SearchWorkspaceMemoryParams{
			Embedding:   embedding,
			WorkspaceID: workspaceID,
			Limit:       candidateLimit,
		})
		if err == nil {
			vectorResults = memory.RankResults(workspaceMemoryRowsToSearchResults(rows))
		} else {
			slog.Debug("hybrid memory: vector search failed", "error", err)
		}
	}

	// Determine search type and fuse results.
	searchType := "recent"
	var fused []memory.FusedResult
	if len(bm25Results) > 0 && len(vectorResults) > 0 {
		bm25Results = memory.RankResults(bm25Results)
		fused = memory.RRFusion(bm25Results, vectorResults, int(limit))
		searchType = "hybrid"
	} else if len(vectorResults) > 0 {
		for _, v := range vectorResults {
			fused = append(fused, memory.FusedResult{
				Memory:      v.Memory,
				FusedScore:  v.Score,
				VectorScore: v.Score,
			})
		}
		if int(limit) < len(fused) {
			fused = fused[:limit]
		}
		searchType = "vector"
	} else if len(bm25Results) > 0 {
		for _, b := range bm25Results {
			fused = append(fused, memory.FusedResult{
				Memory:     b.Memory,
				FusedScore: b.Score,
				BM25Score:  b.Score,
			})
		}
		if int(limit) < len(fused) {
			fused = fused[:limit]
		}
		searchType = "bm25"
	}

	// Fallback to recent memories when no search channel produced results.
	if len(fused) == 0 {
		memories, err := s.Queries.ListRecentWorkspaceMemory(ctx, db.ListRecentWorkspaceMemoryParams{
			WorkspaceID: workspaceID,
			Limit:       limit,
		})
		if err != nil {
			return nil, err
		}
		recalls := make([]protocol.MemoryRecall, 0, len(memories))
		for _, m := range memories {
			recalls = append(recalls, protocol.MemoryRecall{
				ID:         util.UUIDToString(m.ID),
				Content:    m.Content,
				Similarity: 0,
				SearchType: "recent",
			})
		}
		return recalls, nil
	}

	// Convert fused results to protocol types, enriching with agent names.
	recalls := make([]protocol.MemoryRecall, 0, len(fused))
	for _, f := range fused {
		agentName := ""
		if a, err := s.Queries.GetAgent(ctx, f.Memory.AgentID); err == nil {
			agentName = a.Name
		}
		recalls = append(recalls, protocol.MemoryRecall{
			ID:         util.UUIDToString(f.Memory.ID),
			Content:    f.Memory.Content,
			Similarity: f.VectorScore,
			AgentName:  agentName,
			BM25Score:  f.BM25Score,
			FusedScore: f.FusedScore,
			SearchType: searchType,
		})
	}
	return recalls, nil
}

// agentMemoryRowsToSearchResults converts SearchAgentMemoryRow to SearchResult.
func agentMemoryRowsToSearchResults(rows []db.SearchAgentMemoryRow) []memory.SearchResult {
	results := make([]memory.SearchResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, memory.SearchResult{
			Memory: db.AgentMemory{
				ID:          r.ID,
				WorkspaceID: r.WorkspaceID,
				AgentID:     r.AgentID,
				Content:     r.Content,
				Embedding:   r.Embedding,
				Metadata:    r.Metadata,
				CreatedAt:   r.CreatedAt,
				ExpiresAt:   r.ExpiresAt,
				TsvContent:  r.TsvContent,
			},
			Score: float64(r.Similarity),
		})
	}
	return results
}

// workspaceMemoryRowsToSearchResults converts SearchWorkspaceMemoryRow to SearchResult.
func workspaceMemoryRowsToSearchResults(rows []db.SearchWorkspaceMemoryRow) []memory.SearchResult {
	results := make([]memory.SearchResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, memory.SearchResult{
			Memory: db.AgentMemory{
				ID:          r.ID,
				WorkspaceID: r.WorkspaceID,
				AgentID:     r.AgentID,
				Content:     r.Content,
				Embedding:   r.Embedding,
				Metadata:    r.Metadata,
				CreatedAt:   r.CreatedAt,
				ExpiresAt:   r.ExpiresAt,
				TsvContent:  r.TsvContent,
			},
			Score: float64(r.Similarity),
		})
	}
	return results
}

// recentMemoryRowsToSearchResults converts ListRecentWorkspaceMemoryRow to SearchResult.
func recentMemoryRowsToSearchResults(rows []db.ListRecentWorkspaceMemoryRow) []memory.SearchResult {
	results := make([]memory.SearchResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, memory.SearchResult{
			Memory: db.AgentMemory{
				ID:          r.ID,
				WorkspaceID: r.WorkspaceID,
				AgentID:     r.AgentID,
				Content:     r.Content,
				Embedding:   r.Embedding,
				Metadata:    r.Metadata,
				CreatedAt:   r.CreatedAt,
				ExpiresAt:   r.ExpiresAt,
				TsvContent:  r.TsvContent,
			},
			Score: r.Similarity,
		})
	}
	return results
}

// bm25RowToAgentMemory converts SearchWorkspaceMemoryBM25Row to AgentMemory.
func bm25RowToAgentMemory(r db.SearchWorkspaceMemoryBM25Row) db.AgentMemory {
	return db.AgentMemory{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		AgentID:     r.AgentID,
		Content:     r.Content,
		Embedding:   r.Embedding,
		Metadata:    r.Metadata,
		CreatedAt:   r.CreatedAt,
		ExpiresAt:   r.ExpiresAt,
		TsvContent:  r.TsvContent,
	}
}

// --- Task Checkpoints ---

// SaveCheckpoint persists an agent's intermediate execution state.
func (s *CollaborationService) SaveCheckpoint(ctx context.Context, workspaceID, taskID pgtype.UUID, label string, state map[string]any, filesChanged []string) (db.TaskCheckpoint, error) {
	stateBytes, _ := json.Marshal(state)
	if stateBytes == nil {
		stateBytes = []byte("{}")
	}
	filesBytes, _ := json.Marshal(filesChanged)
	if filesBytes == nil {
		filesBytes = []byte("[]")
	}

	cp, err := s.Queries.CreateTaskCheckpoint(ctx, db.CreateTaskCheckpointParams{
		TaskID:       taskID,
		WorkspaceID:  workspaceID,
		Label:        label,
		State:        stateBytes,
		FilesChanged: filesBytes,
	})
	if err != nil {
		return db.TaskCheckpoint{}, fmt.Errorf("save checkpoint: %w", err)
	}

	slog.Info("checkpoint saved",
		"checkpoint_id", util.UUIDToString(cp.ID),
		"task_id", util.UUIDToString(taskID),
		"label", label,
	)

	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskCheckpointCreated,
		WorkspaceID: util.UUIDToString(workspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskCheckpointPayload{
			CheckpointID: util.UUIDToString(cp.ID),
			TaskID:       util.UUIDToString(taskID),
			Label:        label,
		},
	})

	return cp, nil
}

// GetLatestCheckpoint returns the most recent checkpoint for a task.
func (s *CollaborationService) GetLatestCheckpoint(ctx context.Context, taskID pgtype.UUID) (*protocol.CheckpointInfo, error) {
	cp, err := s.Queries.GetLatestCheckpoint(ctx, taskID)
	if err != nil {
		return nil, err // pgx.ErrNoRows means no checkpoint
	}

	var state any
	json.Unmarshal(cp.State, &state)
	var files any
	json.Unmarshal(cp.FilesChanged, &files)

	return &protocol.CheckpointInfo{
		ID:           util.UUIDToString(cp.ID),
		Label:        cp.Label,
		State:        state,
		FilesChanged: files,
		CreatedAt:    cp.CreatedAt.Time.Format(time.RFC3339),
	}, nil
}

// --- Shared Context Preparation ---

// BuildSharedContext assembles the full collaborative context for an agent's
// prompt: colleagues, pending messages, dependencies, workspace memory, and
// the last checkpoint. queryText (e.g. issue title + description) enables
// hybrid BM25+vector memory search; pass empty string to fall back.
func (s *CollaborationService) BuildSharedContext(ctx context.Context, workspaceID, agentID, taskID pgtype.UUID, embedding pgvector_go.Vector, queryText string) (*protocol.SharedContext, error) {
	sc := &protocol.SharedContext{}

	// 1. Load colleagues (other active agents in workspace).
	agents, err := s.Queries.ListAgents(ctx, workspaceID)
	if err == nil {
		for _, a := range agents {
			if a.ID == agentID {
				continue
			}
			sc.Colleagues = append(sc.Colleagues, protocol.ColleagueInfo{
				ID:          util.UUIDToString(a.ID),
				Name:        a.Name,
				Description: a.Description,
				Status:      a.Status,
			})
		}
	}

	// 2. Load pending inter-agent messages.
	messages, err := s.GetPendingMessages(ctx, agentID)
	if err == nil {
		for _, m := range messages {
			sc.PendingMessages = append(sc.PendingMessages, protocol.AgentMessagePayload{
				MessageID:   util.UUIDToString(m.ID),
				FromAgentID: util.UUIDToString(m.FromAgentID),
				ToAgentID:   util.UUIDToString(m.ToAgentID),
				TaskID:      util.UUIDToString(m.TaskID),
				Content:     m.Content,
				MessageType: m.MessageType,
				ReplyToID:   util.UUIDToString(m.ReplyToID),
			})
		}
	}

	// 3. Load task dependencies.
	if taskID.Valid {
		deps, err := s.GetDependencyInfo(ctx, taskID)
		if err == nil {
			sc.Dependencies = deps
		}

		// 4. Load latest checkpoint.
		cp, err := s.GetLatestCheckpoint(ctx, taskID)
		if err == nil && cp != nil {
			sc.LastCheckpoint = cp
		}
	}

	// 5. Hybrid recall: BM25 + vector + RRF, falling back to recent.
	recalls, err := s.HybridRecallWorkspaceMemory(ctx, workspaceID, queryText, embedding, 5)
	if err == nil {
		sc.WorkspaceMemory = recalls
	}

	return sc, nil
}
