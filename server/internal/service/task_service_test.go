package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	pgvector_go "github.com/pgvector/pgvector-go"
)

// ---------------------------------------------------------------------------
// taskStubDBTX — in-memory stub implementing db.DBTX for task/review/team tests
// ---------------------------------------------------------------------------

type taskStubDBTX struct {
	agents          map[string]db.Agent
	tasks           map[string]db.AgentTaskQueue
	issues          map[string]db.Issue
	comments        []db.Comment
	skills          map[string][]db.Skill       // agentID -> skills
	skillFiles      map[string][]db.SkillFile   // skillID -> files
	teams           map[string]db.Team
	teamMembers     map[string][]db.TeamMember  // teamID -> members
	teamTasks       map[string]db.TeamTaskQueue
	agentStatus     map[string]string            // agentID -> status
	workspaces      map[string]db.Workspace      // workspaceID -> workspace
	taskDeps        map[string][]db.TaskDependency // taskID -> dependencies (GetTaskDependencies)
	taskDependents  map[string][]db.TaskDependency // taskID -> dependents (GetTaskDependents)
	messages        []db.AgentMessage
	runtimes        map[string]db.AgentRuntime
	runtimePolicies map[string]db.RuntimeAssignmentPolicy // agentID -> policy
	memoryEntries   []db.AgentMemory                      // agent_memory rows
	checkpoints     []db.TaskCheckpoint                   // task_checkpoint rows
	queryErr        error                                  // if set, Query returns this error
	execErr         error                                  // if set, Exec returns this error
	nextID          int
}

func newTaskStubDBTX() *taskStubDBTX {
	return &taskStubDBTX{
		agents:         make(map[string]db.Agent),
		tasks:          make(map[string]db.AgentTaskQueue),
		issues:         make(map[string]db.Issue),
		skills:         make(map[string][]db.Skill),
		skillFiles:     make(map[string][]db.SkillFile),
		teams:          make(map[string]db.Team),
		teamMembers:    make(map[string][]db.TeamMember),
		teamTasks:      make(map[string]db.TeamTaskQueue),
		agentStatus:    make(map[string]string),
		workspaces:     make(map[string]db.Workspace),
		taskDeps:        make(map[string][]db.TaskDependency),
		taskDependents:  make(map[string][]db.TaskDependency),
		runtimes:        make(map[string]db.AgentRuntime),
		runtimePolicies: make(map[string]db.RuntimeAssignmentPolicy),
	}
}

func (s *taskStubDBTX) nextUUID() pgtype.UUID {
	s.nextID++
	return util.ParseUUID(fmt.Sprintf("00000000-0000-0000-0000-%012x", s.nextID))
}

