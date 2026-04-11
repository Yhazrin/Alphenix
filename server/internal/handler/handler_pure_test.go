package handler

import (
	"strings"
	"testing"
	"time"

	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// hashToken (handler-local copy)
// ---------------------------------------------------------------------------

func TestHashToken_Handler_Deterministic(t *testing.T) {
	h1 := hashToken("test-token")
	h2 := hashToken("test-token")
	if h1 != h2 {
		t.Error("hashToken should be deterministic")
	}
}

func TestHashToken_Handler_OutputLength(t *testing.T) {
	got := hashToken("anything")
	if len(got) != 64 {
		t.Errorf("hashToken output length = %d, want 64", len(got))
	}
}

func TestHashToken_Handler_DifferentInputs(t *testing.T) {
	h1 := hashToken("a")
	h2 := hashToken("b")
	if h1 == h2 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestHashToken_Handler_Empty(t *testing.T) {
	got := hashToken("")
	if len(got) != 64 {
		t.Errorf("hashToken(\"\") length = %d, want 64", len(got))
	}
}

// ---------------------------------------------------------------------------
// nullTimeToString
// ---------------------------------------------------------------------------

func TestNullTimeToString_Valid(t *testing.T) {
	ts := pgtype.Timestamptz{
		Time:  time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC),
		Valid: true,
	}
	got := nullTimeToString(ts)
	if got != "2026-03-15T10:30:00Z" {
		t.Errorf("got %q, want RFC3339", got)
	}
}

func TestNullTimeToString_Invalid(t *testing.T) {
	got := nullTimeToString(pgtype.Timestamptz{})
	if got != "" {
		t.Errorf("invalid timestamp should return empty, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// parseOptionalUUID
// ---------------------------------------------------------------------------

func TestParseOptionalUUID_Empty(t *testing.T) {
	got := parseOptionalUUID("")
	if got.Valid {
		t.Error("empty string should produce invalid UUID")
	}
}

func TestParseOptionalUUID_Valid(t *testing.T) {
	got := parseOptionalUUID("550e8400-e29b-41d4-a716-446655440000")
	if !got.Valid {
		t.Error("valid UUID string should produce Valid=true")
	}
}

func TestParseOptionalUUID_Invalid(t *testing.T) {
	got := parseOptionalUUID("not-a-uuid")
	if got.Valid {
		t.Error("invalid UUID string should produce Valid=false")
	}
}

// ---------------------------------------------------------------------------
// buildIssueSection
// ---------------------------------------------------------------------------

func TestBuildIssueSection_WithTitleStatusPriority(t *testing.T) {
	issue := db.Issue{
		Title:    "Fix login bug",
		Status:   "open",
		Priority: "high",
	}
	got := buildIssueSection(issue)
	for _, want := range []string{"Title: Fix login bug", "Status: open", "Priority: high"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got:\n%s", want, got)
		}
	}
}

func TestBuildIssueSection_WithDescription(t *testing.T) {
	issue := db.Issue{
		Title:       "T",
		Status:      "s",
		Priority:    "p",
		Description: pgtype.Text{String: "details here", Valid: true},
	}
	got := buildIssueSection(issue)
	if !strings.Contains(got, "details here") {
		t.Errorf("should include description, got:\n%s", got)
	}
}

func TestBuildIssueSection_EmptyValidDescription(t *testing.T) {
	issue := db.Issue{
		Title:       "T",
		Status:      "s",
		Priority:    "p",
		Description: pgtype.Text{String: "", Valid: true},
	}
	got := buildIssueSection(issue)
	// Empty string with Valid=true should not add description line
	if strings.Count(got, "\n") > 3 {
		t.Errorf("empty description should not add extra lines, got:\n%s", got)
	}
}

func TestBuildIssueSection_InvalidDescription(t *testing.T) {
	issue := db.Issue{
		Title:       "T",
		Status:      "s",
		Priority:    "p",
		Description: pgtype.Text{Valid: false},
	}
	got := buildIssueSection(issue)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (title/status/priority), got %d:\n%s", len(lines), got)
	}
}

// ---------------------------------------------------------------------------
// buildCommentsSection
// ---------------------------------------------------------------------------

func TestBuildCommentsSection_Empty(t *testing.T) {
	got := buildCommentsSection(nil)
	if got != "" {
		t.Errorf("nil comments should produce empty string, got %q", got)
	}
}

func TestBuildCommentsSection_Single(t *testing.T) {
	comments := []db.Comment{
		{
			AuthorType: "member",
			Content:    "looks good",
			CreatedAt:  pgtype.Timestamptz{Time: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC), Valid: true},
		},
	}
	got := buildCommentsSection(comments)
	for _, want := range []string{"[1]", "member", "looks good", "2026-01-15 10:00"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got:\n%s", want, got)
		}
	}
}

