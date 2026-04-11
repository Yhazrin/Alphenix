package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ── issueToResponse ─────────────────────────────────────────────────────────

func TestIssueToResponse_Basic(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	issue := db.Issue{
		ID:           makeTestUUID("issue-1"),
		WorkspaceID:  makeTestUUID("ws-1"),
		Number:       42,
		Title:        "Test Issue",
		Description:  pgtype.Text{String: "A description", Valid: true},
		Status:       "open",
		Priority:     "high",
		AssigneeType: pgtype.Text{String: "agent", Valid: true},
		AssigneeID:   makeTestUUID("agent-1"),
		CreatorType:  "user",
		CreatorID:    makeTestUUID("user-1"),
		Position:     100.5,
		CreatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := issueToResponse(issue, "PROJ")

	if resp.Identifier != "PROJ-42" {
		t.Errorf("Identifier = %q, want %q", resp.Identifier, "PROJ-42")
	}
	if resp.Title != "Test Issue" {
		t.Errorf("Title = %q, want %q", resp.Title, "Test Issue")
	}
	if resp.Status != "open" {
		t.Errorf("Status = %q, want %q", resp.Status, "open")
	}
	if resp.Priority != "high" {
		t.Errorf("Priority = %q, want %q", resp.Priority, "high")
	}
	if resp.Position != 100.5 {
		t.Errorf("Position = %v, want 100.5", resp.Position)
	}
	if resp.Description == nil || *resp.Description != "A description" {
		t.Errorf("Description = %v, want %q", resp.Description, "A description")
	}
	if resp.AssigneeType == nil || *resp.AssigneeType != "agent" {
		t.Errorf("AssigneeType = %v, want %q", resp.AssigneeType, "agent")
	}
}

func TestIssueToResponse_NullFields(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	issue := db.Issue{
		ID:          makeTestUUID("issue-2"),
		WorkspaceID: makeTestUUID("ws-2"),
		Number:      1,
		Title:       "Minimal",
		Status:      "open",
		Priority:    "medium",
		CreatorType: "user",
		CreatorID:   makeTestUUID("user-2"),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := issueToResponse(issue, "WS")

	if resp.Description != nil {
		t.Errorf("Description = %v, want nil", resp.Description)
	}
	if resp.AssigneeType != nil {
		t.Errorf("AssigneeType = %v, want nil", resp.AssigneeType)
	}
	if resp.AssigneeID != nil {
		t.Errorf("AssigneeID = %v, want nil", resp.AssigneeID)
	}
	if resp.ParentIssueID != nil {
		t.Errorf("ParentIssueID = %v, want nil", resp.ParentIssueID)
	}
	if resp.DueDate != nil {
		t.Errorf("DueDate = %v, want nil", resp.DueDate)
	}
	if resp.Identifier != "WS-1" {
		t.Errorf("Identifier = %q, want %q", resp.Identifier, "WS-1")
	}
}

func makeTestUUID(prefix string) pgtype.UUID {
	id := [16]byte{}
	for i := 0; i < 16 && i < len(prefix); i++ {
		id[i] = prefix[i]
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// ── validateBatchIssueIDs ─────────────────────────────────────────────────────

func TestValidateBatchIssueIDs_Empty(t *testing.T) {
	msg := validateBatchIssueIDs(nil)
	if msg != "issue_ids is required" {
		t.Errorf("nil slice: got %q, want %q", msg, "issue_ids is required")
	}
	msg = validateBatchIssueIDs([]string{})
	if msg != "issue_ids is required" {
		t.Errorf("empty slice: got %q, want %q", msg, "issue_ids is required")
	}
}

func TestValidateBatchIssueIDs_OverMax(t *testing.T) {
	ids := make([]string, 501)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}
	msg := validateBatchIssueIDs(ids)
	if msg != "too many issue IDs (max 500)" {
		t.Errorf("got %q, want %q", msg, "too many issue IDs (max 500)")
	}
}

func TestValidateBatchIssueIDs_Valid(t *testing.T) {
	tests := []struct {
		name string
		ids  []string
	}{
		{"single", []string{"a"}},
		{"max boundary", make([]string, 500)},
		{"mid-range", make([]string, 100)},
	}
	for _, tt := range tests {
		msg := validateBatchIssueIDs(tt.ids)
		if msg != "" {
			t.Errorf("%s: got %q, want empty", tt.name, msg)
		}
	}
}

// ── parseSearchLimit ──────────────────────────────────────────────────────────

func TestParseSearchLimit_Empty(t *testing.T) {
	if got := parseSearchLimit(""); got != 20 {
		t.Errorf("empty: got %d, want 20", got)
	}
}

func TestParseSearchLimit_Valid(t *testing.T) {
	tests := map[string]int{
		"1":  1,
		"10": 10,
		"50": 50,
	}
	for input, want := range tests {
		if got := parseSearchLimit(input); got != want {
			t.Errorf("parseSearchLimit(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseSearchLimit_Invalid(t *testing.T) {
	tests := []string{"abc", "0", "-1", "51", "100", "999"}
	for _, input := range tests {
		if got := parseSearchLimit(input); got != 20 {
			t.Errorf("parseSearchLimit(%q) = %d, want 20 (default)", input, got)
		}
	}
}