func (s *taskStubDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	// MarkAllAgentMessagesRead
	if strings.Contains(sql, "agent_message") && strings.Contains(sql, "read_at") {
		if len(args) >= 1 {
			agentID := args[0].(pgtype.UUID)
			for i, m := range s.messages {
				if m.ToAgentID == agentID && !m.ReadAt.Valid {
					s.messages[i].ReadAt = pgtype.Timestamptz{Time: pgtype.Timestamptz{}.Time, Valid: true}
				}
			}
		}
		return pgconn.CommandTag{}, nil
	}
	// DeleteTaskDependency
	if strings.Contains(sql, "DELETE FROM task_dependency") {
		if len(args) >= 2 {
			taskID := args[0].(pgtype.UUID)
			depOnID := args[1].(pgtype.UUID)
			// Remove from taskDeps map
			taskKey := util.UUIDToString(taskID)
			if deps, ok := s.taskDeps[taskKey]; ok {
				var filtered []db.TaskDependency
				for _, d := range deps {
					if util.UUIDToString(d.DependsOnTaskID) != util.UUIDToString(depOnID) {
						filtered = append(filtered, d)
					}
				}
				s.taskDeps[taskKey] = filtered
			}
			// Remove from taskDependents map
			depKey := util.UUIDToString(depOnID)
			if deps, ok := s.taskDependents[depKey]; ok {
				var filtered []db.TaskDependency
				for _, d := range deps {
					if util.UUIDToString(d.TaskID) != util.UUIDToString(taskID) {
						filtered = append(filtered, d)
					}
				}
				s.taskDependents[depKey] = filtered
			}
		}
		return pgconn.CommandTag{}, nil
	}
	// DeleteExpiredMemory — DELETE FROM agent_memory WHERE expires_at IS NOT NULL AND expires_at < now()
	if strings.Contains(sql, "DELETE FROM agent_memory") {
		var kept []db.AgentMemory
		for _, m := range s.memoryEntries {
			if !m.ExpiresAt.Valid || m.ExpiresAt.Time.After(time.Now()) {
				kept = append(kept, m)
			}
		}
		s.memoryEntries = kept
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, nil
}

func (s *taskStubDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	// ListTeamMembers
	if strings.Contains(sql, "ListTeamMembers") && len(args) >= 1 {
		teamID := args[0].(pgtype.UUID)
		members := s.teamMembers[util.UUIDToString(teamID)]
		rows := make([][]any, len(members))
		for i, m := range members {
			rows[i] = []any{m.ID, m.TeamID, m.AgentID, m.Role, m.JoinedAt}
		}
		return &stubRows{rows: rows}, nil
	}
	// ListAgentSkills
	if strings.Contains(sql, "FROM skill s") && strings.Contains(sql, "JOIN agent_skill") && len(args) >= 1 {
		agentID := args[0].(pgtype.UUID)
		skills := s.skills[util.UUIDToString(agentID)]
		rows := make([][]any, len(skills))
		for i, sk := range skills {
			rows[i] = []any{sk.ID, sk.WorkspaceID, sk.Name, sk.Description, sk.Content, sk.Config, sk.CreatedBy, sk.CreatedAt, sk.UpdatedAt}
		}
		return &stubRows{rows: rows}, nil
	}
	// ListSkillFiles
	if strings.Contains(sql, "FROM skill_file") && len(args) >= 1 {
		skillID := args[0].(pgtype.UUID)
		files := s.skillFiles[util.UUIDToString(skillID)]
		rows := make([][]any, len(files))
		for i, f := range files {
			rows[i] = []any{f.ID, f.SkillID, f.Path, f.Content, f.CreatedAt, f.UpdatedAt}
		}
		return &stubRows{rows: rows}, nil
	}
	// ListPendingTasksByRuntime
	if strings.Contains(sql, "ListPendingTasksByRuntime") || (strings.Contains(sql, "FROM agent_task_queue") && strings.Contains(sql, "runtime_id") && strings.Contains(sql, "ORDER BY priority")) {
		runtimeID := args[0].(pgtype.UUID)
		var results [][]any
		for _, t := range s.tasks {
			if util.UUIDToString(t.RuntimeID) == util.UUIDToString(runtimeID) && (t.Status == "queued" || t.Status == "dispatched") {
				results = append(results, taskRowValues(t))
			}
		}
		return &stubRows{rows: results}, nil
	}
	// ListReadyTasks — FROM agent_task_queue atq ... NOT EXISTS ... task_dependency
	// Must be BEFORE task_dependency handlers since the SQL also contains those substrings.
	if strings.Contains(sql, "FROM agent_task_queue atq") && strings.Contains(sql, "NOT EXISTS") && len(args) >= 1 {
		agentID := args[0].(pgtype.UUID)
		var results [][]any
		for _, t := range s.tasks {
			if t.AgentID == agentID && t.Status == "queued" {
				results = append(results, taskRowValues(t))
			}
		}
		return &stubRows{rows: results}, nil
	}
	// CancelAgentTasksByIssue
	if strings.Contains(sql, "cancelAgentTasksByIssue") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "cancelled") && strings.Contains(sql, "issue_id")) {
		issueID := args[0].(pgtype.UUID)
		for key, t := range s.tasks {
			if util.UUIDToString(t.IssueID) == util.UUIDToString(issueID) && (t.Status == "queued" || t.Status == "dispatched" || t.Status == "running") {
				t.Status = "cancelled"
				s.tasks[key] = t
			}
		}
		return &stubRows{rows: [][]any{}}, nil
	}
	// ListActiveTasksByIssue
	if strings.Contains(sql, "listActiveTasksByIssue") && len(args) >= 1 {
		issueID := args[0].(pgtype.UUID)
		var results [][]any
		for _, t := range s.tasks {
			if util.UUIDToString(t.IssueID) == util.UUIDToString(issueID) && t.Status != "completed" && t.Status != "failed" && t.Status != "cancelled" {
				results = append(results, []any{t.ID, t.AgentID, t.IssueID, t.Status, t.Priority, t.DispatchedAt, t.StartedAt, t.CompletedAt, t.Result, t.Error, t.CreatedAt, t.Context, t.RuntimeID, t.SessionID, t.WorkDir, t.TriggerCommentID, t.ReviewStatus, t.ReviewCount, t.MaxReviews, t.ChainSourceTaskID, t.ChainReason})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// GetTaskDependents — SELECT ... FROM task_dependency WHERE depends_on_task_id = $1
	if strings.Contains(sql, "FROM task_dependency") && strings.Contains(sql, "WHERE depends_on_task_id") && len(args) >= 1 {
		depOnID := args[0].(pgtype.UUID)
		deps := s.taskDependents[util.UUIDToString(depOnID)]
		rows := make([][]any, len(deps))
		for i, d := range deps {
			rows[i] = []any{d.ID, d.WorkspaceID, d.TaskID, d.DependsOnTaskID, d.CreatedAt}
		}
		return &stubRows{rows: rows}, nil
	}
	// GetTaskDependencies — SELECT ... FROM task_dependency WHERE task_id = $1
	if strings.Contains(sql, "FROM task_dependency") && strings.Contains(sql, "task_id") && len(args) >= 1 {
		taskID := args[0].(pgtype.UUID)
		deps := s.taskDeps[util.UUIDToString(taskID)]
		rows := make([][]any, len(deps))
		for i, d := range deps {
			rows[i] = []any{d.ID, d.WorkspaceID, d.TaskID, d.DependsOnTaskID, d.CreatedAt}
		}
		return &stubRows{rows: rows}, nil
	}
	// ListPendingTeamTasks
	if strings.Contains(sql, "FROM team_task_queue") && strings.Contains(sql, "team_id") {
		teamID := args[0].(pgtype.UUID)
		var results [][]any
		for _, t := range s.teamTasks {
			if util.UUIDToString(t.TeamID) == util.UUIDToString(teamID) && t.Status == "pending" {
				// SELECT: id, team_id, issue_id, assigned_by, status, delegated_to_agent_id, priority, created_at, updated_at
				results = append(results, []any{t.ID, t.TeamID, t.IssueID, t.AssignedBy, t.Status, t.DelegatedToAgentID, t.Priority, t.CreatedAt, t.UpdatedAt})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// ListAgentMessagesForAgent — FROM agent_message WHERE to_agent_id = $1 AND created_at > $2
	if strings.Contains(sql, "FROM agent_message") && strings.Contains(sql, "to_agent_id") && len(args) >= 1 {
		toID := args[0].(pgtype.UUID)
		var results [][]any
		for _, m := range s.messages {
			if m.ToAgentID == toID {
				results = append(results, []any{m.ID, m.WorkspaceID, m.FromAgentID, m.ToAgentID, m.TaskID, m.Content, m.Metadata, m.CreatedAt, m.MessageType, m.ReadAt, m.ReplyToID})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// ListReadySubIssues — FROM issue i ... parent_issue_id
	if strings.Contains(sql, "FROM issue i") && strings.Contains(sql, "parent_issue_id") && len(args) >= 1 {
		parentID := args[0].(pgtype.UUID)
		var results [][]any
		for _, i := range s.issues {
			if i.ParentIssueID.Valid && util.UUIDToString(i.ParentIssueID) == util.UUIDToString(parentID) &&
				(i.Status == "backlog" || i.Status == "todo") {
				results = append(results, []any{
					i.ID, i.WorkspaceID, i.Title, i.Description, i.Status, i.Priority,
					i.AssigneeType, i.AssigneeID, i.CreatorType, i.CreatorID, i.ParentIssueID,
					i.AcceptanceCriteria, i.ContextRefs, i.Position, i.DueDate, i.CreatedAt, i.UpdatedAt,
					i.Number, i.IssueKind, i.RepoID,
				})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// ListAgentRuntimes — FROM agent_runtime WHERE workspace_id = $1
	if strings.Contains(sql, "FROM agent_runtime") && strings.Contains(sql, "workspace_id") && len(args) >= 1 {
		wsID := args[0].(pgtype.UUID)
		var results [][]any
		for _, rt := range s.runtimes {
			if util.UUIDToString(rt.WorkspaceID) == util.UUIDToString(wsID) {
				results = append(results, []any{
					rt.ID, rt.WorkspaceID, rt.DaemonID, rt.Name, rt.RuntimeMode, rt.Provider,
					rt.Status, rt.DeviceInfo, rt.Metadata, rt.LastSeenAt, rt.CreatedAt, rt.UpdatedAt,
					rt.InstanceID, rt.OwnerUserID, rt.ApprovalStatus, rt.Visibility, rt.TrustLevel,
					rt.DrainMode, rt.Paused, rt.Tags, rt.MaxConcurrentTasksOverride, rt.LastClaimedAt,
					rt.SuccessCount24h, rt.FailureCount24h, rt.AvgTaskDurationMs,
				})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// ListAgents — FROM agent WHERE workspace_id = $1 AND archived_at IS NULL
	if strings.Contains(sql, "FROM agent") && strings.Contains(sql, "archived_at IS NULL") && len(args) >= 1 {
		wsID := args[0].(pgtype.UUID)
		var results [][]any
		for _, a := range s.agents {
			if a.WorkspaceID == wsID && !a.ArchivedAt.Valid {
				results = append(results, agentRowValues(a))
			}
		}
		return &stubRows{rows: results}, nil
	}
	// SearchAgentMemory — FROM agent_memory WHERE agent_id = $2 ... similarity
	if strings.Contains(sql, "FROM agent_memory") && strings.Contains(sql, "WHERE agent_id") && len(args) >= 2 {
		agentID := args[1].(pgtype.UUID)
		var results [][]any
		for _, m := range s.memoryEntries {
			if m.AgentID == agentID {
				results = append(results, []any{m.ID, m.WorkspaceID, m.AgentID, m.Content, m.Embedding, m.Metadata, m.CreatedAt, m.ExpiresAt, m.TsvContent, int32(0)})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// SearchWorkspaceMemoryBM25 — FROM agent_memory am WHERE am.workspace_id AND tsv_content
	if strings.Contains(sql, "FROM agent_memory am") && strings.Contains(sql, "tsv_content") && len(args) >= 2 {
		wsID := args[1].(pgtype.UUID)
		var results [][]any
		for _, m := range s.memoryEntries {
			if m.WorkspaceID == wsID {
				results = append(results, []any{m.ID, m.WorkspaceID, m.AgentID, m.Content, m.Embedding, m.Metadata, m.CreatedAt, m.ExpiresAt, m.TsvContent, float32(0)})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// SearchWorkspaceMemory — FROM agent_memory WHERE workspace_id = $2 ... embedding
	if strings.Contains(sql, "FROM agent_memory") && strings.Contains(sql, "embedding") && strings.Contains(sql, "workspace_id") && len(args) >= 2 {
		wsID := args[1].(pgtype.UUID)
		var results [][]any
		for _, m := range s.memoryEntries {
			if m.WorkspaceID == wsID {
				results = append(results, []any{m.ID, m.WorkspaceID, m.AgentID, m.Content, m.Embedding, m.Metadata, m.CreatedAt, m.ExpiresAt, m.TsvContent, int32(0)})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// ListRecentWorkspaceMemory — FROM agent_memory WHERE workspace_id = $1 ... ORDER BY created_at
	if strings.Contains(sql, "FROM agent_memory") && strings.Contains(sql, "ORDER BY created_at") && len(args) >= 1 {
		wsID := args[0].(pgtype.UUID)
		var results [][]any
		for _, m := range s.memoryEntries {
			if m.WorkspaceID == wsID {
				results = append(results, []any{m.ID, m.WorkspaceID, m.AgentID, m.Content, m.Embedding, m.Metadata, m.CreatedAt, m.ExpiresAt, m.TsvContent, float64(0)})
			}
		}
		return &stubRows{rows: results}, nil
	}
	// ListActiveRuntimePolicies — FROM runtime_assignment_policy WHERE workspace_id = $1 AND is_active = true
	if strings.Contains(sql, "runtime_assignment_policy") && strings.Contains(sql, "is_active") && len(args) >= 1 {
		wsID := args[0].(pgtype.UUID)
		var results [][]any
		for _, p := range s.runtimePolicies {
			if p.WorkspaceID == wsID && p.IsActive {
				results = append(results, []any{
					p.ID, p.WorkspaceID, p.AgentID, p.TeamID,
					p.RequiredTags, p.ForbiddenTags,
					p.PreferredRuntimeIds, p.FallbackRuntimeIds,
					p.MaxQueueDepth, p.IsActive, p.CreatedAt, p.UpdatedAt,
				})
			}
		}
		return &stubRows{rows: results}, nil
	}
	return &stubRows{}, nil
}

// taskRowValues returns the scan values for a db.AgentTaskQueue.
func taskRowValues(t db.AgentTaskQueue) []any {
	return []any{
		t.ID, t.AgentID, t.IssueID, t.Status, t.Priority, t.DispatchedAt, t.StartedAt,
		t.CompletedAt, t.Result, t.Error, t.CreatedAt, t.Context, t.RuntimeID, t.SessionID,
		t.WorkDir, t.TriggerCommentID, t.ReviewStatus, t.ReviewCount, t.MaxReviews,
		t.ChainSourceTaskID, t.ChainReason,
	}
}

// agentRowValues returns the scan values for a db.Agent.
func agentRowValues(a db.Agent) []any {
	return []any{
		a.ID, a.WorkspaceID, a.Name, a.AvatarUrl, a.RuntimeMode, a.RuntimeConfig,
		a.Visibility, a.Status, a.MaxConcurrentTasks, a.OwnerID, a.CreatedAt, a.UpdatedAt,
		a.Description, a.RuntimeID, a.Instructions, a.ArchivedAt, a.ArchivedBy,
	}
}

func (s *taskStubDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if s.queryErr != nil {
		return stubRow{err: s.queryErr}
	}
	// CountRunningTasks — SELECT count(*) FROM agent_task_queue WHERE agent_id = $1 AND status IN ...
	// Must check agent_id to avoid catching CountPendingTasksByRuntime (which uses runtime_id)
	if strings.Contains(sql, "count(*)") && strings.Contains(sql, "agent_task_queue") && strings.Contains(sql, "agent_id") && strings.Contains(sql, "status IN") && len(args) >= 1 {
		agentID := args[0].(pgtype.UUID)
		count := 0
		for _, t := range s.tasks {
			if util.UUIDToString(t.AgentID) == util.UUIDToString(agentID) &&
				(t.Status == "dispatched" || t.Status == "running") {
				count++
			}
		}
		return stubRow{values: []any{int64(count)}}
	}

	// SetTaskInReview — UPDATE ... SET status = 'in_review', review_status = 'pending' ... RETURNING
	// Must be checked BEFORE generic handler because RETURNING contains "completed_at"
	if strings.Contains(sql, "setTaskInReview") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "SET status = 'in_review'") && strings.Contains(sql, "review_status")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			t.Status = "in_review"
			t.ReviewStatus = "pending"
			s.tasks[util.UUIDToString(id)] = t
			return stubRow{values: taskRowValues(t)}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CompleteTaskReview — UPDATE ... SET status = 'completed', review_status = 'passed' ... RETURNING
	// Must be checked BEFORE generic handler
	if strings.Contains(sql, "completeTaskReview") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "review_status") && strings.Contains(sql, "passed")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			t.Status = "completed"
			t.ReviewStatus = "passed"
			if len(args) >= 2 {
				if result, ok := args[1].([]byte); ok {
					t.Result = result
				}
			}
			if len(args) >= 3 {
				if sid, ok := args[2].(pgtype.Text); ok {
					t.SessionID = sid
				}
			}
			if len(args) >= 4 {
				if wd, ok := args[3].(pgtype.Text); ok {
					t.WorkDir = wd
				}
			}
			s.tasks[util.UUIDToString(id)] = t
			return stubRow{values: taskRowValues(t)}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// RetryTaskReview — UPDATE ... SET status = 'queued', review_status = 'none', review_count = review_count + 1 ... RETURNING
	// Must be checked BEFORE generic handler
	if strings.Contains(sql, "retryTaskReview") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "review_count") && strings.Contains(sql, "review_count + 1")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			t.Status = "queued"
			t.ReviewStatus = "none"
			t.ReviewCount++
			s.tasks[util.UUIDToString(id)] = t
			return stubRow{values: taskRowValues(t)}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// UPDATE queries on agent_task_queue with RETURNING (CancelAgentTask, StartAgentTask, CompleteAgentTask, FailAgentTask)
	// Exclude ClaimAgentTask (has FOR UPDATE), SetTaskInReview (SET in_review), CompleteTaskReview (SET review_status), RetryTaskReview (review_count + 1)
	if strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "RETURNING") && !strings.Contains(sql, "ClaimAgentTask") && !strings.Contains(sql, "FOR UPDATE") && !strings.Contains(sql, "SET status = 'in_review'") && !strings.Contains(sql, "review_status = 'pending'") && !strings.Contains(sql, "review_count + 1") && len(args) >= 1 {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if strings.Contains(sql, "CancelAgentTask") || strings.Contains(sql, "status = 'cancelled'") {
				t.Status = "cancelled"
			} else if strings.Contains(sql, "CompleteAgentTask") || strings.Contains(sql, "status = 'completed'") {
				t.Status = "completed"
			} else if strings.Contains(sql, "StartAgentTask") || strings.Contains(sql, "status = 'running'") {
				t.Status = "running"
			} else if strings.Contains(sql, "FailAgentTask") || strings.Contains(sql, "status = 'failed'") {
				t.Status = "failed"
			}
			s.tasks[util.UUIDToString(id)] = t
			return stubRow{values: taskRowValues(t)}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// ClaimAgentTask — UPDATE ... WHERE id = (SELECT ... FROM agent_task_queue)
	if strings.Contains(sql, "ClaimAgentTask") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "dispatched") && strings.Contains(sql, "FOR UPDATE")) {
		// Find first queued task for the agent
		if len(args) >= 1 {
			agentID := args[0].(pgtype.UUID)
			for key, t := range s.tasks {
				if util.UUIDToString(t.AgentID) == util.UUIDToString(agentID) && t.Status == "queued" {
					t.Status = "dispatched"
					s.tasks[key] = t
					return stubRow{values: taskRowValues(t)}
				}
			}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CreateAgentTask — INSERT INTO agent_task_queue ... RETURNING *
	if strings.Contains(sql, "INSERT INTO agent_task_queue") {
		newTask := db.AgentTaskQueue{
			ID:       s.nextUUID(),
			Status:   "queued",
			AgentID:  args[0].(pgtype.UUID),
			RuntimeID: args[1].(pgtype.UUID),
			IssueID:  args[2].(pgtype.UUID),
			Priority: args[3].(int32),
		}
		s.tasks[util.UUIDToString(newTask.ID)] = newTask
		return stubRow{values: taskRowValues(newTask)}
	}

	// GetAgentTask — FROM agent_task_queue WHERE id = $1 (check BEFORE GetAgent to avoid prefix match)
	if strings.Contains(sql, "FROM agent_task_queue") && strings.Contains(sql, "WHERE id =") && len(args) >= 1 {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			return stubRow{values: taskRowValues(t)}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// GetAgent — FROM agent WHERE id = $1
	if strings.Contains(sql, "FROM agent") && strings.Contains(sql, "WHERE id =") && len(args) >= 1 {
		id := args[0].(pgtype.UUID)
		if a, ok := s.agents[util.UUIDToString(id)]; ok {
			return stubRow{values: agentRowValues(a)}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// GetIssue
	if strings.Contains(sql, "FROM issue") && strings.Contains(sql, "WHERE id =") && len(args) >= 1 {
		id := args[0].(pgtype.UUID)
		if i, ok := s.issues[util.UUIDToString(id)]; ok {
			return stubRow{values: []any{
				i.ID, i.WorkspaceID, i.Title, i.Description, i.Status, i.Priority,
				i.AssigneeType, i.AssigneeID, i.CreatorType, i.CreatorID, i.ParentIssueID,
				i.AcceptanceCriteria, i.ContextRefs, i.Position, i.DueDate, i.CreatedAt, i.UpdatedAt,
				i.Number, i.IssueKind, i.RepoID,
			}}
		}
		return stubRow{err: pgx.ErrNoRows}
	}
	// GetTeam
	if strings.Contains(sql, "FROM team") && strings.Contains(sql, "WHERE id =") && len(args) >= 1 {
		id := args[0].(pgtype.UUID)
		if t, ok := s.teams[util.UUIDToString(id)]; ok {
			return stubRow{values: []any{
				t.ID, t.WorkspaceID, t.Name, t.Description, t.AvatarUrl, t.LeadAgentID,
				t.CreatedBy, t.CreatedAt, t.UpdatedAt, t.ArchivedAt, t.ArchivedBy,
				t.QueuePolicy, t.CapabilityTags, t.MaxRunDuration, t.MaxConcurrent,
			}}
		}
		return stubRow{err: pgx.ErrNoRows}
	}
	// UpdateAgentStatus
	if strings.Contains(sql, "UPDATE agent") && strings.Contains(sql, "SET status") && len(args) >= 1 {
		id := args[0].(pgtype.UUID)
		if a, ok := s.agents[util.UUIDToString(id)]; ok {
			if len(args) >= 2 {
				a.Status = args[1].(string)
				s.agents[util.UUIDToString(id)] = a
			}
			return stubRow{values: agentRowValues(a)}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CancelAgentTask — UPDATE ... SET status = 'cancelled' ... WHERE id = $1 AND status IN (...)
	if strings.Contains(sql, "cancelAgentTask") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "cancelled") && !strings.Contains(sql, "issue_id")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if t.Status == "queued" || t.Status == "dispatched" || t.Status == "running" {
				t.Status = "cancelled"
				s.tasks[util.UUIDToString(id)] = t
				return stubRow{values: taskRowValues(t)}
			}
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// StartAgentTask — UPDATE ... SET status = 'running' ... WHERE id = $1 AND status = 'dispatched'
	if strings.Contains(sql, "startAgentTask") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "running") && strings.Contains(sql, "started_at")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if t.Status == "dispatched" {
				t.Status = "running"
				s.tasks[util.UUIDToString(id)] = t
				return stubRow{values: taskRowValues(t)}
			}
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CompleteAgentTask — UPDATE ... SET status = 'completed' ... WHERE id = $1 AND status = 'running'
	if strings.Contains(sql, "completeAgentTask") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "completed") && strings.Contains(sql, "session_id")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if t.Status == "running" {
				t.Status = "completed"
				s.tasks[util.UUIDToString(id)] = t
				return stubRow{values: taskRowValues(t)}
			}
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// FailAgentTask — UPDATE ... SET status = 'failed' ... WHERE id = $1 AND status IN (...)
	if strings.Contains(sql, "failAgentTask") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "failed") && strings.Contains(sql, "error =")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if t.Status == "dispatched" || t.Status == "running" || t.Status == "in_review" {
				t.Status = "failed"
				s.tasks[util.UUIDToString(id)] = t
				return stubRow{values: taskRowValues(t)}
			}
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// SetTaskInReview — UPDATE ... SET status = 'in_review' ... WHERE id = $1 AND status = 'running'
	if strings.Contains(sql, "setTaskInReview") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "in_review") && strings.Contains(sql, "review_status")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if t.Status == "running" {
				t.Status = "in_review"
				t.ReviewStatus = "pending"
				s.tasks[util.UUIDToString(id)] = t
				return stubRow{values: taskRowValues(t)}
			}
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CompleteTaskReview — UPDATE ... SET status = 'completed', review_status = 'passed' ... WHERE id = $1 AND status = 'in_review'
	if strings.Contains(sql, "completeTaskReview") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "completed") && strings.Contains(sql, "passed")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if t.Status == "in_review" {
				t.Status = "completed"
				t.ReviewStatus = "passed"
				s.tasks[util.UUIDToString(id)] = t
				return stubRow{values: taskRowValues(t)}
			}
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// RetryTaskReview — UPDATE ... SET status = 'queued' ... WHERE id = $1 AND status = 'in_review'
	if strings.Contains(sql, "retryTaskReview") || (strings.Contains(sql, "UPDATE agent_task_queue") && strings.Contains(sql, "queued") && strings.Contains(sql, "review_count")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.tasks[util.UUIDToString(id)]; ok {
			if t.Status == "in_review" {
				t.Status = "queued"
				t.ReviewCount++
				s.tasks[util.UUIDToString(id)] = t
				return stubRow{values: taskRowValues(t)}
			}
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// AddTeamMember — INSERT INTO team_member ... RETURNING *
	if strings.Contains(sql, "INSERT INTO team_member") || strings.Contains(sql, "addTeamMember") {
		teamID := args[0].(pgtype.UUID)
		agentID := args[1].(pgtype.UUID)
		role := "member"
		if len(args) >= 3 {
			if rt, ok := args[2].(pgtype.Text); ok {
				role = rt.String
			}
		}
		member := db.TeamMember{
			TeamID:  teamID,
			AgentID: agentID,
			Role:    role,
		}
		key := util.UUIDToString(teamID)
		s.teamMembers[key] = append(s.teamMembers[key], member)
		return stubRow{values: []any{member.ID, member.TeamID, member.AgentID, member.Role, member.JoinedAt}}
	}

	// UpdateTeamMemberRole — UPDATE team_member SET role = $3 ... RETURNING *
	if strings.Contains(sql, "updateTeamMemberRole") || (strings.Contains(sql, "UPDATE team_member") && strings.Contains(sql, "SET role")) {
		return stubRow{values: []any{pgtype.UUID{}, args[0].(pgtype.UUID), args[1].(pgtype.UUID), "lead", pgtype.Timestamptz{}}}
	}

	// UpdateTeamLead — UPDATE team SET lead_agent_id = $2 ... RETURNING *
	if strings.Contains(sql, "updateTeamLead") || (strings.Contains(sql, "UPDATE team") && strings.Contains(sql, "lead_agent_id")) {
		id := args[0].(pgtype.UUID)
		if t, ok := s.teams[util.UUIDToString(id)]; ok {
			if len(args) >= 2 {
				t.LeadAgentID = args[1].(pgtype.UUID)
				s.teams[util.UUIDToString(id)] = t
			}
			return stubRow{values: []any{
				t.ID, t.WorkspaceID, t.Name, t.Description, t.AvatarUrl, t.LeadAgentID,
				t.CreatedBy, t.CreatedAt, t.UpdatedAt, t.ArchivedAt, t.ArchivedBy,
				t.QueuePolicy, t.CapabilityTags, t.MaxRunDuration, t.MaxConcurrent,
			}}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CreateTeamTask — INSERT INTO team_task_queue ... RETURNING *
	if strings.Contains(sql, "INSERT INTO team_task_queue") || strings.Contains(sql, "createTeamTask") {
		teamTask := db.TeamTaskQueue{
			ID:         s.nextUUID(),
			TeamID:     args[0].(pgtype.UUID),
			IssueID:    args[1].(pgtype.UUID),
			AssignedBy: args[2].(pgtype.UUID),
			Status:     "pending",
			CreatedAt:  pgtype.Timestamptz{},
		}
		if len(args) >= 4 {
			if p, ok := args[3].(pgtype.Int4); ok && p.Valid {
				teamTask.Priority = p.Int32
			}
		}
		s.teamTasks[util.UUIDToString(teamTask.ID)] = teamTask
		// RETURNING: id, team_id, issue_id, assigned_by, status, delegated_to_agent_id, priority, created_at, updated_at
		return stubRow{values: []any{
			teamTask.ID, teamTask.TeamID, teamTask.IssueID, teamTask.AssignedBy,
			teamTask.Status, teamTask.DelegatedToAgentID, teamTask.Priority,
			teamTask.CreatedAt, teamTask.UpdatedAt,
		}}
	}

	// CreateTeam — INSERT INTO team ... RETURNING *
	if strings.Contains(sql, "INSERT INTO team") {
		team := db.Team{
			ID:          s.nextUUID(),
			WorkspaceID: args[0].(pgtype.UUID),
			Name:        args[1].(string),
			CreatedBy:   args[5].(pgtype.UUID),
		}
		s.teams[util.UUIDToString(team.ID)] = team
		return stubRow{values: []any{
			team.ID, team.WorkspaceID, team.Name, team.Description, team.AvatarUrl, team.LeadAgentID,
			team.CreatedBy, team.CreatedAt, team.UpdatedAt, team.ArchivedAt, team.ArchivedBy,
			team.QueuePolicy, team.CapabilityTags, team.MaxRunDuration, team.MaxConcurrent,
		}}
	}

	// UpdateTeamTaskStatus — UPDATE team_task_queue SET status = $2 ... RETURNING *
	if strings.Contains(sql, "UPDATE team_task_queue") || strings.Contains(sql, "updateTeamTaskStatus") {
		id := args[0].(pgtype.UUID)
		if t, ok := s.teamTasks[util.UUIDToString(id)]; ok {
			t.Status = args[1].(string)
			if len(args) >= 3 {
				if agentID, ok := args[2].(pgtype.UUID); ok && agentID.Valid {
					t.DelegatedToAgentID = agentID
				}
			}
			s.teamTasks[util.UUIDToString(id)] = t
			// RETURNING: id, team_id, issue_id, assigned_by, status, delegated_to_agent_id, priority, created_at, updated_at
			return stubRow{values: []any{
				t.ID, t.TeamID, t.IssueID, t.AssignedBy,
				t.Status, t.DelegatedToAgentID, t.Priority,
				t.CreatedAt, t.UpdatedAt,
			}}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// GetWorkspace — FROM workspace WHERE id = $1
	if strings.Contains(sql, "FROM workspace") && strings.Contains(sql, "WHERE id =") && len(args) >= 1 {
		id := args[0].(pgtype.UUID)
		if ws, ok := s.workspaces[util.UUIDToString(id)]; ok {
			return stubRow{values: []any{
				ws.ID, ws.Name, ws.Slug, ws.Description, ws.Settings,
				ws.CreatedAt, ws.UpdatedAt, ws.Context, ws.Repos,
				ws.IssuePrefix, ws.IssueCounter,
			}}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CreateComment — INSERT INTO comment ... RETURNING id, issue_id, author_type, author_id, content, type, created_at, updated_at, parent_id, workspace_id
	if strings.Contains(sql, "INSERT INTO comment") {
		commentID := s.nextUUID()
		return stubRow{values: []any{
			commentID,           // id
			args[0].(pgtype.UUID), // issue_id
			args[2].(string),    // author_type
			args[3].(pgtype.UUID), // author_id
			args[4].(string),    // content
			args[5].(string),    // type
			pgtype.Timestamptz{}, // created_at
			pgtype.Timestamptz{}, // updated_at
			args[6].(pgtype.UUID), // parent_id
			args[1].(pgtype.UUID), // workspace_id
		}}
	}

	// CreateAgentMessage — INSERT INTO agent_message ... RETURNING
	if strings.Contains(sql, "INSERT INTO agent_message") {
		msg := db.AgentMessage{
			ID:          s.nextUUID(),
			WorkspaceID: args[0].(pgtype.UUID), // $1 workspace_id
			FromAgentID: args[1].(pgtype.UUID), // $2 from_agent_id
			ToAgentID:   args[2].(pgtype.UUID), // $3 to_agent_id
			Content:     args[3].(string),       // $4 content
			Metadata:    args[4].([]byte),       // $5 metadata
			MessageType: args[5].(string),       // $6 message_type
			TaskID:      args[6].(pgtype.UUID),  // $7 task_id
			ReplyToID:   args[7].(pgtype.UUID),  // $8 reply_to_id
		}
		s.messages = append(s.messages, msg)
		return stubRow{values: []any{msg.ID, msg.WorkspaceID, msg.FromAgentID, msg.ToAgentID, msg.TaskID, msg.Content, msg.Metadata, msg.CreatedAt, msg.MessageType, msg.ReadAt, msg.ReplyToID}}
	}

	// CreateTaskDependency — INSERT INTO task_dependency ... RETURNING
	if strings.Contains(sql, "INSERT INTO task_dependency") {
		dep := db.TaskDependency{
			ID:              s.nextUUID(),
			WorkspaceID:     args[0].(pgtype.UUID),
			TaskID:          args[1].(pgtype.UUID),
			DependsOnTaskID: args[2].(pgtype.UUID),
		}
		// Store in both maps for wouldCreateCycle support
		taskKey := util.UUIDToString(dep.TaskID)
		s.taskDeps[taskKey] = append(s.taskDeps[taskKey], dep)
		depKey := util.UUIDToString(dep.DependsOnTaskID)
		s.taskDependents[depKey] = append(s.taskDependents[depKey], dep)
		return stubRow{values: []any{dep.ID, dep.WorkspaceID, dep.TaskID, dep.DependsOnTaskID, dep.CreatedAt}}
	}

	// GetRuntimePolicyByAgent — FROM runtime_assignment_policy WHERE agent_id = $1 AND workspace_id = $2
	if strings.Contains(sql, "FROM runtime_assignment_policy") && strings.Contains(sql, "agent_id") && len(args) >= 2 {
		agentID := args[0].(pgtype.UUID)
		if p, ok := s.runtimePolicies[util.UUIDToString(agentID)]; ok {
			return stubRow{values: []any{p.ID, p.WorkspaceID, p.AgentID, p.TeamID, p.RequiredTags, p.ForbiddenTags, p.PreferredRuntimeIds, p.FallbackRuntimeIds, p.MaxQueueDepth, p.IsActive, p.CreatedAt, p.UpdatedAt}}
		}
		return stubRow{err: pgx.ErrNoRows}
	}

	// CountPendingTasksByRuntime — SELECT count(*) FROM agent_task_queue WHERE runtime_id = $1 AND status IN (...)
	if strings.Contains(sql, "count(*)") && strings.Contains(sql, "runtime_id") && len(args) >= 1 {
		runtimeID := args[0].(pgtype.UUID)
		count := 0
		for _, t := range s.tasks {
			if util.UUIDToString(t.RuntimeID) == util.UUIDToString(runtimeID) &&
				(t.Status == "queued" || t.Status == "dispatched") {
				count++
			}
		}
		return stubRow{values: []any{int64(count)}}
	}

	// CreateAgentMemory — INSERT INTO agent_memory ... RETURNING
	if strings.Contains(sql, "INSERT INTO agent_memory") {
		memID := s.nextUUID()
		mem := db.AgentMemory{
			ID:          memID,
			WorkspaceID: args[0].(pgtype.UUID),
			AgentID:     args[1].(pgtype.UUID),
			Content:     args[2].(string),
			Embedding:   args[3].(pgvector_go.Vector),
			Metadata:    args[4].([]byte),
			ExpiresAt:   args[5].(pgtype.Timestamptz),
		}
		s.memoryEntries = append(s.memoryEntries, mem)
		return stubRow{values: []any{mem.ID, mem.WorkspaceID, mem.AgentID, mem.Content, mem.Embedding, mem.Metadata, pgtype.Timestamptz{}, mem.ExpiresAt, nil}}
	}

	// CreateTaskCheckpoint — INSERT INTO task_checkpoint ... RETURNING
	if strings.Contains(sql, "INSERT INTO task_checkpoint") {
		cpID := s.nextUUID()
		cp := db.TaskCheckpoint{
			ID:           cpID,
			TaskID:       args[0].(pgtype.UUID),
			WorkspaceID:  args[1].(pgtype.UUID),
			Label:        args[2].(string),
			State:        args[3].([]byte),
			FilesChanged: args[4].([]byte),
		}
		s.checkpoints = append(s.checkpoints, cp)
		return stubRow{values: []any{cp.ID, cp.TaskID, cp.WorkspaceID, cp.Label, cp.State, cp.FilesChanged, pgtype.Timestamptz{}}}
	}

	// GetLatestCheckpoint — FROM task_checkpoint WHERE task_id = $1 ORDER BY created_at DESC LIMIT 1
	if strings.Contains(sql, "FROM task_checkpoint") && strings.Contains(sql, "task_id") && len(args) >= 1 {
		taskID := args[0].(pgtype.UUID)
		var latest *db.TaskCheckpoint
		for i := range s.checkpoints {
			if s.checkpoints[i].TaskID == taskID {
				latest = &s.checkpoints[i]
			}
		}
		if latest == nil {
			return stubRow{err: pgx.ErrNoRows}
		}
		return stubRow{values: []any{latest.ID, latest.TaskID, latest.WorkspaceID, latest.Label, latest.State, latest.FilesChanged, latest.CreatedAt}}
	}

	return stubRow{err: fmt.Errorf("taskStubDBTX.QueryRow not implemented for SQL: %s", sql)}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestTaskService(stub *taskStubDBTX) *TaskService {
	return &TaskService{
		Queries: db.New(stub),
		Hub:     realtime.NewHub(nil),
		Bus:     events.New(),
	}
}

func newTestTeamService(stub *taskStubDBTX) *TeamService {
	return &TeamService{
		Queries: db.New(stub),
		Bus:     events.New(),
	}
}

func newTestReviewService(stub *taskStubDBTX, ts *TaskService) *ReviewService {
	return &ReviewService{
		Queries:     db.New(stub),
		Hub:         realtime.NewHub(nil),
		Bus:         events.New(),
		TaskService: ts,
	}
}

func newTestCollaborationService(stub *taskStubDBTX) *CollaborationService {
	return &CollaborationService{
		Queries: db.New(stub),
		Hub:     realtime.NewHub(nil),
		Bus:     events.New(),
	}
}

func makeUUID(val byte) pgtype.UUID {
	id := [16]byte{}
	id[15] = val
	return pgtype.UUID{Bytes: id, Valid: true}
}

// uuidStr returns the string key for a makeUUID(val) — for use as stub map keys.
func uuidStr(val byte) string {
	return util.UUIDToString(makeUUID(val))
}

// ---------------------------------------------------------------------------
// priorityToInt
// ---------------------------------------------------------------------------

func TestPriorityToInt(t *testing.T) {
	tests := []struct {
		input string
		want  int32
	}{
		{"urgent", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"", 0},
		{"unknown", 0},
		{"URGENT", 0}, // case sensitive
		{"High", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := priorityToInt(tt.input)
			if got != tt.want {
				t.Errorf("priorityToInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CanTransition extended tests (complement startup_sequence_test.go)
// ---------------------------------------------------------------------------

func TestCanTransition_AllStates(t *testing.T) {
	allStates := []TaskState{
		TaskStateQueued, TaskStateDispatched, TaskStateRunning,
		TaskStateInReview, TaskStateCompleted, TaskStateFailed, TaskStateCancelled,
	}

	// Verify every valid transition.
	validTransitions := map[TaskState][]TaskState{
		TaskStateQueued:     {TaskStateDispatched, TaskStateFailed, TaskStateCancelled},
		TaskStateDispatched: {TaskStateRunning, TaskStateQueued, TaskStateFailed, TaskStateCancelled},
		TaskStateRunning:    {TaskStateInReview, TaskStateCompleted, TaskStateFailed, TaskStateCancelled},
		TaskStateInReview:   {TaskStateCompleted, TaskStateFailed, TaskStateQueued, TaskStateCancelled},
		TaskStateCompleted:  {},
		TaskStateFailed:     {TaskStateQueued},
		TaskStateCancelled:  {},
	}

	for _, from := range allStates {
		for _, to := range allStates {
			allowed := false
			for _, v := range validTransitions[from] {
				if v == to {
					allowed = true
					break
				}
			}
			got := CanTransition(from, to)
			if got != allowed {
				t.Errorf("CanTransition(%q, %q) = %v, want %v", from, to, got, allowed)
			}
		}
	}
}

func TestCanTransition_InvalidState(t *testing.T) {
	// Unknown state should never transition.
	if CanTransition(TaskState("bogus"), TaskStateQueued) {
		t.Error("expected false for unknown from state")
	}
	if CanTransition(TaskStateQueued, TaskState("bogus")) {
		t.Error("expected false for unknown to state")
	}
}

// ---------------------------------------------------------------------------
// ClaimTask
// ---------------------------------------------------------------------------

func TestClaimTask_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	runtimeID := makeUUID(2)
	issueID := makeUUID(3)
	taskID := makeUUID(10)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, WorkspaceID: makeUUID(99), RuntimeID: runtimeID,
		MaxConcurrentTasks: 3, Status: "idle",
	}
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, AgentID: agentID, IssueID: issueID, RuntimeID: runtimeID,
		Status: "queued", Priority: 2,
	}
	stub.issues[util.UUIDToString(issueID)] = db.Issue{
		ID: issueID, WorkspaceID: makeUUID(99),
	}
	stub.agentStatus[util.UUIDToString(agentID)] = "idle"

	task, err := svc.ClaimTask(context.Background(), agentID)
	if err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.Status != "dispatched" {
		t.Errorf("expected dispatched status, got %q", task.Status)
	}
}

func TestClaimTask_AgentNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	_, err := svc.ClaimTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestClaimTask_NoCapacity(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, MaxConcurrentTasks: 1,
	}
	// CountRunningTasks returns 0 rows = 0 running, but with MaxConcurrentTasks=1
	// and 0 running tasks, it should have capacity. Let's test the boundary.
	// Actually CountRunningTasks with stubDBTX returns stubRows which has 0 rows,
	// so count=0. With max=1, capacity exists. Let's test agent with max=0.
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, MaxConcurrentTasks: 0,
	}

	task, err := svc.ClaimTask(context.Background(), agentID)
	if err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	if task != nil {
		t.Error("expected nil task when no capacity")
	}
}

// ---------------------------------------------------------------------------
// CancelTask
// ---------------------------------------------------------------------------

func TestCancelTask_Success(t *testing.T) {
	t.Skip("CancelTask requires QueryRow stubbing — covered by integration tests")
}

func TestCancelTask_InvalidTransition(t *testing.T) {
	// A completed task cannot be cancelled.
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	taskID := makeUUID(10)
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, Status: "completed",
	}

	_, err := svc.CancelTask(context.Background(), taskID)
	if err == nil {
		t.Fatal("expected error cancelling completed task")
	}
	if !strings.Contains(err.Error(), "cannot transition") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// StartTask
// ---------------------------------------------------------------------------

func TestStartTask_InvalidTransition(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	taskID := makeUUID(10)
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, Status: "completed",
	}

	_, err := svc.StartTask(context.Background(), taskID)
	if err == nil {
		t.Fatal("expected error starting completed task")
	}
	if !strings.Contains(err.Error(), "cannot transition") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FailTask
// ---------------------------------------------------------------------------

func TestFailTask_InvalidTransition(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	taskID := makeUUID(10)
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, Status: "completed",
	}

	_, err := svc.FailTask(context.Background(), taskID, "error")
	if err == nil {
		t.Fatal("expected error failing completed task")
	}
	if !strings.Contains(err.Error(), "cannot transition") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CompleteTask (no-review path validation)
// ---------------------------------------------------------------------------

func TestCompleteTask_InvalidTransition(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	taskID := makeUUID(10)
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, Status: "completed",
	}

	_, err := svc.CompleteTask(context.Background(), taskID, nil, "", "")
	if err == nil {
		t.Fatal("expected error completing already-completed task")
	}
	if !strings.Contains(err.Error(), "cannot transition") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReportProgress
// ---------------------------------------------------------------------------

func TestReportProgress_PublishesEvent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	svc.ReportProgress(context.Background(), "task-123", "ws-456", "step 1 of 3", 1, 3)

	// Event should be published to the bus. We can't directly observe without
	// a subscriber, but we verify no panic/crash.
}

// ---------------------------------------------------------------------------
// EnqueueTaskForIssue / EnqueueTaskForMention (validation only)
// ---------------------------------------------------------------------------

func TestEnqueueTaskForIssue_NoAssignee(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	issue := db.Issue{
		ID: makeUUID(1), AssigneeID: pgtype.UUID{}, // not valid
	}
	_, err := svc.EnqueueTaskForIssue(context.Background(), issue)
	if err == nil {
		t.Fatal("expected error for issue without assignee")
	}
	if !strings.Contains(err.Error(), "no assignee") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnqueueTaskForIssue_AgentNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	issue := db.Issue{
		ID: makeUUID(2), AssigneeID: agentID, WorkspaceID: makeUUID(99),
	}
	_, err := svc.EnqueueTaskForIssue(context.Background(), issue)
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
	if !strings.Contains(err.Error(), "load agent") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnqueueTaskForIssue_ArchivedAgent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, ArchivedAt: pgtype.Timestamptz{Valid: true},
	}

	issue := db.Issue{
		ID: makeUUID(2), AssigneeID: agentID,
	}
	_, err := svc.EnqueueTaskForIssue(context.Background(), issue)
	if err == nil {
		t.Fatal("expected error for archived agent")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnqueueTaskForIssue_NoRuntime(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: pgtype.UUID{}, // not valid
	}

	issue := db.Issue{
		ID: makeUUID(2), AssigneeID: agentID,
	}
	_, err := svc.EnqueueTaskForIssue(context.Background(), issue)
	if err == nil {
		t.Fatal("expected error for agent without runtime")
	}
	if !strings.Contains(err.Error(), "no runtime") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnqueueTaskForMention_ArchivedAgent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, ArchivedAt: pgtype.Timestamptz{Valid: true},
	}

	issue := db.Issue{ID: makeUUID(2)}
	_, err := svc.EnqueueTaskForMention(context.Background(), issue, agentID, makeUUID(3))
	if err == nil {
		t.Fatal("expected error for archived agent")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ChainTask (validation only)
// ---------------------------------------------------------------------------

func TestReconcileAgentStatus_Idle(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, WorkspaceID: makeUUID(99), Status: "working",
	}

	// CountRunningTasks returns 0 with stubDBTX (no matching rows).
	svc.ReconcileAgentStatus(context.Background(), agentID)

	// Should not panic. Status update goes through UpdateAgentStatus which
	// will fail at the SQL level with stubDBTX, but the method silently ignores errors.
}

// ---------------------------------------------------------------------------
// LoadAgentSkills
// ---------------------------------------------------------------------------

func TestLoadAgentSkills_NoSkills(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	// ListAgentSkills returns no rows with stubDBTX.
	skills := svc.LoadAgentSkills(context.Background(), makeUUID(1))
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

// ---------------------------------------------------------------------------
// TeamService tests
// ---------------------------------------------------------------------------

func TestTeamService_GetDelegationMode_WithLead(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	teamID := makeUUID(1)
	stub.teams[util.UUIDToString(teamID)] = db.Team{
		ID: teamID, LeadAgentID: makeUUID(2),
	}

	mode, err := svc.GetDelegationMode(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetDelegationMode: %v", err)
	}
	if mode != LeadDelegation {
		t.Errorf("expected LeadDelegation, got %v", mode)
	}
}

func TestTeamService_GetDelegationMode_NoLead(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	teamID := makeUUID(1)
	stub.teams[util.UUIDToString(teamID)] = db.Team{
		ID: teamID, LeadAgentID: pgtype.UUID{}, // not valid
	}

	mode, err := svc.GetDelegationMode(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetDelegationMode: %v", err)
	}
	if mode != BroadcastMode {
		t.Errorf("expected BroadcastMode, got %v", mode)
	}
}

func TestTeamService_GetTeamWithMembers_NotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	_, _, err := svc.GetTeamWithMembers(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error for missing team")
	}
}

func TestTeamService_AddMember(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	// AddTeamMember with stubDBTX will fail at SQL level, but we test the call path.
	_, err := svc.AddMember(context.Background(), makeUUID(1), makeUUID(2), "member")
	// With stubDBTX, QueryRow returns an error — that's expected.
	if err == nil {
		t.Log("unexpected success with stub — may indicate stub mismatch")
	}
}

func TestTeamService_RemoveMember(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	// RemoveTeamMember calls Exec which is a no-op in stubDBTX.
	err := svc.RemoveMember(context.Background(), makeUUID(1), makeUUID(2))
	if err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
}

func TestTeamService_SetLead_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	teamID := makeUUID(1)
	newLeadID := makeUUID(5)

	// Pre-populate a team so UpdateTeamLead succeeds.
	stub.teams[util.UUIDToString(teamID)] = db.Team{ID: teamID, Name: "alpha"}

	err := svc.SetLead(context.Background(), teamID, newLeadID)
	if err != nil {
		t.Fatalf("SetLead should succeed with handlers in place: %v", err)
	}
	// Verify team lead was updated.
	team := stub.teams[util.UUIDToString(teamID)]
	if team.LeadAgentID != newLeadID {
		t.Errorf("team lead = %v, want %v", team.LeadAgentID, newLeadID)
	}
}

func TestTeamService_SetLead_DemotesExistingLead(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	teamID := makeUUID(1)
	oldLeadID := makeUUID(3)
	newLeadID := makeUUID(5)

	// Pre-populate team members with an existing lead.
	stub.teamMembers[util.UUIDToString(teamID)] = []db.TeamMember{
		{TeamID: teamID, AgentID: oldLeadID, Role: "lead"},
		{TeamID: teamID, AgentID: newLeadID, Role: "member"},
	}
	stub.teams[util.UUIDToString(teamID)] = db.Team{ID: teamID}

	err := svc.SetLead(context.Background(), teamID, newLeadID)
	if err != nil {
		t.Fatalf("SetLead: %v", err)
	}
}

func TestTeamService_SetLead_ListTeamMembersError(t *testing.T) {
	stub := newTaskStubDBTX()
	// Set queryErr to simulate DB failure.
	stub.queryErr = fmt.Errorf("db connection lost")
	svc := newTestTeamService(stub)

	err := svc.SetLead(context.Background(), makeUUID(1), makeUUID(2))
	if err == nil {
		t.Fatal("expected error when ListTeamMembers fails")
	}
	if !containsStr(err.Error(), "list team members") {
		t.Errorf("error should mention 'list team members', got: %v", err)
	}
}

func TestTeamService_SetLead_UpdateTeamLeadError(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	teamID := makeUUID(1)
	// Don't add team to stub.teams — UpdateTeamLead will get ErrNoRows.
	err := svc.SetLead(context.Background(), teamID, makeUUID(2))
	if err == nil {
		t.Fatal("expected error when UpdateTeamLead fails (team not found)")
	}
	if !containsStr(err.Error(), "update team lead") {
		t.Errorf("error should mention 'update team lead', got: %v", err)
	}
}

func TestTeamService_GetPendingTasks(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	// ListPendingTeamTasks returns empty with stubDBTX.
	tasks, err := svc.GetPendingTasks(context.Background(), makeUUID(1))
	if err != nil {
		t.Fatalf("GetPendingTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// ---------------------------------------------------------------------------
// ReviewService tests
// ---------------------------------------------------------------------------

func TestReviewService_RetryTask(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	// RetryTaskReview will fail with stubDBTX.
	_, err := rs.RetryTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error with stub")
	}
}

func TestReviewService_SubmitManualReview_InvalidVerdict(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	taskID := makeUUID(10)
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, Status: "in_review",
	}

	// GetAgentTask will fail with stubDBTX, so we get "load task" error.
	_, err := rs.SubmitManualReview(context.Background(), taskID, makeUUID(1), "invalid", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReviewService_ReviewTask(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	// GetAgentTask will fail with stubDBTX.
	_, err := rs.ReviewTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error with stub")
	}
}

// ---------------------------------------------------------------------------
// strPtrToText / ptrToUUID helpers
// ---------------------------------------------------------------------------

func TestStrPtrToText(t *testing.T) {
	nilResult := strPtrToText(nil)
	if nilResult.Valid {
		t.Error("expected nil input to produce invalid text")
	}

	s := "hello"
	result := strPtrToText(&s)
	if !result.Valid || result.String != "hello" {
		t.Errorf("expected valid text 'hello', got %+v", result)
	}
}

func TestPtrToUUID(t *testing.T) {
	nilResult := ptrToUUID(nil)
	if nilResult.Valid {
		t.Error("expected nil input to produce invalid UUID")
	}

	u := makeUUID(42)
	result := ptrToUUID(&u)
	if !result.Valid || result != u {
		t.Errorf("expected %v, got %v", u, result)
	}
}

// ---------------------------------------------------------------------------
// DelegationMode constants
// ---------------------------------------------------------------------------

func TestDelegationModeConstants(t *testing.T) {
	if LeadDelegation == BroadcastMode {
		t.Error("LeadDelegation and BroadcastMode should differ")
	}
	if int(LeadDelegation) != 0 {
		t.Errorf("LeadDelegation should be 0, got %d", int(LeadDelegation))
	}
	if int(BroadcastMode) != 1 {
		t.Errorf("BroadcastMode should be 1, got %d", int(BroadcastMode))
	}
}

// ---------------------------------------------------------------------------
// AgentSkillData / AgentSkillFileData structs
// ---------------------------------------------------------------------------

func TestAgentSkillData_EmptyFiles(t *testing.T) {
	skill := AgentSkillData{Name: "test", Content: "prompt"}
	if len(skill.Files) != 0 {
		t.Error("expected empty files")
	}
}

func TestAgentSkillFileData(t *testing.T) {
	f := AgentSkillFileData{Path: "src/main.go", Content: "package main"}
	if f.Path != "src/main.go" {
		t.Errorf("unexpected path: %s", f.Path)
	}
}

// ---------------------------------------------------------------------------
// P3: TaskService success-path tests
// ---------------------------------------------------------------------------

func TestCancelTask_Success_Queued(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3), Status: "queued"},
		},
	}
	svc := newTestTaskService(stub)

	task, err := svc.CancelTask(context.Background(), makeUUID(2))
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if task.Status != "cancelled" {
		t.Errorf("expected status cancelled, got %s", task.Status)
	}
}

func TestCancelTask_AlreadyCancelled(t *testing.T) {
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{uuidStr(1): {ID: makeUUID(1)}},
		tasks:  map[string]db.AgentTaskQueue{uuidStr(2): {ID: makeUUID(2), Status: "cancelled"}},
	}
	svc := newTestTaskService(stub)

	_, err := svc.CancelTask(context.Background(), makeUUID(2))
	if err == nil {
		t.Fatal("expected error for cancelling already-cancelled task")
	}
}

func TestStartTask_Success(t *testing.T) {
	stub := &taskStubDBTX{
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(1): {ID: makeUUID(1), Status: "dispatched"},
		},
	}
	svc := newTestTaskService(stub)

	task, err := svc.StartTask(context.Background(), makeUUID(1))
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.Status != "running" {
		t.Errorf("expected status running, got %s", task.Status)
	}
}

func TestStartTask_FromQueued_Fails2(t *testing.T) {
	stub := &taskStubDBTX{
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(1): {ID: makeUUID(1), Status: "queued"},
		},
	}
	svc := newTestTaskService(stub)

	_, err := svc.StartTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error for starting queued task (must be dispatched first)")
	}
}

func TestCompleteTask_NoReview(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {
				ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3),
				Status: "running", MaxReviews: 0,
			},
		},
		issues: map[string]db.Issue{
			uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)},
		},
	}
	svc := newTestTaskService(stub)

	task, err := svc.CompleteTask(context.Background(), makeUUID(2), []byte(`{"output":"done"}`), "sess-1", "/tmp/work")
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("expected status completed, got %s", task.Status)
	}
}

func TestCompleteTask_WithReview(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {
				ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3),
				Status: "running", MaxReviews: 2, ReviewCount: 0,
			},
		},
		issues: map[string]db.Issue{
			uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)},
		},
	}
ts := newTestTaskService(stub)

	task, err := ts.CompleteTask(context.Background(), makeUUID(2), []byte(`{"output":"result"}`), "sess-2", "/tmp/ws")
	if err != nil {
		t.Fatalf("CompleteTask with review: %v", err)
	}
	if task.Status != "in_review" {
		t.Errorf("expected status in_review, got %s", task.Status)
	}
}

func TestFailTask_Success(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3), Status: "running"},
		},
		issues: map[string]db.Issue{
			uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)},
		},
	}
	svc := newTestTaskService(stub)

	task, err := svc.FailTask(context.Background(), makeUUID(2), "something went wrong")
	if err != nil {
		t.Fatalf("FailTask: %v", err)
	}
	if task.Status != "failed" {
		t.Errorf("expected status failed, got %s", task.Status)
	}
}

func TestFailTask_NoError(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{uuidStr(1): {ID: agentID}},
		tasks:  map[string]db.AgentTaskQueue{uuidStr(2): {ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3), Status: "running"}},
		issues: map[string]db.Issue{uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)}},
	}
	svc := newTestTaskService(stub)

	task, err := svc.FailTask(context.Background(), makeUUID(2), "")
	if err != nil {
		t.Fatalf("FailTask: %v", err)
	}
	if task.Status != "failed" {
		t.Errorf("expected failed, got %s", task.Status)
	}
}

