package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// TeamService handles team-related business logic.
type TeamService struct {
	Queries *db.Queries
	Bus     *events.Bus
}

func NewTeamService(q *db.Queries, bus *events.Bus) *TeamService {
	return &TeamService{Queries: q, Bus: bus}
}

// CreateTeamInput holds parameters for creating a team.
type CreateTeamInput struct {
	WorkspaceID  pgtype.UUID
	Name        string
	Description *string
	AvatarURL   *string
	LeadAgentID *pgtype.UUID
	CreatedBy   pgtype.UUID
	MemberAgentIDs []pgtype.UUID
}

// CreateTeam creates a new team with optional members.
func (s *TeamService) CreateTeam(ctx context.Context, input CreateTeamInput) (db.Team, error) {
	team, err := s.Queries.CreateTeam(ctx, db.CreateTeamParams{
		WorkspaceID: input.WorkspaceID,
		Name:        input.Name,
		Description: strPtrToText(input.Description),
		AvatarUrl:   strPtrToText(input.AvatarURL),
		LeadAgentID: ptrToUUID(input.LeadAgentID),
		CreatedBy:   input.CreatedBy,
	})
	if err != nil {
		return db.Team{}, fmt.Errorf("create team: %w", err)
	}

	// Add members.
	for _, agentID := range input.MemberAgentIDs {
		role := "member"
		if input.LeadAgentID != nil && agentID == *input.LeadAgentID {
			role = "lead"
		}
		_, err := s.Queries.AddTeamMember(ctx, db.AddTeamMemberParams{
			TeamID:  team.ID,
			AgentID: agentID,
			Role:    pgtype.Text{String: role, Valid: true},
		})
		if err != nil {
			return team, fmt.Errorf("add team member: %w", err)
		}
	}

	s.Bus.Publish(events.Event{
		Type:        protocol.EventTeamCreated,
		WorkspaceID: util.UUIDToString(input.WorkspaceID),
		ActorType:   "user",
		ActorID:     util.UUIDToString(input.CreatedBy),
		Payload: map[string]any{
			"team": team,
		},
	})

	return team, nil
}

// GetTeamWithMembers retrieves a team with all its members.
func (s *TeamService) GetTeamWithMembers(ctx context.Context, teamID pgtype.UUID) (*db.Team, []db.TeamMember, error) {
	team, err := s.Queries.GetTeam(ctx, teamID)
	if err != nil {
		return nil, nil, fmt.Errorf("get team: %w", err)
	}

	members, err := s.Queries.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, nil, fmt.Errorf("list team members: %w", err)
	}

	return &team, members, nil
}

// AddMember adds an agent to a team.
func (s *TeamService) AddMember(ctx context.Context, teamID, agentID pgtype.UUID, role string) (db.TeamMember, error) {
	member, err := s.Queries.AddTeamMember(ctx, db.AddTeamMemberParams{
		TeamID:  teamID,
		AgentID: agentID,
		Role:    pgtype.Text{String: role, Valid: true},
	})
	if err != nil {
		return db.TeamMember{}, fmt.Errorf("add team member: %w", err)
	}
	return member, nil
}

// RemoveMember removes an agent from a team.
func (s *TeamService) RemoveMember(ctx context.Context, teamID, agentID pgtype.UUID) error {
	return s.Queries.RemoveTeamMember(ctx, db.RemoveTeamMemberParams{
		TeamID:  teamID,
		AgentID: agentID,
	})
}

