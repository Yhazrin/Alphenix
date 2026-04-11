package middleware

import (
	"context"
	"testing"
)

func TestDaemonWorkspaceIDFromContext_Set(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKeyDaemonWorkspaceID, "ws-123")
	got := DaemonWorkspaceIDFromContext(ctx)
	if got != "ws-123" {
		t.Errorf("got %q, want %q", got, "ws-123")
	}
}

func TestDaemonWorkspaceIDFromContext_Unset(t *testing.T) {
	got := DaemonWorkspaceIDFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty string for unset context, got %q", got)
	}
}

func TestDaemonIDFromContext_Set(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKeyDaemonID, "daemon-456")
	got := DaemonIDFromContext(ctx)
	if got != "daemon-456" {
		t.Errorf("got %q, want %q", got, "daemon-456")
	}
}

func TestDaemonIDFromContext_Unset(t *testing.T) {
	got := DaemonIDFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty string for unset context, got %q", got)
	}
}

func TestDaemonContextKeys_Independent(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxKeyDaemonWorkspaceID, "ws-1")
	ctx = context.WithValue(ctx, ctxKeyDaemonID, "d-1")

	if got := DaemonWorkspaceIDFromContext(ctx); got != "ws-1" {
		t.Errorf("workspace ID = %q, want %q", got, "ws-1")
	}
	if got := DaemonIDFromContext(ctx); got != "d-1" {
		t.Errorf("daemon ID = %q, want %q", got, "d-1")
	}
}