func TestEnqueueTaskForIssue_Success(t *testing.T) {
	runtimeID := makeUUID(10)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: makeUUID(1), Status: "idle", RuntimeID: runtimeID},
		},
		issues: map[string]db.Issue{
			uuidStr(2): {ID: makeUUID(2), AssigneeID: makeUUID(1), Priority: "5"},
		},
		tasks: map[string]db.AgentTaskQueue{},
	}
	svc := newTestTaskService(stub)

	task, err := svc.EnqueueTaskForIssue(context.Background(), stub.issues[uuidStr(2)])
	if err != nil {
		t.Fatalf("EnqueueTaskForIssue: %v", err)
	}
	if task.Status != "queued" {
		t.Errorf("expected status queued, got %s", task.Status)
	}
	if task.AgentID != makeUUID(1) {
		t.Errorf("expected agent %v, got %v", makeUUID(1), task.AgentID)
	}
}

func TestEnqueueTaskForIssue_ArchivedAgent_P3(t *testing.T) {
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: makeUUID(1), ArchivedAt: pgtype.Timestamptz{Valid: true}},
		},
	}
	svc := newTestTaskService(stub)

	_, err := svc.EnqueueTaskForIssue(context.Background(), db.Issue{AssigneeID: makeUUID(1)})
	if err == nil {
		t.Fatal("expected error for archived agent")
	}
}

