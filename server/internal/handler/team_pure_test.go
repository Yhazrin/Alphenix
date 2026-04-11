package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ── teamToResponse ───────────────────────────────────────────────────────────

func TestTeamToResponse(t *testing.T) {
	team := db.Team{
		ID:          pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		Name:        "Backend Team",
		Description: pgtype.Text{String: "Handles APIs", Valid: true},
		AvatarUrl:   pgtype.Text{String: "https://example.com/team.png", Valid: true},
		LeadAgentID: pgtype.UUID{Bytes: [16]byte{5}, Valid: true},
		CreatedBy:   pgtype.UUID{Bytes: [16]byte{6}, Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: mustParseTime("2026-01-10T00:00:00Z"), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: mustParseTime("2026-01-11T00:00:00Z"), Valid: true},
	}

	resp := teamToResponse(team)

	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.Name != "Backend Team" {
		t.Errorf("Name = %q, want %q", resp.Name, "Backend Team")
	}
	if resp.Description == nil || *resp.Description != "Handles APIs" {
		t.Error("Description should be 'Handles APIs'")
	}
	if resp.AvatarURL == nil || *resp.AvatarURL != "https://example.com/team.png" {
		t.Error("AvatarURL should be set")
	}
	if resp.LeadAgentID == nil {
		t.Error("LeadAgentID should not be nil when valid")
	}
	if resp.CreatedBy == nil {
		t.Error("CreatedBy should not be nil when valid")
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
	if resp.Members == nil {
		t.Error("Members should default to empty slice, not nil")
	}
}

func TestTeamToResponse_NullFields(t *testing.T) {
	team := db.Team{
		ID:   pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Name: "Minimal Team",
		// All nullable fields: null
	}

	resp := teamToResponse(team)

	if resp.Description != nil {
		t.Error("null Description should map to nil")
	}
	if resp.AvatarURL != nil {
		t.Error("null AvatarUrl should map to nil")
	}
	if resp.LeadAgentID != nil {
		t.Error("null LeadAgentID should map to nil")
	}
	if resp.CreatedBy != nil {
		t.Error("null CreatedBy should map to nil")
	}
	if resp.ArchivedAt != nil {
		t.Error("null ArchivedAt should map to nil")
	}
	if resp.ArchivedBy != nil {
		t.Error("null ArchivedBy should map to nil")
	}
	if len(resp.Members) != 0 {
		t.Error("Members should be empty slice")
	}
}

func TestTeamToResponse_ArchivedFields(t *testing.T) {
	team := db.Team{
		ID:         pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Name:       "Archived Team",
		ArchivedAt: pgtype.Timestamptz{Time: mustParseTime("2026-03-01T00:00:00Z"), Valid: true},
		ArchivedBy: pgtype.UUID{Bytes: [16]byte{7}, Valid: true},
	}

	resp := teamToResponse(team)
	if resp.ArchivedAt == nil {
		t.Error("ArchivedAt should not be nil when valid")
	}
	if resp.ArchivedBy == nil {
		t.Error("ArchivedBy should not be nil when valid")
	}
}

// ── teamMemberToResponse ─────────────────────────────────────────────────────

func TestTeamMemberToResponse(t *testing.T) {
	m := db.TeamMember{
		ID:       pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
		TeamID:   pgtype.UUID{Bytes: [16]byte{11}, Valid: true},
		AgentID:  pgtype.UUID{Bytes: [16]byte{12}, Valid: true},
		Role:     "lead",
		JoinedAt: pgtype.Timestamptz{Time: mustParseTime("2026-02-15T00:00:00Z"), Valid: true},
	}

	resp := teamMemberToResponse(m)

	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.TeamID == "" {
		t.Error("TeamID should not be empty")
	}
	if resp.AgentID == "" {
		t.Error("AgentID should not be empty")
	}
	if resp.Role != "lead" {
		t.Errorf("Role = %q, want %q", resp.Role, "lead")
	}
	if resp.JoinedAt == "" {
		t.Error("JoinedAt should not be empty")
	}
}

func TestTeamMemberToResponse_NullTimestamp(t *testing.T) {
	m := db.TeamMember{
		ID:      pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		TeamID:  pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		AgentID: pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		Role:    "member",
		// JoinedAt: null
	}

	resp := teamMemberToResponse(m)
	if resp.JoinedAt != "" {
		t.Errorf("null JoinedAt should produce empty string, got %q", resp.JoinedAt)
	}
}
