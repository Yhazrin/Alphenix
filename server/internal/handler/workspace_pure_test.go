package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// ── generateIssuePrefix ──────────────────────────────────────────────────────

func TestGenerateIssuePrefix_Table(t *testing.T) {
	tests := []struct {
		name string
		input string
		want string
	}{
		{"simple name", "My Workspace", "MYW"},
		{"single word", "Jiayuan", "JIA"},
		{"with apostrophe", "Jiayuan's Workspace", "JIA"},
		{"two chars", "AB", "AB"},
		{"single char", "X", "X"},
		{"empty string", "", "WS"},
		{"only non-alpha", "123!@#", "WS"},
		{"numbers mixed", "Team1Workspace", "TEA"},
		{"long name", "Engineering Workspace", "ENG"},
		{"underscored", "my_workspace", "MYW"},
		{"hyphenated", "my-workspace", "MYW"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateIssuePrefix(tt.input)
			if got != tt.want {
				t.Errorf("generateIssuePrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateIssuePrefix_Length(t *testing.T) {
	// All prefixes should be at most 3 chars (or the full alpha string if shorter).
	for _, input := range []string{"ABCDEFGHIJ", "Workspace", "A"} {
		got := generateIssuePrefix(input)
		if len(got) > 3 {
			t.Errorf("generateIssuePrefix(%q) = %q (len %d), want <= 3", input, got, len(got))
		}
	}
}

// ── workspaceToResponse ──────────────────────────────────────────────────────

func TestWorkspaceToResponse(t *testing.T) {
	ws := db.Workspace{
		ID:          pgtype.UUID{Bytes: [16]byte{1, 2, 3}, Valid: true},
		Name:        "Test Workspace",
		Slug:        "test-workspace",
		Description: pgtype.Text{String: "A test", Valid: true},
		Context:     pgtype.Text{String: "some context", Valid: true},
		Settings:    []byte(`{"theme":"dark"}`),
		Repos:       []byte(`[{"url":"https://github.com/x/y"}]`),
		IssuePrefix: "TES",
		CreatedAt:   pgtype.Timestamptz{Time: mustParseTime("2026-01-01T00:00:00Z"), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: mustParseTime("2026-01-02T00:00:00Z"), Valid: true},
	}

	resp := workspaceToResponse(ws)

	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.Name != "Test Workspace" {
		t.Errorf("Name = %q, want %q", resp.Name, "Test Workspace")
	}
	if resp.Slug != "test-workspace" {
		t.Errorf("Slug = %q, want %q", resp.Slug, "test-workspace")
	}
	if resp.Description == nil || *resp.Description != "A test" {
		t.Error("Description should be 'A test'")
	}
	if resp.Context == nil || *resp.Context != "some context" {
		t.Error("Context should be 'some context'")
	}
	if resp.IssuePrefix != "TES" {
		t.Errorf("IssuePrefix = %q, want %q", resp.IssuePrefix, "TES")
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
	if resp.Settings == nil {
		t.Error("Settings should not be nil")
	}
	if resp.Repos == nil {
		t.Error("Repos should not be nil")
	}
}

func TestWorkspaceToResponse_NullFields(t *testing.T) {
	ws := db.Workspace{
		ID:   pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Name: "Minimal",
		Slug: "minimal",
		// Description, Context: null Text
		// Settings, Repos: nil byte slices
		IssuePrefix: "MIN",
	}

	resp := workspaceToResponse(ws)
	if resp.Description != nil {
		t.Error("null Description should map to nil pointer")
	}
	if resp.Context != nil {
		t.Error("null Context should map to nil pointer")
	}
	// Settings should default to empty map, Repos to empty slice.
	settings, ok := resp.Settings.(map[string]any)
	if !ok || len(settings) != 0 {
		t.Error("null Settings should default to empty map")
	}
}

// ── memberToResponse ─────────────────────────────────────────────────────────

func TestMemberToResponse(t *testing.T) {
	m := db.Member{
		ID:          pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: [16]byte{20}, Valid: true},
		UserID:      pgtype.UUID{Bytes: [16]byte{30}, Valid: true},
		Role:        "admin",
		CreatedAt:   pgtype.Timestamptz{Time: mustParseTime("2026-03-15T10:00:00Z"), Valid: true},
	}

	resp := memberToResponse(m)

	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.WorkspaceID == "" {
		t.Error("WorkspaceID should not be empty")
	}
	if resp.UserID == "" {
		t.Error("UserID should not be empty")
	}
	if resp.Role != "admin" {
		t.Errorf("Role = %q, want %q", resp.Role, "admin")
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

// ── memberWithUserResponse ───────────────────────────────────────────────────

func TestMemberWithUserResponse(t *testing.T) {
	member := db.Member{
		ID:          pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		UserID:      pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		Role:        "member",
		CreatedAt:   pgtype.Timestamptz{Time: mustParseTime("2026-02-01T00:00:00Z"), Valid: true},
	}
	user := db.User{
		ID:        pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		Name:      "Alice",
		Email:     "alice@example.com",
		AvatarUrl: pgtype.Text{String: "https://example.com/avatar.png", Valid: true},
	}

	resp := memberWithUserResponse(member, user)

	if resp.Name != "Alice" {
		t.Errorf("Name = %q, want %q", resp.Name, "Alice")
	}
	if resp.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", resp.Email, "alice@example.com")
	}
	if resp.AvatarURL == nil || *resp.AvatarURL != "https://example.com/avatar.png" {
		t.Error("AvatarURL should be set")
	}
	if resp.Role != "member" {
		t.Errorf("Role = %q, want %q", resp.Role, "member")
	}
}

func TestMemberWithUserResponse_NullAvatar(t *testing.T) {
	member := db.Member{
		ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
	}
	user := db.User{
		Name:  "Bob",
		Email: "bob@example.com",
		// AvatarUrl: null
	}

	resp := memberWithUserResponse(member, user)
	if resp.AvatarURL != nil {
		t.Error("null AvatarUrl should map to nil pointer")
	}
}

// ── normalizeMemberRole ──────────────────────────────────────────────────────

func TestNormalizeMemberRole(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantOK  bool
	}{
		{"", "member", true},
		{"owner", "owner", true},
		{"admin", "admin", true},
		{"member", "member", true},
		{"  admin  ", "admin", true},
		{"manager", "", false},
		{"OWNER", "", false},
		{"  ", "member", true}, // whitespace-only trims to empty → default
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := normalizeMemberRole(tt.input)
			if ok != tt.wantOK {
				t.Errorf("normalizeMemberRole(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("normalizeMemberRole(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