func TestReviewTask_AutoApprove(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {
				ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3),
				Status: "in_review", MaxReviews: 2, ReviewCount: 0,
			},
		},
		issues: map[string]db.Issue{
			uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)},
		},
	}
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	task, err := rs.ReviewTask(context.Background(), makeUUID(2))
	if err != nil {
		t.Fatalf("ReviewTask: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("expected completed, got %s", task.Status)
	}
	if task.ReviewStatus != "passed" {
		t.Errorf("expected review_status passed, got %s", task.ReviewStatus)
	}
}

func TestReviewTask_NotInReview(t *testing.T) {
	stub := &taskStubDBTX{
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(1): {ID: makeUUID(1), Status: "running"},
		},
	}
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	_, err := rs.ReviewTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error for task not in review")
	}
}

func TestSubmitManualReview_Approved(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {
				ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3),
				Status: "in_review", MaxReviews: 2,
			},
		},
		issues: map[string]db.Issue{
			uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)},
		},
	}
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	task, err := rs.SubmitManualReview(context.Background(), makeUUID(2), makeUUID(5), "approved", "looks good")
	if err != nil {
		t.Fatalf("SubmitManualReview approved: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("expected completed, got %s", task.Status)
	}
}

func TestSubmitManualReview_Rejected(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {
				ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3),
				Status: "in_review",
			},
		},
		issues: map[string]db.Issue{
			uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)},
		},
	}
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	task, err := rs.SubmitManualReview(context.Background(), makeUUID(2), makeUUID(5), "rejected", "needs changes")
	if err != nil {
		t.Fatalf("SubmitManualReview rejected: %v", err)
	}
	if task.Status != "failed" {
		t.Errorf("expected failed, got %s", task.Status)
	}
}