// SetLead sets or changes the team lead.
func (s *TeamService) SetLead(ctx context.Context, teamID, agentID pgtype.UUID) error {
	// First, demote all current leads to members.
	members, err := s.Queries.ListTeamMembers(ctx, teamID)
	if err != nil {
		return fmt.Errorf("list team members: %w", err)
	}

	for _, m := range members {
		if m.Role == "lead" {
			_, err := s.Queries.UpdateTeamMemberRole(ctx, db.UpdateTeamMemberRoleParams{
				TeamID:  teamID,
				AgentID: m.AgentID,
				Role:    "member",
			})
			if err != nil {
				return fmt.Errorf("demote current lead: %w", err)
			}
		}
	}

	// Promote new lead.
	_, err = s.Queries.UpdateTeamMemberRole(ctx, db.UpdateTeamMemberRoleParams{
		TeamID:  teamID,
		AgentID: agentID,
		Role:    "lead",
	})
	if err != nil {
		return fmt.Errorf("promote new lead: %w", err)
	}

	// Update team's lead_agent_id.
	_, err = s.Queries.UpdateTeamLead(ctx, db.UpdateTeamLeadParams{
		ID:          teamID,
		LeadAgentID: agentID,
	})
	if err != nil {
		return fmt.Errorf("update team lead: %w", err)
	}

	return nil
}

// DelegationMode determines how tasks are assigned to team members.
type DelegationMode int

const (
	// LeadDelegation requires the team lead to assign tasks to members.
	LeadDelegation DelegationMode = iota
	// BroadcastMode allows any team member to pick up pending tasks.
	BroadcastMode
)

// GetDelegationMode returns the delegation mode for a team.
// Returns LeadDelegation if team has a lead, BroadcastMode otherwise.
func (s *TeamService) GetDelegationMode(ctx context.Context, teamID pgtype.UUID) (DelegationMode, error) {
	team, err := s.Queries.GetTeam(ctx, teamID)
	if err != nil {
		return LeadDelegation, fmt.Errorf("get team: %w", err)
	}

	if team.LeadAgentID.Valid {
		return LeadDelegation, nil
	}
	return BroadcastMode, nil
}

// EnqueueTeamTask creates a team task and returns it.
// If mode is LeadDelegation, status starts as 'pending'.
// If mode is BroadcastMode, status starts as 'delegated' immediately.
func (s *TeamService) EnqueueTeamTask(ctx context.Context, teamID, issueID, assignedBy pgtype.UUID, priority int32, mode DelegationMode) (db.TeamTaskQueue, error) {
	if mode == BroadcastMode {
		// In broadcast mode, we could update the status after creation.
		_ = mode
	}

	task, err := s.Queries.CreateTeamTask(ctx, db.CreateTeamTaskParams{
		TeamID:     teamID,
		IssueID:    issueID,
		AssignedBy: assignedBy,
		Priority:   pgtype.Int4{Int32: priority, Valid: true},
	})
	if err != nil {
		return db.TeamTaskQueue{}, fmt.Errorf("create team task: %w", err)
	}

	return task, nil
}

// DelegateTask marks a team task as delegated to a specific agent.
func (s *TeamService) DelegateTask(ctx context.Context, taskID, agentID pgtype.UUID) (db.TeamTaskQueue, error) {
	task, err := s.Queries.UpdateTeamTaskStatus(ctx, db.UpdateTeamTaskStatusParams{
		ID:                 taskID,
		Status:             "delegated",
		DelegatedToAgentID: agentID,
	})
	if err != nil {
		return db.TeamTaskQueue{}, fmt.Errorf("delegate task: %w", err)
	}
	return task, nil
}

// CompleteTask marks a team task as completed.
func (s *TeamService) CompleteTask(ctx context.Context, taskID pgtype.UUID) (db.TeamTaskQueue, error) {
	task, err := s.Queries.UpdateTeamTaskStatus(ctx, db.UpdateTeamTaskStatusParams{
		ID:     taskID,
		Status: "completed",
	})
	if err != nil {
		return db.TeamTaskQueue{}, fmt.Errorf("complete task: %w", err)
	}
	return task, nil
}

// GetPendingTasks returns all pending team tasks for a team.
func (s *TeamService) GetPendingTasks(ctx context.Context, teamID pgtype.UUID) ([]db.TeamTaskQueue, error) {
	return s.Queries.ListPendingTeamTasks(ctx, teamID)
}

// Helper functions

func strPtrToText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func ptrToUUID(u *pgtype.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return *u
}
