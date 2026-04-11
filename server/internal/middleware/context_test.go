package middleware

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

func TestSetMemberContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	member := db.Member{Role: "owner"}

	ctx = SetMemberContext(ctx, "ws-123", member)

	gotID := WorkspaceIDFromContext(ctx)
	if gotID != "ws-123" {
		t.Errorf("WorkspaceIDFromContext() = %q, want %q", gotID, "ws-123")
	}

	gotMember, ok := MemberFromContext(ctx)
	if !ok {
		t.Fatal("MemberFromContext() returned false")
	}
	if gotMember.Role != "owner" {
		t.Errorf("Role = %q, want %q", gotMember.Role, "owner")
	}
}

func TestSetMemberContext_Overwrites(t *testing.T) {
	ctx := context.Background()
	ctx = SetMemberContext(ctx, "ws-1", db.Member{Role: "member"})
	ctx = SetMemberContext(ctx, "ws-2", db.Member{Role: "admin"})

	if got := WorkspaceIDFromContext(ctx); got != "ws-2" {
		t.Errorf("should overwrite workspace ID, got %q", got)
	}
	m, _ := MemberFromContext(ctx)
	if m.Role != "admin" {
		t.Errorf("should overwrite member, got %q", m.Role)
	}
}

func TestMemberFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	_, ok := MemberFromContext(ctx)
	if ok {
		t.Error("empty context should return false for member")
	}
}

func TestWorkspaceIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	got := WorkspaceIDFromContext(ctx)
	if got != "" {
		t.Errorf("empty context should return empty workspace ID, got %q", got)
	}
}

// Suppress unused import
var _ = pgtype.UUID{}