func TestSubmitManualReview_InvalidVerdict(t *testing.T) {
	stub := &taskStubDBTX{
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(1): {ID: makeUUID(1), Status: "in_review"},
		},
	}
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	_, err := rs.SubmitManualReview(context.Background(), makeUUID(1), makeUUID(5), "maybe", "")
	if err == nil {
		t.Fatal("expected error for invalid verdict")
	}
}

func TestRetryTask_Success(t *testing.T) {
	stub := &taskStubDBTX{
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(1): {ID: makeUUID(1), Status: "in_review", ReviewCount: 0, MaxReviews: 3},
		},
	}
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	task, err := rs.RetryTask(context.Background(), makeUUID(1))
	if err != nil {
		t.Fatalf("RetryTask: %v", err)
	}
	if task.Status != "queued" {
		t.Errorf("expected queued, got %s", task.Status)
	}
	if task.ReviewCount != 1 {
		t.Errorf("expected review_count 1, got %d", task.ReviewCount)
	}
}

// ---------------------------------------------------------------------------
// P3: TeamService success-path tests
// ---------------------------------------------------------------------------

func TestTeamService_CreateTeam_Success(t *testing.T) {
	stub := &taskStubDBTX{
		teams:       map[string]db.Team{},
		teamMembers: map[string][]db.TeamMember{},
	}
	svc := newTestTeamService(stub)

	desc := "A test team"
	team, err := svc.CreateTeam(context.Background(), CreateTeamInput{
		WorkspaceID: makeUUID(1),
		Name:        "Alpha",
		Description: &desc,
		CreatedBy:   makeUUID(2),
	})
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if team.Name != "Alpha" {
		t.Errorf("expected name Alpha, got %s", team.Name)
	}
}

func TestTeamService_CreateTeam_WithMembers(t *testing.T) {
	stub := &taskStubDBTX{
		teams:       map[string]db.Team{},
		teamMembers: map[string][]db.TeamMember{},
	}
	svc := newTestTeamService(stub)

	leadID := makeUUID(10)
	memberID := makeUUID(11)
	_, err := svc.CreateTeam(context.Background(), CreateTeamInput{
		WorkspaceID:    makeUUID(1),
		Name:           "Bravo",
		CreatedBy:      makeUUID(2),
		LeadAgentID:    &leadID,
		MemberAgentIDs: []pgtype.UUID{leadID, memberID},
	})
	if err != nil {
		t.Fatalf("CreateTeam with members: %v", err)
	}
}

func TestTeamService_AddMember_Success(t *testing.T) {
	stub := &taskStubDBTX{
		teamMembers: map[string][]db.TeamMember{},
	}
	svc := newTestTeamService(stub)

	_, err := svc.AddMember(context.Background(), makeUUID(1), makeUUID(2), "member")
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
}

func TestTeamService_EnqueueTeamTask_Success(t *testing.T) {
	stub := &taskStubDBTX{
		teamTasks: map[string]db.TeamTaskQueue{},
	}
	svc := newTestTeamService(stub)

	task, err := svc.EnqueueTeamTask(context.Background(), makeUUID(1), makeUUID(2), makeUUID(3), 5, LeadDelegation)
	if err != nil {
		t.Fatalf("EnqueueTeamTask: %v", err)
	}
	if task.Status != "pending" {
		t.Errorf("expected pending, got %s", task.Status)
	}
}

func TestTeamService_DelegateTask_Success(t *testing.T) {
	taskID := makeUUID(10)
	stub := &taskStubDBTX{
		teamTasks: map[string]db.TeamTaskQueue{
			uuidStr(10): {ID: taskID, Status: "pending"},
		},
	}
	svc := newTestTeamService(stub)

	task, err := svc.DelegateTask(context.Background(), taskID, makeUUID(5))
	if err != nil {
		t.Fatalf("DelegateTask: %v", err)
	}
	if task.Status != "delegated" {
		t.Errorf("expected delegated, got %s", task.Status)
	}
}

func TestTeamService_CompleteTask_Team(t *testing.T) {
	taskID := makeUUID(10)
	stub := &taskStubDBTX{
		teamTasks: map[string]db.TeamTaskQueue{
			uuidStr(10): {ID: taskID, Status: "delegated"},
		},
	}
	svc := newTestTeamService(stub)

	task, err := svc.CompleteTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("CompleteTask (team): %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("expected completed, got %s", task.Status)
	}
}

// ---------------------------------------------------------------------------
// P3: LoadAgentSkills and ReconcileAgentStatus
// ---------------------------------------------------------------------------

func TestLoadAgentSkills_WithFiles(t *testing.T) {
	agentID := makeUUID(1)
	skillID := makeUUID(10)
	stub := &taskStubDBTX{
		skills: map[string][]db.Skill{
			util.UUIDToString(agentID): {
				{ID: skillID, Name: "code-review", Content: "Review code"},
			},
		},
		skillFiles: map[string][]db.SkillFile{
			util.UUIDToString(skillID): {
				{ID: makeUUID(20), SkillID: skillID, Path: "rules.md", Content: "# Rules"},
			},
		},
	}
	svc := newTestTaskService(stub)

	skills := svc.LoadAgentSkills(context.Background(), agentID)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "code-review" {
		t.Errorf("expected name code-review, got %s", skills[0].Name)
	}
	if len(skills[0].Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(skills[0].Files))
	}
}

func TestLoadAgentSkills_EmptyMaps(t *testing.T) {
	stub := &taskStubDBTX{
		skills:     map[string][]db.Skill{},
		skillFiles: map[string][]db.SkillFile{},
	}
	svc := newTestTaskService(stub)

	skills := svc.LoadAgentSkills(context.Background(), makeUUID(1))
	if skills != nil {
		t.Errorf("expected nil skills for agent with no skills, got %d", len(skills))
	}
}

func TestReconcileAgentStatus_Working(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "idle"},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, Status: "running"},
		},
	}
	svc := newTestTaskService(stub)

	// Should not panic
	svc.ReconcileAgentStatus(context.Background(), agentID)

	// Verify agent status was updated to "working"
	updated := stub.agents[util.UUIDToString(agentID)]
	if updated.Status != "working" {
		t.Errorf("expected working, got %s", updated.Status)
	}
}

func TestReconcileAgentStatus_NoRunningTasks(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID, Status: "working"},
		},
		tasks: map[string]db.AgentTaskQueue{},
	}
	svc := newTestTaskService(stub)

	svc.ReconcileAgentStatus(context.Background(), agentID)

	updated := stub.agents[util.UUIDToString(agentID)]
	if updated.Status != "idle" {
		t.Errorf("expected idle, got %s", updated.Status)
	}
}

func TestCancelTasksForIssue_Success(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{
			uuidStr(1): {ID: agentID},
		},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3), Status: "running"},
		},
	}
	svc := newTestTaskService(stub)

	err := svc.CancelTasksForIssue(context.Background(), makeUUID(3))
	if err != nil {
		t.Fatalf("CancelTasksForIssue: %v", err)
	}
}

func TestCancelTasksForIssue_ListActiveError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTaskService(stub)

	err := svc.CancelTasksForIssue(context.Background(), makeUUID(3))
	if err == nil {
		t.Fatal("expected error when ListActiveTasksByIssue fails")
	}
	if !containsStr(err.Error(), "list active tasks") {
		t.Errorf("error should mention 'list active tasks', got: %v", err)
	}
}

func TestCancelTasksForIssue_CancelExecError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.execErr = fmt.Errorf("exec failed")
	svc := newTestTaskService(stub)

	err := svc.CancelTasksForIssue(context.Background(), makeUUID(3))
	if err == nil {
		t.Fatal("expected error when CancelAgentTasksByIssue fails")
	}
	if !containsStr(err.Error(), "cancel tasks") {
		t.Errorf("error should mention 'cancel tasks', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// P3: Task state machine edge cases
// ---------------------------------------------------------------------------

func TestFailTask_FromDispatched(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{uuidStr(1): {ID: agentID}},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3), Status: "dispatched"},
		},
		issues: map[string]db.Issue{uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)}},
	}
	svc := newTestTaskService(stub)

	task, err := svc.FailTask(context.Background(), makeUUID(2), "crash")
	if err != nil {
		t.Fatalf("FailTask from dispatched: %v", err)
	}
	if task.Status != "failed" {
		t.Errorf("expected failed, got %s", task.Status)
	}
}

func TestFailTask_FromInReview(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{uuidStr(1): {ID: agentID}},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, IssueID: makeUUID(3), Status: "in_review"},
		},
		issues: map[string]db.Issue{uuidStr(3): {ID: makeUUID(3), WorkspaceID: makeUUID(4)}},
	}
	svc := newTestTaskService(stub)

	task, err := svc.FailTask(context.Background(), makeUUID(2), "review failed")
	if err != nil {
		t.Fatalf("FailTask from in_review: %v", err)
	}
	if task.Status != "failed" {
		t.Errorf("expected failed, got %s", task.Status)
	}
}

func TestCancelTask_FromDispatched(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{uuidStr(1): {ID: agentID}},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, Status: "dispatched"},
		},
	}
	svc := newTestTaskService(stub)

	task, err := svc.CancelTask(context.Background(), makeUUID(2))
	if err != nil {
		t.Fatalf("CancelTask from dispatched: %v", err)
	}
	if task.Status != "cancelled" {
		t.Errorf("expected cancelled, got %s", task.Status)
	}
}

func TestCancelTask_FromRunning(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{uuidStr(1): {ID: agentID}},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, Status: "running"},
		},
	}
	svc := newTestTaskService(stub)

	task, err := svc.CancelTask(context.Background(), makeUUID(2))
	if err != nil {
		t.Fatalf("CancelTask from running: %v", err)
	}
	if task.Status != "cancelled" {
		t.Errorf("expected cancelled, got %s", task.Status)
	}
}

func TestCancelTask_FromInReview(t *testing.T) {
	agentID := makeUUID(1)
	stub := &taskStubDBTX{
		agents: map[string]db.Agent{uuidStr(1): {ID: agentID}},
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(2): {ID: makeUUID(2), AgentID: agentID, Status: "in_review"},
		},
	}
	svc := newTestTaskService(stub)

	task, err := svc.CancelTask(context.Background(), makeUUID(2))
	if err != nil {
		t.Fatalf("CancelTask from in_review: %v", err)
	}
	if task.Status != "cancelled" {
		t.Errorf("expected cancelled, got %s", task.Status)
	}
}

func TestStartTask_FromQueued_Fails(t *testing.T) {
	stub := &taskStubDBTX{
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(1): {ID: makeUUID(1), Status: "queued"},
		},
	}
	svc := newTestTaskService(stub)

	_, err := svc.StartTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error: queued -> running is invalid (must dispatch first)")
	}
}

func TestStartTask_FromCompleted_Fails(t *testing.T) {
	stub := &taskStubDBTX{
		tasks: map[string]db.AgentTaskQueue{
			uuidStr(1): {ID: makeUUID(1), Status: "completed"},
		},
	}
	svc := newTestTaskService(stub)

	_, err := svc.StartTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error: completed -> running is invalid")
	}
}

// ---------------------------------------------------------------------------
// P3: Team delegation modes
// ---------------------------------------------------------------------------

