package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// TaskQuerier abstracts the database operations needed by TaskService,
// SelectRuntime, and RunStartupSequence.  Implementations: *db.Queries
// (production), stubTaskQueries (tests).
type TaskQuerier interface {
	// Agent operations
	GetAgent(ctx context.Context, id pgtype.UUID) (db.Agent, error)
	UpdateAgentStatus(ctx context.Context, arg db.UpdateAgentStatusParams) (db.Agent, error)

	// Task queue operations
	CreateAgentTask(ctx context.Context, arg db.CreateAgentTaskParams) (db.AgentTaskQueue, error)
	GetAgentTask(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error)
	StartAgentTask(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error)
	CompleteAgentTask(ctx context.Context, arg db.CompleteAgentTaskParams) (db.AgentTaskQueue, error)
	FailAgentTask(ctx context.Context, arg db.FailAgentTaskParams) (db.AgentTaskQueue, error)
	SetTaskInReview(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error)
	CancelAgentTask(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error)
	ClaimAgentTask(ctx context.Context, agentID pgtype.UUID) (db.AgentTaskQueue, error)
	CountRunningTasks(ctx context.Context, agentID pgtype.UUID) (int64, error)
	ListActiveTasksByIssue(ctx context.Context, issueID pgtype.UUID) ([]db.AgentTaskQueue, error)
	CancelAgentTasksByIssue(ctx context.Context, issueID pgtype.UUID) error
	ListPendingTasksByRuntime(ctx context.Context, runtimeID pgtype.UUID) ([]db.AgentTaskQueue, error)

	// Skill operations
	ListAgentSkills(ctx context.Context, agentID pgtype.UUID) ([]db.Skill, error)
	ListSkillFiles(ctx context.Context, skillID pgtype.UUID) ([]db.SkillFile, error)

	// Issue operations
	GetIssue(ctx context.Context, id pgtype.UUID) (db.Issue, error)
	GetIssueByNumber(ctx context.Context, arg db.GetIssueByNumberParams) (db.Issue, error)
	ListReadySubIssues(ctx context.Context, parentIssueID pgtype.UUID) ([]db.Issue, error)

	// Workspace operations
	GetWorkspace(ctx context.Context, id pgtype.UUID) (db.Workspace, error)

	// Dependency operations
	GetTaskDependents(ctx context.Context, taskID pgtype.UUID) ([]db.TaskDependency, error)
	GetTaskDependencies(ctx context.Context, taskID pgtype.UUID) ([]db.TaskDependency, error)

	// Comment operations
	CreateComment(ctx context.Context, arg db.CreateCommentParams) (db.Comment, error)

	// Runtime operations
	GetRuntimePolicyByAgent(ctx context.Context, arg db.GetRuntimePolicyByAgentParams) (db.RuntimeAssignmentPolicy, error)
	ListActiveRuntimePolicies(ctx context.Context, workspaceID pgtype.UUID) ([]db.RuntimeAssignmentPolicy, error)
	ListAgentRuntimes(ctx context.Context, workspaceID pgtype.UUID) ([]db.AgentRuntime, error)
	CountPendingTasksByRuntime(ctx context.Context, runtimeID pgtype.UUID) (int64, error)
}
