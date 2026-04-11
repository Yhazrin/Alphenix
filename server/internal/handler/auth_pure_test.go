package handler

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ── defaultWorkspaceName ───────────────────────────────────────────────────

func TestDefaultWorkspaceName_WithName(t *testing.T) {
	user := db.User{Name: "Alice", Email: "alice@example.com"}
	got := defaultWorkspaceName(user)
	if got != "Alice's Workspace" {
		t.Errorf("got %q, want %q", got, "Alice's Workspace")
	}
}

func TestDefaultWorkspaceName_EmptyNameUsesEmailPrefix(t *testing.T) {
	user := db.User{Name: "", Email: "bob@example.com"}
	got := defaultWorkspaceName(user)
	if got != "bob's Workspace" {
		t.Errorf("got %q, want %q", got, "bob's Workspace")
	}
}

func TestDefaultWorkspaceName_WhitespaceNameUsesEmail(t *testing.T) {
	user := db.User{Name: "   ", Email: "carol@test.org"}
	got := defaultWorkspaceName(user)
	if got != "carol's Workspace" {
		t.Errorf("got %q, want %q", got, "carol's Workspace")
	}
}

func TestDefaultWorkspaceName_FallbackToPersonal(t *testing.T) {
	user := db.User{Name: "", Email: ""}
	got := defaultWorkspaceName(user)
	if got != "Personal's Workspace" {
		t.Errorf("got %q, want %q", got, "Personal's Workspace")
	}
}

func TestDefaultWorkspaceName_InvalidEmailFallback(t *testing.T) {
	// Email without @ — at=-1, so name stays empty → falls back to Personal.
	user := db.User{Name: "", Email: "notanemail"}
	got := defaultWorkspaceName(user)
	if got != "Personal's Workspace" {
		t.Errorf("got %q, want %q", got, "Personal's Workspace")
	}
}

func TestDefaultWorkspaceName_EmailAtStart(t *testing.T) {
	// Email starting with @ — at=0, so name stays empty → falls back to Personal.
	user := db.User{Name: "", Email: "@example.com"}
	got := defaultWorkspaceName(user)
	if got != "Personal's Workspace" {
		t.Errorf("got %q, want %q", got, "Personal's Workspace")
	}
}

// ── defaultWorkspaceSlug ──────────────────────────────────────────────────

func TestDefaultWorkspaceSlug_UsesName(t *testing.T) {
	id := [16]byte{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11}
	user := db.User{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Name:  "Alice",
		Email: "alice@example.com",
	}
	got := defaultWorkspaceSlug(user)
	if got == "" {
		t.Error("slug should not be empty")
	}
	// Should start with "alice" (slugified name).
	if len(got) < 9 { // "alice" + "-" + at least 8 hex chars
		t.Errorf("slug %q too short", got)
	}
}

func TestDefaultWorkspaceSlug_FallsBackToEmail(t *testing.T) {
	id := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	user := db.User{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Name:  "",
		Email: "bob@example.com",
	}
	got := defaultWorkspaceSlug(user)
	// Should use email prefix "bob" as slug base.
	if got == "" {
		t.Error("slug should not be empty")
	}
}

func TestDefaultWorkspaceSlug_FallbackToWorkspace_Empty(t *testing.T) {
	id := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	user := db.User{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Name:  "",
		Email: "",
	}
	got := defaultWorkspaceSlug(user)
	// No name, no email → base "workspace".
	if got == "" {
		t.Error("slug should not be empty")
	}
}

func TestDefaultWorkspaceSlug_SpecialCharsStripped(t *testing.T) {
	id := [16]byte{0xaa, 0xbb, 0xcc, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	user := db.User{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Name:  "Hello World!",
		Email: "hw@test.com",
	}
	got := defaultWorkspaceSlug(user)
	if !strings.HasPrefix(got, "hello-world-") {
		t.Errorf("expected 'hello-world-' prefix, got %q", got)
	}
}

func TestDefaultWorkspaceSlug_ContainsUUIDSuffix(t *testing.T) {
	id := makeTestUUID("user000001")
	user := db.User{
		ID:    id,
		Name:  "Test",
		Email: "t@t.com",
	}
	got := defaultWorkspaceSlug(user)
	parts := strings.Split(got, "-")
	if len(parts) < 2 {
		t.Errorf("slug should contain dash-separated UUID suffix, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// generateCode
// ---------------------------------------------------------------------------

func TestGenerateCode_Length(t *testing.T) {
	code, err := generateCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Errorf("code length = %d, want 6", len(code))
	}
}

func TestGenerateCode_AllDigits(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("code %q contains non-digit %q", code, string(c))
			}
		}
	}
}

func TestGenerateCode_ZeroPadded(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 {
			t.Errorf("code %q is not 6 chars", code)
		}
	}
}

func TestGenerateCode_Variety(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatal(err)
		}
		seen[code] = true
	}
	// With 200 draws from 1M possibilities, we should get at least 190 unique.
	if len(seen) < 190 {
		t.Errorf("only %d unique codes in 200 draws — suspiciously low", len(seen))
	}
}