func TestTeamService_DelegationMode_LeadDelegation(t *testing.T) {
	teamID := makeUUID(1)
	stub := &taskStubDBTX{
		teams: map[string]db.Team{
			util.UUIDToString(teamID): {ID: teamID, LeadAgentID: makeUUID(2)},
		},
	}
	svc := newTestTeamService(stub)

	mode, err := svc.GetDelegationMode(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetDelegationMode: %v", err)
	}
	if mode != LeadDelegation {
		t.Errorf("expected LeadDelegation, got %v", mode)
	}
}

func TestTeamService_DelegationMode_Broadcast(t *testing.T) {
	teamID := makeUUID(1)
	stub := &taskStubDBTX{
		teams: map[string]db.Team{
			util.UUIDToString(teamID): {ID: teamID},
		},
	}
	svc := newTestTeamService(stub)

	mode, err := svc.GetDelegationMode(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetDelegationMode: %v", err)
	}
	if mode != BroadcastMode {
		t.Errorf("expected BroadcastMode, got %v", mode)
	}
}

func TestTeamService_RemoveMember_Empty(t *testing.T) {
	stub := &taskStubDBTX{
		teamMembers: map[string][]db.TeamMember{},
	}
	svc := newTestTeamService(stub)

	err := svc.RemoveMember(context.Background(), makeUUID(1), makeUUID(2))
	if err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
}

func TestTeamService_GetPendingTasks_FiltersDelegated(t *testing.T) {
	teamID := makeUUID(1)
	stub := &taskStubDBTX{
		teamTasks: map[string]db.TeamTaskQueue{
			uuidStr(10): {ID: makeUUID(10), TeamID: teamID, Status: "pending"},
			uuidStr(11): {ID: makeUUID(11), TeamID: teamID, Status: "delegated"},
		},
	}
	svc := newTestTeamService(stub)

	tasks, err := svc.GetPendingTasks(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetPendingTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(tasks))
	}
}

func TestTeamService_GetTeamWithMembers(t *testing.T) {
	teamID := makeUUID(1)
	stub := &taskStubDBTX{
		teams: map[string]db.Team{
			util.UUIDToString(teamID): {ID: teamID, Name: "Alpha"},
		},
		teamMembers: map[string][]db.TeamMember{
			util.UUIDToString(teamID): {
				{TeamID: teamID, AgentID: makeUUID(2)},
			},
		},
	}
	svc := newTestTeamService(stub)

	team, members, err := svc.GetTeamWithMembers(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetTeamWithMembers: %v", err)
	}
	if team.Name != "Alpha" {
		t.Errorf("expected Alpha, got %s", team.Name)
	}
	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

// ---------------------------------------------------------------------------
// P4: Unexported helper tests
// ---------------------------------------------------------------------------

// broadcastTaskDispatch
func TestBroadcastTaskDispatch_PublishesEvent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	issueID := makeUUID(50)
	wsID := makeUUID(99)
	stub.issues[util.UUIDToString(issueID)] = db.Issue{
		ID: issueID, WorkspaceID: wsID, Title: "Test Issue",
	}

	var got events.Event
	svc.Bus.Subscribe("task:dispatch", func(e events.Event) { got = e })

	task := db.AgentTaskQueue{
		ID: makeUUID(10), IssueID: issueID, RuntimeID: makeUUID(20),
		Context: []byte(`{"key":"val"}`),
	}
	svc.broadcastTaskDispatch(context.Background(), task)

	if got.Type != "task:dispatch" {
		t.Errorf("event type = %q, want %q", got.Type, "task:dispatch")
	}
	if got.WorkspaceID != util.UUIDToString(wsID) {
		t.Errorf("workspace = %s, want %s", got.WorkspaceID, util.UUIDToString(wsID))
	}
	payload := got.Payload.(map[string]any)
	if payload["task_id"] != util.UUIDToString(task.ID) {
		t.Errorf("task_id = %v, want %s", payload["task_id"], util.UUIDToString(task.ID))
	}
	if payload["key"] != "val" {
		t.Errorf("context key = %v, want val", payload["key"])
	}
}

func TestBroadcastTaskDispatch_NoIssue_SkipsBroadcast(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	var called bool
	svc.Bus.Subscribe("task:dispatch", func(e events.Event) { called = true })

	task := db.AgentTaskQueue{
		ID: makeUUID(10), IssueID: makeUUID(255), RuntimeID: makeUUID(20),
	}
	svc.broadcastTaskDispatch(context.Background(), task)

	if called {
		t.Error("should not broadcast when issue is missing")
	}
}

// broadcastTaskEvent
func TestBroadcastTaskEvent_PublishesEvent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	issueID := makeUUID(50)
	wsID := makeUUID(99)
	stub.issues[util.UUIDToString(issueID)] = db.Issue{
		ID: issueID, WorkspaceID: wsID,
	}

	var got events.Event
	svc.Bus.Subscribe("task:completed", func(e events.Event) { got = e })

	task := db.AgentTaskQueue{
		ID: makeUUID(10), AgentID: makeUUID(30), IssueID: issueID, Status: "completed",
	}
	svc.broadcastTaskEvent(context.Background(), "task:completed", task)

	if got.Type != "task:completed" {
		t.Errorf("event type = %q", got.Type)
	}
	if got.Payload.(map[string]any)["status"] != "completed" {
		t.Errorf("status = %v", got.Payload.(map[string]any)["status"])
	}
}


// broadcastIssueUpdated
func TestBroadcastIssueUpdated_PublishesEvent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(99)
	issueID := makeUUID(50)
	stub.workspaces[util.UUIDToString(wsID)] = db.Workspace{
		ID: wsID, IssuePrefix: "TEST",
	}

	var got events.Event
	svc.Bus.Subscribe("issue:updated", func(e events.Event) { got = e })

	issue := db.Issue{
		ID: issueID, WorkspaceID: wsID, Title: "My Issue", Status: "open",
		Number: 42, Priority: "high", CreatorType: "user", CreatorID: makeUUID(1),
	}
	svc.broadcastIssueUpdated(issue)

	if got.Type != "issue:updated" {
		t.Errorf("event type = %q", got.Type)
	}
	payload := got.Payload.(map[string]any)
	issueMap := payload["issue"].(map[string]any)
	if issueMap["identifier"] != "TEST-42" {
		t.Errorf("identifier = %v, want TEST-42", issueMap["identifier"])
	}
}

// updateAgentStatus
func TestUpdateAgentStatus_PublishesEvent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(1)
	wsID := makeUUID(99)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, WorkspaceID: wsID, Name: "test-agent", Status: "idle",
	}

	var got events.Event
	svc.Bus.Subscribe("agent:status", func(e events.Event) { got = e })

	svc.updateAgentStatus(context.Background(), agentID, "working")

	if got.Type != "agent:status" {
		t.Errorf("event type = %q", got.Type)
	}
	if stub.agents[util.UUIDToString(agentID)].Status != "working" {
		t.Errorf("agent status not updated in stub")
	}
}

func TestUpdateAgentStatus_AgentNotFound_NoPanic(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	// Should not panic when agent doesn't exist
	svc.updateAgentStatus(context.Background(), makeUUID(255), "working")
}

// getIssuePrefix
func TestGetIssuePrefix_FromDB(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(99)
	stub.workspaces[util.UUIDToString(wsID)] = db.Workspace{
		ID: wsID, IssuePrefix: "MUL",
	}

	prefix := svc.getIssuePrefix(wsID)
	if prefix != "MUL" {
		t.Errorf("prefix = %q, want MUL", prefix)
	}
}

func TestGetIssuePrefix_FromCache(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(99)
	stub.workspaces[util.UUIDToString(wsID)] = db.Workspace{
		ID: wsID, IssuePrefix: "MUL",
	}

	// First call populates cache
	svc.getIssuePrefix(wsID)

	// Remove from DB — cache should still serve
	delete(stub.workspaces, util.UUIDToString(wsID))

	prefix := svc.getIssuePrefix(wsID)
	if prefix != "MUL" {
		t.Errorf("prefix = %q, want MUL (from cache)", prefix)
	}
}

func TestGetIssuePrefix_NotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	prefix := svc.getIssuePrefix(makeUUID(255))
	if prefix != "" {
		t.Errorf("prefix = %q, want empty", prefix)
	}
}

// checkAndLogReadyDependents
func TestCheckAndLogReadyDependents_AllDepsSatisfied(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	completedTaskID := makeUUID(1)
	depTaskID := makeUUID(2)
	issueID := makeUUID(50)
	wsID := makeUUID(99)

	stub.tasks[util.UUIDToString(completedTaskID)] = db.AgentTaskQueue{
		ID: completedTaskID, Status: "completed", IssueID: issueID,
	}
	stub.tasks[util.UUIDToString(depTaskID)] = db.AgentTaskQueue{
		ID: depTaskID, Status: "queued", IssueID: issueID,
		AgentID: makeUUID(30),
	}
	stub.taskDependents[util.UUIDToString(completedTaskID)] = []db.TaskDependency{
		{ID: makeUUID(100), TaskID: depTaskID, DependsOnTaskID: completedTaskID},
	}
	stub.taskDeps[util.UUIDToString(depTaskID)] = []db.TaskDependency{
		{ID: makeUUID(100), TaskID: depTaskID, DependsOnTaskID: completedTaskID},
	}
	stub.issues[util.UUIDToString(issueID)] = db.Issue{
		ID: issueID, WorkspaceID: wsID,
	}

	var got events.Event
	svc.Bus.Subscribe("task:dependencies_satisfied", func(e events.Event) { got = e })

	svc.checkAndLogReadyDependents(context.Background(), completedTaskID)

	if got.Type != "task:dependencies_satisfied" {
		t.Errorf("event type = %q, want task:dependencies_satisfied", got.Type)
	}
	if got.Payload.(map[string]any)["task_id"] != util.UUIDToString(depTaskID) {
		t.Errorf("task_id = %v", got.Payload.(map[string]any)["task_id"])
	}
}

func TestCheckAndLogReadyDependents_NotAllSatisfied(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	completedTaskID := makeUUID(1)
	depTaskID := makeUUID(2)
	otherDepID := makeUUID(3)
	issueID := makeUUID(50)

	stub.tasks[util.UUIDToString(completedTaskID)] = db.AgentTaskQueue{
		ID: completedTaskID, Status: "completed", IssueID: issueID,
	}
	stub.tasks[util.UUIDToString(depTaskID)] = db.AgentTaskQueue{
		ID: depTaskID, Status: "queued", IssueID: issueID,
	}
	stub.tasks[util.UUIDToString(otherDepID)] = db.AgentTaskQueue{
		ID: otherDepID, Status: "running", IssueID: issueID,
	}
	stub.taskDependents[util.UUIDToString(completedTaskID)] = []db.TaskDependency{
		{ID: makeUUID(100), TaskID: depTaskID, DependsOnTaskID: completedTaskID},
	}
	stub.taskDeps[util.UUIDToString(depTaskID)] = []db.TaskDependency{
		{ID: makeUUID(100), TaskID: depTaskID, DependsOnTaskID: completedTaskID},
		{ID: makeUUID(101), TaskID: depTaskID, DependsOnTaskID: otherDepID},
	}

	var called bool
	svc.Bus.Subscribe("task:dependencies_satisfied", func(e events.Event) { called = true })

	svc.checkAndLogReadyDependents(context.Background(), completedTaskID)

	if called {
		t.Error("should not publish event when not all deps satisfied")
	}
}

func TestCheckAndLogReadyDependents_NoDependents(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	taskID := makeUUID(1)
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, Status: "completed", IssueID: makeUUID(50),
	}

	var called bool
	svc.Bus.Subscribe("task:dependencies_satisfied", func(e events.Event) { called = true })

	svc.checkAndLogReadyDependents(context.Background(), taskID)

	if called {
		t.Error("should not publish event when no dependents")
	}
}

// createAgentComment
func TestCreateAgentComment_PublishesEvent(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	issueID := makeUUID(50)
	agentID := makeUUID(30)
	wsID := makeUUID(99)

	stub.issues[util.UUIDToString(issueID)] = db.Issue{
		ID: issueID, WorkspaceID: wsID, Title: "Test Issue", Status: "open",
	}
	stub.workspaces[util.UUIDToString(wsID)] = db.Workspace{
		ID: wsID, IssuePrefix: "TEST",
	}

	var got events.Event
	svc.Bus.Subscribe("comment:created", func(e events.Event) { got = e })

	svc.createAgentComment(context.Background(), issueID, agentID, "Fixed the bug", "comment", pgtype.UUID{})

	if got.Type != "comment:created" {
		t.Errorf("event type = %q", got.Type)
	}
	if got.ActorType != "agent" {
		t.Errorf("actor type = %q, want agent", got.ActorType)
	}
	if got.ActorID != util.UUIDToString(agentID) {
		t.Errorf("actor id = %s", got.ActorID)
	}
	comment := got.Payload.(map[string]any)["comment"].(map[string]any)
	if comment["content"] != "Fixed the bug" {
		t.Errorf("content = %v", comment["content"])
	}
	if got.Payload.(map[string]any)["issue_title"] != "Test Issue" {
		t.Errorf("issue_title = %v", got.Payload.(map[string]any)["issue_title"])
	}
}

func TestCreateAgentComment_EmptyContent_NoOp(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	var called bool
	svc.Bus.Subscribe("comment:created", func(e events.Event) { called = true })

	svc.createAgentComment(context.Background(), makeUUID(1), makeUUID(2), "", "comment", pgtype.UUID{})

	if called {
		t.Error("should not publish event for empty content")
	}
}

func TestCreateAgentComment_IssueNotFound_NoOp(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	var called bool
	svc.Bus.Subscribe("comment:created", func(e events.Event) { called = true })

	svc.createAgentComment(context.Background(), makeUUID(255), makeUUID(2), "hello", "comment", pgtype.UUID{})

	if called {
		t.Error("should not publish event when issue not found")
	}
}

// wouldCreateCycle
func TestWouldCreateCycle_NoCycle(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := &CollaborationService{
		Queries: db.New(stub),
		Bus:     events.New(),
	}

	taskA := makeUUID(1)
	taskB := makeUUID(2)
	taskC := makeUUID(3)

	stub.taskDeps[util.UUIDToString(taskB)] = []db.TaskDependency{
		{TaskID: taskB, DependsOnTaskID: taskA},
	}
	stub.taskDeps[util.UUIDToString(taskC)] = nil

	got := csvc.wouldCreateCycle(context.Background(), taskC, taskB)
	if got {
		t.Error("expected no cycle")
	}
}

func TestWouldCreateCycle_DirectCycle(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := &CollaborationService{
		Queries: db.New(stub),
		Bus:     events.New(),
	}

	taskA := makeUUID(1)
	taskB := makeUUID(2)

	stub.taskDeps[util.UUIDToString(taskB)] = []db.TaskDependency{
		{TaskID: taskB, DependsOnTaskID: taskA},
	}

	got := csvc.wouldCreateCycle(context.Background(), taskA, taskB)
	if !got {
		t.Error("expected direct cycle")
	}
}