func TestBuildCommentsSection_NoTimestamp(t *testing.T) {
	comments := []db.Comment{
		{AuthorType: "agent", Content: "done", CreatedAt: pgtype.Timestamptz{}},
	}
	got := buildCommentsSection(comments)
	if !strings.Contains(got, "[1] agent (): done") {
		t.Errorf("unexpected format, got:\n%s", got)
	}
}

func TestBuildCommentsSection_Multiple(t *testing.T) {
	comments := []db.Comment{
		{AuthorType: "member", Content: "first", CreatedAt: pgtype.Timestamptz{}},
		{AuthorType: "agent", Content: "second", CreatedAt: pgtype.Timestamptz{}},
	}
	got := buildCommentsSection(comments)
	if !strings.Contains(got, "[1]") || !strings.Contains(got, "[2]") {
		t.Errorf("should number comments, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// buildAttachmentsSection
// ---------------------------------------------------------------------------

func TestBuildAttachmentsSection_Empty(t *testing.T) {
	got := buildAttachmentsSection(nil)
	if got != "" {
		t.Errorf("nil attachments should produce empty string")
	}
}

func TestBuildAttachmentsSection_Single(t *testing.T) {
	attachments := []db.Attachment{
		{Filename: "screenshot.png", ContentType: "image/png", SizeBytes: 1048576},
	}
	got := buildAttachmentsSection(attachments)
	for _, want := range []string{"screenshot.png", "image/png", "1.00 MB"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got:\n%s", want, got)
		}
	}
}

func TestBuildAttachmentsSection_SmallFile(t *testing.T) {
	attachments := []db.Attachment{
		{Filename: "tiny.txt", ContentType: "text/plain", SizeBytes: 512},
	}
	got := buildAttachmentsSection(attachments)
	if !strings.Contains(got, "0.00 MB") {
		t.Errorf("small file should show ~0.00 MB, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// buildSkillsSection
// ---------------------------------------------------------------------------

func TestBuildSkillsSection_Empty(t *testing.T) {
	got := buildSkillsSection(nil)
	if got != "" {
		t.Errorf("nil skills should produce empty string")
	}
}

func TestBuildSkillsSection_NameOnly(t *testing.T) {
	skills := []db.Skill{{Name: "code-review"}}
	got := buildSkillsSection(skills)
	if !strings.Contains(got, "- code-review\n") {
		t.Errorf("should contain skill name, got:\n%s", got)
	}
}

func TestBuildSkillsSection_WithDescription(t *testing.T) {
	skills := []db.Skill{{Name: "lint", Description: "Code linter"}}
	got := buildSkillsSection(skills)
	if !strings.Contains(got, "lint: Code linter") {
		t.Errorf("should contain name: description, got:\n%s", got)
	}
}

func TestBuildSkillsSection_Multiple(t *testing.T) {
	skills := []db.Skill{
		{Name: "a"},
		{Name: "b", Description: "desc"},
	}
	got := buildSkillsSection(skills)
	if !strings.Contains(got, "- a\n") {
		t.Errorf("missing first skill, got:\n%s", got)
	}
	if !strings.Contains(got, "- b: desc\n") {
		t.Errorf("missing second skill, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// runEventToMap (additional tests beyond handler_test.go)
// ---------------------------------------------------------------------------

func TestRunEventToMap_Basic(t *testing.T) {
	ev := db.RunEvent{
		ID: pgtype.UUID{
			Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Valid: true,
		},
		RunID: pgtype.UUID{
			Bytes: [16]byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80,
				0x90, 0xa0, 0xb0, 0xc0, 0xd0, 0xe0, 0xf0, 0x00},
			Valid: true,
		},
		Seq:       pgtype.Int8{Int64: 5, Valid: true},
		EventType: "step_completed",
		Payload:   []byte(`{"key":"value"}`),
		CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), Valid: true},
	}
	m := runEventToMap(ev)

	if m["event_type"] != "step_completed" {
		t.Errorf("event_type = %v", m["event_type"])
	}
	if m["seq"] != int64(5) {
		t.Errorf("seq = %v", m["seq"])
	}
	payload, ok := m["payload"].(map[string]any)
	if !ok {
		t.Fatal("payload should be map[string]any")
	}
	if payload["key"] != "value" {
		t.Errorf("payload key = %v", payload["key"])
	}
}

func TestRunEventToMap_NilPayload(t *testing.T) {
	ev := db.RunEvent{
		ID:        pgtype.UUID{Valid: true},
		RunID:     pgtype.UUID{Valid: true},
		Seq:       pgtype.Int8{Int64: 0, Valid: true},
		EventType: "created",
		Payload:   nil,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m := runEventToMap(ev)
	if m["event_type"] != "created" {
		t.Errorf("event_type = %v", m["event_type"])
	}
	payload, ok := m["payload"].(map[string]any)
	if !ok {
		t.Fatal("payload should be map[string]any")
	}
	if len(payload) != 0 {
		t.Errorf("nil payload should produce empty map, got %v", payload)
	}
}