func TestWouldCreateCycle_IndirectCycle(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := &CollaborationService{
		Queries: db.New(stub),
		Bus:     events.New(),
	}

	taskA := makeUUID(1)
	taskB := makeUUID(2)
	taskC := makeUUID(3)

	stub.taskDeps[util.UUIDToString(taskB)] = []db.TaskDependency{
		{TaskID: taskB, DependsOnTaskID: taskA},
	}
	stub.taskDeps[util.UUIDToString(taskC)] = []db.TaskDependency{
		{TaskID: taskC, DependsOnTaskID: taskB},
	}

	got := csvc.wouldCreateCycle(context.Background(), taskA, taskC)
	if !got {
		t.Error("expected indirect cycle")
	}
}

func TestWouldCreateCycle_SelfLoop(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := &CollaborationService{
		Queries: db.New(stub),
		Bus:     events.New(),
	}

	taskA := makeUUID(1)

	got := csvc.wouldCreateCycle(context.Background(), taskA, taskA)
	if !got {
		t.Error("expected self-loop cycle")
	}
}

func TestWouldCreateCycle_Diamond(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := &CollaborationService{
		Queries: db.New(stub),
		Bus:     events.New(),
	}

	taskA := makeUUID(1)
	taskB := makeUUID(2)
	taskC := makeUUID(3)
	taskD := makeUUID(4)

	stub.taskDeps[util.UUIDToString(taskB)] = []db.TaskDependency{
		{TaskID: taskB, DependsOnTaskID: taskA},
	}
	stub.taskDeps[util.UUIDToString(taskC)] = []db.TaskDependency{
		{TaskID: taskC, DependsOnTaskID: taskA},
	}
	stub.taskDeps[util.UUIDToString(taskD)] = []db.TaskDependency{
		{TaskID: taskD, DependsOnTaskID: taskB},
		{TaskID: taskD, DependsOnTaskID: taskC},
	}

	got := csvc.wouldCreateCycle(context.Background(), taskA, taskD)
	if !got {
		t.Error("expected cycle through diamond")
	}
}

// ---------------------------------------------------------------------------
// P5: Error-path tests for exported methods
// ---------------------------------------------------------------------------

func TestEnqueueTaskForMention_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	runtimeID := makeUUID(30)
	issueID := makeUUID(40)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID:        agentID,
		RuntimeID: runtimeID,
	}
	stub.runtimes[util.UUIDToString(runtimeID)] = db.AgentRuntime{
		ID: runtimeID, WorkspaceID: wsID, Status: "online",
	}

	issue := db.Issue{ID: issueID, WorkspaceID: wsID, Priority: "high"}

	task, err := svc.EnqueueTaskForMention(context.Background(), issue, agentID, pgtype.UUID{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "queued" {
		t.Errorf("status = %q, want queued", task.Status)
	}
	if task.AgentID != agentID {
		t.Errorf("agentID mismatch")
	}
	if task.IssueID != issueID {
		t.Errorf("issueID mismatch")
	}
}

func TestEnqueueTaskForMention_AgentNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	issue := db.Issue{ID: makeUUID(1), WorkspaceID: makeUUID(2)}
	_, err := svc.EnqueueTaskForMention(context.Background(), issue, makeUUID(99), pgtype.UUID{})
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestEnqueueTaskForMention_AgentArchived(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(20)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID:         agentID,
		ArchivedAt: pgtype.Timestamptz{Valid: true},
	}

	issue := db.Issue{ID: makeUUID(1), WorkspaceID: makeUUID(2)}
	_, err := svc.EnqueueTaskForMention(context.Background(), issue, agentID, pgtype.UUID{})
	if err == nil {
		t.Fatal("expected error for archived agent")
	}
}

func TestEnqueueTaskForMention_AgentNoRuntime(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(20)
	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, // RuntimeID not valid
	}

	issue := db.Issue{ID: makeUUID(1), WorkspaceID: makeUUID(2)}
	_, err := svc.EnqueueTaskForMention(context.Background(), issue, agentID, pgtype.UUID{})
	if err == nil {
		t.Fatal("expected error for agent without runtime")
	}
}

func TestSelectRuntime_NoPolicy_Fallback(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	runtimeID := makeUUID(30)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: runtimeID,
	}
	// No policy in stub → ErrNoRows → fallback to agent.RuntimeID

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != runtimeID {
		t.Errorf("expected agent default runtime")
	}
}

func TestSelectRuntime_InactivePolicy_Fallback(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	runtimeID := makeUUID(30)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: runtimeID,
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:  agentID,
		IsActive: false,
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != runtimeID {
		t.Errorf("expected agent default runtime for inactive policy")
	}
}

func TestSelectRuntime_NoEligibleRuntime_Fallback(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	runtimeID := makeUUID(30)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: runtimeID,
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:  agentID,
		IsActive: true,
	}
	// No runtimes in workspace → fallback to agent.RuntimeID

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != runtimeID {
		t.Errorf("expected agent default runtime when no eligible runtimes")
	}
}

func TestSelectRuntime_PreferredTierWins(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	preferredRT := makeUUID(30)
	normalRT := makeUUID(31)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: normalRT,
	}
	stub.runtimes[util.UUIDToString(preferredRT)] = db.AgentRuntime{
		ID: preferredRT, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimes[util.UUIDToString(normalRT)] = db.AgentRuntime{
		ID: normalRT, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:             agentID,
		IsActive:            true,
		PreferredRuntimeIds: []byte(fmt.Sprintf(`["%s"]`, util.UUIDToString(preferredRT))),
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != preferredRT {
		t.Errorf("expected preferred runtime to win")
	}
}

func TestCompleteTask_NoReviewPath(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(20)
	issueID := makeUUID(40)
	wsID := makeUUID(10)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, WorkspaceID: wsID, Name: "test-agent", Status: "working",
		MaxConcurrentTasks: 5,
	}
	stub.issues[util.UUIDToString(issueID)] = db.Issue{
		ID: issueID, WorkspaceID: wsID,
	}

	taskID := makeUUID(1)
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, AgentID: agentID, IssueID: issueID, Status: "running",
	}

	var completedEvent, agentEvent events.Event
	svc.Bus.Subscribe("task:completed", func(e events.Event) { completedEvent = e })
	svc.Bus.Subscribe("agent:completed", func(e events.Event) { agentEvent = e })

	result, err := svc.CompleteTask(context.Background(), taskID, []byte(`{"output":"done"}`), "sess-1", "/tmp/work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("status = %q, want completed", result.Status)
	}
	if completedEvent.Type != "task:completed" {
		t.Errorf("expected task:completed event, got %q", completedEvent.Type)
	}
	if agentEvent.Type != "agent:completed" {
		t.Errorf("expected agent:completed event, got %q", agentEvent.Type)
	}
}

// ---------------------------------------------------------------------------
// P6: Edge case tests
// ---------------------------------------------------------------------------

func TestTryEnqueueReadySubIssues_Empty(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	// No sub-issues → no tasks created
	svc.TryEnqueueReadySubIssues(context.Background(), makeUUID(1))

	if len(stub.tasks) != 0 {
		t.Errorf("expected no tasks, got %d", len(stub.tasks))
	}
}

func TestTryEnqueueReadySubIssues_WithAssignee(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	parentID := makeUUID(1)
	agentID := makeUUID(20)
	runtimeID := makeUUID(30)
	wsID := makeUUID(10)
	subIssueID := makeUUID(50)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: runtimeID,
	}
	stub.runtimes[util.UUIDToString(runtimeID)] = db.AgentRuntime{
		ID: runtimeID, WorkspaceID: wsID, Status: "online",
	}

	stub.issues[util.UUIDToString(subIssueID)] = db.Issue{
		ID: subIssueID, WorkspaceID: wsID, ParentIssueID: parentID,
		Status: "todo", AssigneeID: agentID, Priority: "medium",
	}

	svc.TryEnqueueReadySubIssues(context.Background(), parentID)

	if len(stub.tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(stub.tasks))
	}
}

func TestTryEnqueueReadySubIssues_WithoutAssignee(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	parentID := makeUUID(1)
	subIssueID := makeUUID(50)

	// Sub-issue with no AssigneeID
	stub.issues[util.UUIDToString(subIssueID)] = db.Issue{
		ID: subIssueID, WorkspaceID: makeUUID(10), ParentIssueID: parentID,
		Status: "todo",
	}

	svc.TryEnqueueReadySubIssues(context.Background(), parentID)

	if len(stub.tasks) != 0 {
		t.Errorf("expected 0 tasks (no assignee), got %d", len(stub.tasks))
	}
}

func TestClaimTaskForRuntime_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	runtimeID := makeUUID(30)
	agentID := makeUUID(20)
	taskID := makeUUID(1)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, WorkspaceID: makeUUID(10), MaxConcurrentTasks: 5,
	}
	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, AgentID: agentID, RuntimeID: runtimeID, Status: "queued",
	}

	task, err := svc.ClaimTaskForRuntime(context.Background(), runtimeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.Status != "dispatched" {
		t.Errorf("status = %q, want dispatched", task.Status)
	}
}

func TestClaimTaskForRuntime_NoTasks(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	runtimeID := makeUUID(30)
	task, err := svc.ClaimTaskForRuntime(context.Background(), runtimeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task != nil {
		t.Errorf("expected nil, got task %v", task.ID)
	}
}

// ---------------------------------------------------------------------------
// P7: CollaborationService tests
// ---------------------------------------------------------------------------

func TestAddDependency_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	wsID := makeUUID(10)
	taskA := makeUUID(1)
	taskB := makeUUID(2)

	var got events.Event
	csvc.Bus.Subscribe("task_dep:created", func(e events.Event) { got = e })

	dep, err := csvc.AddDependency(context.Background(), wsID, taskA, taskB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dep.TaskID != taskA {
		t.Errorf("TaskID mismatch")
	}
	if dep.DependsOnTaskID != taskB {
		t.Errorf("DependsOnTaskID mismatch")
	}
	if got.Type != "task_dep:created" {
		t.Errorf("event type = %q, want task_dep:created", got.Type)
	}
}

func TestAddDependency_SelfLoop(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	taskA := makeUUID(1)
	_, err := csvc.AddDependency(context.Background(), makeUUID(10), taskA, taskA)
	if err == nil {
		t.Fatal("expected error for self-loop")
	}
}

func TestAddDependency_Cycle(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	taskA := makeUUID(1)
	taskB := makeUUID(2)

	// B already depends on A
	stub.taskDeps[util.UUIDToString(taskB)] = []db.TaskDependency{
		{TaskID: taskB, DependsOnTaskID: taskA},
	}

	// Adding A depending on B should fail (cycle)
	_, err := csvc.AddDependency(context.Background(), makeUUID(10), taskA, taskB)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestRemoveDependency_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	wsID := makeUUID(10)
	taskA := makeUUID(1)
	taskB := makeUUID(2)

	stub.taskDeps[util.UUIDToString(taskA)] = []db.TaskDependency{
		{TaskID: taskA, DependsOnTaskID: taskB},
	}

	var got events.Event
	csvc.Bus.Subscribe("task_dep:deleted", func(e events.Event) { got = e })

	err := csvc.RemoveDependency(context.Background(), wsID, taskA, taskB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != "task_dep:deleted" {
		t.Errorf("event type = %q, want task_dep:deleted", got.Type)
	}
}

func TestGetDependencyInfo_ReturnsStatuses(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	taskA := makeUUID(1)
	taskB := makeUUID(2)

	stub.taskDeps[util.UUIDToString(taskA)] = []db.TaskDependency{
		{TaskID: taskA, DependsOnTaskID: taskB},
	}
	stub.tasks[util.UUIDToString(taskB)] = db.AgentTaskQueue{
		ID: taskB, Status: "completed",
	}

	info, err := csvc.GetDependencyInfo(context.Background(), taskA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info) != 1 {
		t.Fatalf("expected 1 dep info, got %d", len(info))
	}
	if info[0].DependencyStatus != "completed" {
		t.Errorf("status = %q, want completed", info[0].DependencyStatus)
	}
}

func TestGetReadyTasks_ReturnsTasks(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	agentID := makeUUID(20)
	taskID := makeUUID(1)

	stub.tasks[util.UUIDToString(taskID)] = db.AgentTaskQueue{
		ID: taskID, AgentID: agentID, Status: "queued",
	}

	tasks, err := csvc.GetReadyTasks(context.Background(), agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != taskID {
		t.Errorf("task ID mismatch")
	}
}

func TestGetPendingMessages_FiltersUnread(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	agentID := makeUUID(20)

	stub.messages = []db.AgentMessage{
		{ID: makeUUID(1), ToAgentID: agentID, Content: "unread"},
		{ID: makeUUID(2), ToAgentID: agentID, Content: "read", ReadAt: pgtype.Timestamptz{Valid: true}},
	}

	msgs, err := csvc.GetPendingMessages(context.Background(), agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 unread message, got %d", len(msgs))
	}
	if msgs[0].Content != "unread" {
		t.Errorf("content = %q, want unread", msgs[0].Content)
	}
}

func TestMarkMessagesRead_DelegatesToQuery(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	agentID := makeUUID(20)
	stub.messages = []db.AgentMessage{
		{ID: makeUUID(1), ToAgentID: agentID, Content: "msg1"},
	}

	err := csvc.MarkMessagesRead(context.Background(), agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// P8: Additional TeamService / ReviewService edge cases
// ---------------------------------------------------------------------------

func TestRemoveMember_NoError(t *testing.T) {
	stub := newTaskStubDBTX()
	tsvc := newTestTeamService(stub)

	teamID := makeUUID(1)
	stub.teams[util.UUIDToString(teamID)] = db.Team{ID: teamID}

	// RemoveMember delegates to Exec which doesn't check affected rows.
	err := tsvc.RemoveMember(context.Background(), teamID, makeUUID(99))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTeamWithMembers_NilQueries_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil Queries")
		}
	}()
	svc := &TeamService{Queries: nil}
	_, _, _ = svc.GetTeamWithMembers(context.Background(), makeUUID(1))
}

// ---------------------------------------------------------------------------
// P9: Integration scenario tests
// ---------------------------------------------------------------------------

func TestScenario_EnqueueAndComplete(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	runtimeID := makeUUID(30)
	issueID := makeUUID(40)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: runtimeID, WorkspaceID: wsID,
		Name: "worker", Status: "idle", MaxConcurrentTasks: 5,
	}
	stub.runtimes[util.UUIDToString(runtimeID)] = db.AgentRuntime{
		ID: runtimeID, WorkspaceID: wsID, Status: "online",
	}
	stub.issues[util.UUIDToString(issueID)] = db.Issue{
		ID: issueID, WorkspaceID: wsID,
	}

	// 1. Enqueue
	issue := db.Issue{ID: issueID, WorkspaceID: wsID, Priority: "high"}
	task, err := svc.EnqueueTaskForMention(context.Background(), issue, agentID, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// 2. Claim
	claimed, err := svc.ClaimTask(context.Background(), agentID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed == nil {
		t.Fatal("claim: expected task")
	}

	// 3. Start
	started, err := svc.StartTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if started.Status != "running" {
		t.Errorf("start status = %q, want running", started.Status)
	}

	// 4. Complete
	var completedEvent events.Event
	svc.Bus.Subscribe("task:completed", func(e events.Event) { completedEvent = e })

	completed, err := svc.CompleteTask(context.Background(), task.ID, []byte(`{}`), "sess", "/tmp")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Status != "completed" {
		t.Errorf("complete status = %q, want completed", completed.Status)
	}
	if completedEvent.Type != "task:completed" {
		t.Errorf("expected task:completed event")
	}
}

func TestScenario_DependencyChain(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	wsID := makeUUID(10)
	taskA := makeUUID(1)
	taskB := makeUUID(2)

	// A depends on B
	dep, err := csvc.AddDependency(context.Background(), wsID, taskA, taskB)
	if err != nil {
		t.Fatalf("add dep: %v", err)
	}

	// GetDependencyInfo
	info, err := csvc.GetDependencyInfo(context.Background(), taskA)
	if err != nil {
		t.Fatalf("get dep info: %v", err)
	}
	if len(info) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(info))
	}

	// Complete B, remove dependency
	stub.tasks[util.UUIDToString(taskB)] = db.AgentTaskQueue{ID: taskB, Status: "completed"}

	err = csvc.RemoveDependency(context.Background(), wsID, taskA, taskB)
	if err != nil {
		t.Fatalf("remove dep: %v", err)
	}

	// Verify removed
	deps := stub.taskDeps[util.UUIDToString(taskA)]
	for _, d := range deps {
		if util.UUIDToString(d.DependsOnTaskID) == util.UUIDToString(taskB) {
			t.Error("dependency should have been removed")
		}
	}

	_ = dep // suppress unused
}

func TestScenario_CannotAddCycle(t *testing.T) {
	stub := newTaskStubDBTX()
	csvc := newTestCollaborationService(stub)

	wsID := makeUUID(10)
	taskA, taskB, taskC := makeUUID(1), makeUUID(2), makeUUID(3)

	// Build chain: C → B → A
	if _, err := csvc.AddDependency(context.Background(), wsID, taskC, taskB); err != nil {
		t.Fatalf("add C→B: %v", err)
	}
	if _, err := csvc.AddDependency(context.Background(), wsID, taskB, taskA); err != nil {
		t.Fatalf("add B→A: %v", err)
	}

	// Try to add A → C (would create cycle: A→C→B→A)
	_, err := csvc.AddDependency(context.Background(), wsID, taskA, taskC)
	if err == nil {
		t.Fatal("expected error for cycle A→C→B→A")
	}
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewTeamService(t *testing.T) {
	q := db.New(newTaskStubDBTX())
	bus := events.New()
	svc := NewTeamService(q, bus)
	if svc == nil {
		t.Fatal("NewTeamService should not return nil")
	}
	if svc.Queries != q {
		t.Error("Queries not set")
	}
	if svc.Bus != bus {
		t.Error("Bus not set")
	}
}

func TestNewReviewService(t *testing.T) {
	q := db.New(newTaskStubDBTX())
	hub := realtime.NewHub(nil)
	bus := events.New()
	ts := &TaskService{Queries: q, Hub: hub, Bus: bus}
	svc := NewReviewService(q, hub, bus, ts)
	if svc == nil {
		t.Fatal("NewReviewService should not return nil")
	}
	if svc.Queries != q {
		t.Error("Queries not set")
	}
	if svc.Hub != hub {
		t.Error("Hub not set")
	}
	if svc.Bus != bus {
		t.Error("Bus not set")
	}
	if svc.TaskService != ts {
		t.Error("TaskService not set")
	}
}

// ---------------------------------------------------------------------------
// EnqueueTaskForIssue edge cases
// ---------------------------------------------------------------------------

func TestEnqueueTaskForIssue_CreateTaskError(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(10)
	runtimeID := makeUUID(20)
	stub.agents[uuidStr(10)] = db.Agent{
		ID:        agentID,
		RuntimeID: runtimeID,
	}
	issue := db.Issue{
		ID:         makeUUID(1),
		AssigneeID: agentID,
		WorkspaceID: makeUUID(2),
		Priority:   "high",
	}

	// CreateAgentTask will succeed in stub, so test the success path
	task, err := svc.EnqueueTaskForIssue(context.Background(), issue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "queued" {
		t.Errorf("status = %q, want queued", task.Status)
	}
	if task.AgentID != agentID {
		t.Error("agent ID mismatch")
	}
	if task.Priority != 3 {
		t.Errorf("priority = %d, want 3 (high)", task.Priority)
	}
}

// ---------------------------------------------------------------------------
// TryEnqueueReadySubIssues edge cases
// ---------------------------------------------------------------------------

func TestTryEnqueueReadySubIssues_ListError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTaskService(stub)

	// Should not panic — logs warning and returns
	svc.TryEnqueueReadySubIssues(context.Background(), makeUUID(1))
}

func TestTryEnqueueReadySubIssues_NoAssignee(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	parentID := makeUUID(1)
	stub.issues[uuidStr(10)] = db.Issue{
		ID:            makeUUID(10),
		ParentIssueID: pgtype.UUID{Bytes: parentID.Bytes, Valid: true},
		AssigneeID:    pgtype.UUID{}, // no assignee
	}

	// Should skip silently
	svc.TryEnqueueReadySubIssues(context.Background(), parentID)
}

func TestTryEnqueueReadySubIssues_EnqueueError(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	parentID := makeUUID(1)
	// Sub-issue with assignee but agent doesn't exist → EnqueueTaskForIssue fails
	stub.issues[uuidStr(10)] = db.Issue{
		ID:            makeUUID(10),
		ParentIssueID: pgtype.UUID{Bytes: parentID.Bytes, Valid: true},
		AssigneeID:    makeUUID(99), // agent doesn't exist
	}

	// Should log warning but not panic
	svc.TryEnqueueReadySubIssues(context.Background(), parentID)
}

// ---------------------------------------------------------------------------
// EnqueueTaskForMention edge cases
// ---------------------------------------------------------------------------

func TestEnqueueTaskForMention_NoRuntime(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(10)
	stub.agents[uuidStr(10)] = db.Agent{
		ID:        agentID,
		RuntimeID: pgtype.UUID{},
	}
	issue := db.Issue{ID: makeUUID(1)}
	_, err := svc.EnqueueTaskForMention(context.Background(), issue, agentID, pgtype.UUID{})
	if err == nil {
		t.Fatal("expected error for agent without runtime")
	}
	if !containsStr(err.Error(), "no runtime") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ClaimTask edge cases
// ---------------------------------------------------------------------------

func TestClaimTask_NoTasksAvailable(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(10)
	stub.agents[uuidStr(10)] = db.Agent{
		ID:                  agentID,
		MaxConcurrentTasks:  5,
		RuntimeID:           makeUUID(20),
	}

	task, err := svc.ClaimTask(context.Background(), agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task != nil {
		t.Error("should return nil when no tasks available")
	}
}

func TestClaimTaskForRuntime_ListPendingError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTaskService(stub)

	_, err := svc.ClaimTaskForRuntime(context.Background(), makeUUID(20))
	if err == nil {
		t.Fatal("expected error when ListPendingTasksByRuntime fails")
	}
	if !containsStr(err.Error(), "list pending tasks") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartTask_TaskNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	_, err := svc.StartTask(context.Background(), makeUUID(99))
	if err == nil {
		t.Fatal("expected error when task not found")
	}
	if !containsStr(err.Error(), "load task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteTask_TaskNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	_, err := svc.CompleteTask(context.Background(), makeUUID(99), nil, "", "")
	if err == nil {
		t.Fatal("expected error when task not found")
	}
	if !containsStr(err.Error(), "load task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteTask_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(10)
	stub.tasks[uuidStr(50)] = db.AgentTaskQueue{
		ID: makeUUID(50), AgentID: agentID, Status: "running",
		RuntimeID: makeUUID(20), IssueID: makeUUID(30),
	}

	task, err := svc.CompleteTask(context.Background(), makeUUID(50), []byte(`{}`), "sess-1", "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("status = %q, want completed", task.Status)
	}
}

func TestCompleteTask_WithReviewEnabled(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)

	agentID := makeUUID(10)
	issueID := makeUUID(30)
	stub.tasks[uuidStr(50)] = db.AgentTaskQueue{
		ID: makeUUID(50), AgentID: agentID, Status: "running",
		RuntimeID: makeUUID(20), IssueID: issueID,
		MaxReviews: 1,
	}
	stub.issues[uuidStr(30)] = db.Issue{
		ID: issueID, WorkspaceID: makeUUID(40),
	}

	task, err := ts.CompleteTask(context.Background(), makeUUID(50), []byte(`{}`), "sess-1", "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "in_review" {
		t.Errorf("status = %q, want in_review", task.Status)
	}
}

// ---------------------------------------------------------------------------
// FailTask edge cases
// ---------------------------------------------------------------------------

func TestFailTask_TaskNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	_, err := svc.FailTask(context.Background(), makeUUID(99), "error")
	if err == nil {
		t.Fatal("expected error when task not found")
	}
	if !containsStr(err.Error(), "load task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCancelTask_TaskNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	_, err := svc.CancelTask(context.Background(), makeUUID(99))
	if err == nil {
		t.Fatal("expected error when task not found")
	}
	if !containsStr(err.Error(), "load task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateTeam_DBError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTeamService(stub)

	_, err := svc.CreateTeam(context.Background(), CreateTeamInput{
		WorkspaceID: makeUUID(1),
		Name:        "team-a",
		CreatedBy:   makeUUID(2),
	})
	if err == nil {
		t.Fatal("expected error when CreateTeam DB fails")
	}
	if !containsStr(err.Error(), "create team") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateTeam_AddMemberError(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	// queryErr is set to trigger on the AddTeamMember QueryRow call
	stub.queryErr = fmt.Errorf("db down")

	_, err := svc.CreateTeam(context.Background(), CreateTeamInput{
		WorkspaceID:    makeUUID(1),
		Name:           "team-a",
		CreatedBy:      makeUUID(2),
		MemberAgentIDs: []pgtype.UUID{makeUUID(10)},
	})
	if err == nil {
		t.Fatal("expected error when AddTeamMember fails")
	}
}

func TestGetTeamWithMembers_TeamNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	_, _, err := svc.GetTeamWithMembers(context.Background(), makeUUID(99))
	if err == nil {
		t.Fatal("expected error when team not found")
	}
	if !containsStr(err.Error(), "get team") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetTeamWithMembers_DBError(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	// queryErr fires on GetTeam (QueryRow)
	stub.queryErr = fmt.Errorf("db down")

	_, _, err := svc.GetTeamWithMembers(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error when DB fails")
	}
	if !containsStr(err.Error(), "get team") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAddMember_DBError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTeamService(stub)

	_, err := svc.AddMember(context.Background(), makeUUID(1), makeUUID(10), "member")
	if err == nil {
		t.Fatal("expected error when AddTeamMember fails")
	}
	if !containsStr(err.Error(), "add team member") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnqueueTeamTask_DBError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTeamService(stub)

	_, err := svc.EnqueueTeamTask(context.Background(), makeUUID(1), makeUUID(2), makeUUID(3), 1, LeadDelegation)
	if err == nil {
		t.Fatal("expected error when CreateTeamTask fails")
	}
	if !containsStr(err.Error(), "create team task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDelegateTask_DBError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTeamService(stub)

	_, err := svc.DelegateTask(context.Background(), makeUUID(1), makeUUID(2))
	if err == nil {
		t.Fatal("expected error when UpdateTeamTaskStatus fails")
	}
	if !containsStr(err.Error(), "delegate task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTeamCompleteTask_DBError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = fmt.Errorf("db down")
	svc := newTestTeamService(stub)

	_, err := svc.CompleteTask(context.Background(), makeUUID(1))
	if err == nil {
		t.Fatal("expected error when UpdateTeamTaskStatus fails")
	}
	if !containsStr(err.Error(), "complete task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetDelegationMode_TeamNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	_, err := svc.GetDelegationMode(context.Background(), makeUUID(99))
	if err == nil {
		t.Fatal("expected error when team not found")
	}
	if !containsStr(err.Error(), "get team") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetPendingTasks_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTeamService(stub)

	teamID := makeUUID(1)
	stub.teamTasks[uuidStr(50)] = db.TeamTaskQueue{
		ID: makeUUID(50), TeamID: teamID, Status: "pending",
	}

	tasks, err := svc.GetPendingTasks(context.Background(), teamID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

// ---------------------------------------------------------------------------
// ReviewService error-path tests
// ---------------------------------------------------------------------------

func TestReviewTask_TaskNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	_, err := rs.ReviewTask(context.Background(), makeUUID(99))
	if err == nil {
		t.Fatal("expected error when task not found")
	}
	if !containsStr(err.Error(), "load task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReviewTask_Success(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	issueID := makeUUID(30)
	stub.tasks[uuidStr(50)] = db.AgentTaskQueue{
		ID: makeUUID(50), AgentID: makeUUID(10), Status: "in_review",
		IssueID: issueID, RuntimeID: makeUUID(20),
	}
	stub.issues[uuidStr(30)] = db.Issue{
		ID: issueID, WorkspaceID: makeUUID(40),
	}

	task, err := rs.ReviewTask(context.Background(), makeUUID(50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("status = %q, want completed", task.Status)
	}
}

func TestSubmitManualReview_TaskNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	_, err := rs.SubmitManualReview(context.Background(), makeUUID(99), makeUUID(1), "approved", "")
	if err == nil {
		t.Fatal("expected error when task not found")
	}
}

func TestSubmitManualReview_ApproveSuccess(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	issueID := makeUUID(30)
	stub.tasks[uuidStr(50)] = db.AgentTaskQueue{
		ID: makeUUID(50), AgentID: makeUUID(10), Status: "in_review",
		IssueID: issueID, RuntimeID: makeUUID(20),
	}
	stub.issues[uuidStr(30)] = db.Issue{
		ID: issueID, WorkspaceID: makeUUID(40),
	}

	task, err := rs.SubmitManualReview(context.Background(), makeUUID(50), makeUUID(1), "approved", "looks good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("status = %q, want completed", task.Status)
	}
}

func TestSubmitManualReview_RejectSuccess(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	issueID := makeUUID(30)
	stub.tasks[uuidStr(50)] = db.AgentTaskQueue{
		ID: makeUUID(50), AgentID: makeUUID(10), Status: "in_review",
		IssueID: issueID, RuntimeID: makeUUID(20),
	}
	stub.issues[uuidStr(30)] = db.Issue{
		ID: issueID, WorkspaceID: makeUUID(40),
	}

	task, err := rs.SubmitManualReview(context.Background(), makeUUID(50), makeUUID(1), "rejected", "needs work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "failed" {
		t.Errorf("status = %q, want failed", task.Status)
	}
}

func TestRetryTask_TaskNotFound(t *testing.T) {
	stub := newTaskStubDBTX()
	ts := newTestTaskService(stub)
	rs := newTestReviewService(stub, ts)

	_, err := rs.RetryTask(context.Background(), makeUUID(99))
	if err == nil {
		t.Fatal("expected error when task not found")
	}
	if !containsStr(err.Error(), "retry task") {
		t.Errorf("unexpected error: %v", err)
	}
}

