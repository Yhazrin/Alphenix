package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// --- parseJSONStringSlice ---

func TestParseJSONStringSlice_NilInput(t *testing.T) {
	result := parseJSONStringSlice(nil)
	if len(result) != 0 {
		t.Errorf("nil input should return empty slice, got len=%d", len(result))
	}
}

func TestParseJSONStringSlice_EmptyArray(t *testing.T) {
	result := parseJSONStringSlice([]byte(`[]`))
	if len(result) != 0 {
		t.Errorf("empty array should return empty slice, got len=%d", len(result))
	}
}

func TestParseJSONStringSlice_ValidArray(t *testing.T) {
	result := parseJSONStringSlice([]byte(`["alpha","beta","gamma"]`))
	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}
	if result[0] != "alpha" {
		t.Errorf("result[0] = %q", result[0])
	}
	if result[2] != "gamma" {
		t.Errorf("result[2] = %q", result[2])
	}
}

func TestParseJSONStringSlice_InvalidJSON(t *testing.T) {
	result := parseJSONStringSlice([]byte(`not-json`))
	if len(result) != 0 {
		t.Errorf("invalid JSON should return empty slice, got len=%d", len(result))
	}
}

func TestParseJSONStringSlice_NullJSON(t *testing.T) {
	result := parseJSONStringSlice([]byte(`null`))
	if len(result) != 0 {
		t.Errorf("null JSON should return empty slice, got len=%d", len(result))
	}
}

func TestParseJSONStringSlice_SingleElement(t *testing.T) {
	result := parseJSONStringSlice([]byte(`["solo"]`))
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0] != "solo" {
		t.Errorf("result[0] = %q", result[0])
	}
}

func TestParseJSONStringSlice_EmptyStrings(t *testing.T) {
	result := parseJSONStringSlice([]byte(`["",""]`))
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	for i, s := range result {
		if s != "" {
			t.Errorf("result[%d] = %q, want empty", i, s)
		}
	}
}

func TestParseJSONStringSlice_WhitespaceElements(t *testing.T) {
	result := parseJSONStringSlice([]byte(`[" hello ","  world  "]`))
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0] != " hello " {
		t.Errorf("result[0] = %q (should preserve whitespace)", result[0])
	}
}

// --- runtimePolicyToResponse ---

func TestRuntimePolicyToResponse_WithTeamID(t *testing.T) {
	teamUUID := testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	p := db.RuntimeAssignmentPolicy{
		ID:                  testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:         testUUID("22222222-2222-2222-2222-222222222222"),
		AgentID:             testUUID("33333333-3333-3333-3333-333333333333"),
		TeamID:              teamUUID,
		RequiredTags:        []byte(`["gpu","high-mem"]`),
		ForbiddenTags:       []byte(`["noisy"]`),
		PreferredRuntimeIds: []byte(`["rt-001","rt-002"]`),
		FallbackRuntimeIds:  []byte(`["rt-003"]`),
		MaxQueueDepth:       10,
		IsActive:            true,
		CreatedAt:           testTimestampFromInt(1700000000),
		UpdatedAt:           testTimestampFromInt(1700000000),
	}
	resp := runtimePolicyToResponse(p)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.WorkspaceID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("WorkspaceID = %q", resp.WorkspaceID)
	}
	if resp.AgentID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("AgentID = %q", resp.AgentID)
	}
	if resp.TeamID == nil {
		t.Fatal("TeamID should not be nil")
	}
	if *resp.TeamID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("TeamID = %q", *resp.TeamID)
	}
	if len(resp.RequiredTags) != 2 {
		t.Errorf("RequiredTags len = %d", len(resp.RequiredTags))
	}
	if resp.RequiredTags[0] != "gpu" {
		t.Errorf("RequiredTags[0] = %q", resp.RequiredTags[0])
	}
	if len(resp.ForbiddenTags) != 1 {
		t.Errorf("ForbiddenTags len = %d", len(resp.ForbiddenTags))
	}
	if len(resp.PreferredRuntimeIds) != 2 {
		t.Errorf("PreferredRuntimeIds len = %d", len(resp.PreferredRuntimeIds))
	}
	if len(resp.FallbackRuntimeIds) != 1 {
		t.Errorf("FallbackRuntimeIds len = %d", len(resp.FallbackRuntimeIds))
	}
	if resp.MaxQueueDepth != 10 {
		t.Errorf("MaxQueueDepth = %d", resp.MaxQueueDepth)
	}
	if !resp.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestRuntimePolicyToResponse_NilTeamID(t *testing.T) {
	p := db.RuntimeAssignmentPolicy{
		ID:                  testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:         testUUID("22222222-2222-2222-2222-222222222222"),
		AgentID:             testUUID("33333333-3333-3333-3333-333333333333"),
		TeamID:              pgtype.UUID{Valid: false},
		RequiredTags:        []byte(`[]`),
		ForbiddenTags:       []byte(`[]`),
		PreferredRuntimeIds: []byte(`[]`),
		FallbackRuntimeIds:  []byte(`[]`),
		MaxQueueDepth:       0,
		IsActive:            false,
		CreatedAt:           testTimestampFromInt(1700000000),
		UpdatedAt:           testTimestampFromInt(1700000000),
	}
	resp := runtimePolicyToResponse(p)

	if resp.TeamID != nil {
		t.Errorf("TeamID should be nil when Valid=false, got %q", *resp.TeamID)
	}
	if resp.IsActive {
		t.Error("IsActive should be false")
	}
	if len(resp.RequiredTags) != 0 {
		t.Errorf("RequiredTags should be empty, got len=%d", len(resp.RequiredTags))
	}
}

func TestRuntimePolicyToResponse_NilJSONFields(t *testing.T) {
	p := db.RuntimeAssignmentPolicy{
		ID:                  testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:         testUUID("22222222-2222-2222-2222-222222222222"),
		AgentID:             testUUID("33333333-3333-3333-3333-333333333333"),
		TeamID:              pgtype.UUID{Valid: false},
		RequiredTags:        nil,
		ForbiddenTags:       nil,
		PreferredRuntimeIds: nil,
		FallbackRuntimeIds:  nil,
		MaxQueueDepth:       0,
		IsActive:            true,
		CreatedAt:           testTimestampFromInt(1700000000),
		UpdatedAt:           testTimestampFromInt(1700000000),
	}
	resp := runtimePolicyToResponse(p)

	// nil []byte → parseJSONStringSlice returns empty slice
	if len(resp.RequiredTags) != 0 {
		t.Errorf("RequiredTags should be empty, got len=%d", len(resp.RequiredTags))
	}
	if len(resp.ForbiddenTags) != 0 {
		t.Errorf("ForbiddenTags should be empty, got len=%d", len(resp.ForbiddenTags))
	}
	if len(resp.PreferredRuntimeIds) != 0 {
		t.Errorf("PreferredRuntimeIds should be empty, got len=%d", len(resp.PreferredRuntimeIds))
	}
	if len(resp.FallbackRuntimeIds) != 0 {
		t.Errorf("FallbackRuntimeIds should be empty, got len=%d", len(resp.FallbackRuntimeIds))
	}
}

func TestRuntimePolicyToResponse_MaxQueueDepth(t *testing.T) {
	p := db.RuntimeAssignmentPolicy{
		ID:                  testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:         testUUID("22222222-2222-2222-2222-222222222222"),
		AgentID:             testUUID("33333333-3333-3333-3333-333333333333"),
		TeamID:              pgtype.UUID{Valid: false},
		RequiredTags:        []byte(`[]`),
		ForbiddenTags:       []byte(`[]`),
		PreferredRuntimeIds: []byte(`[]`),
		FallbackRuntimeIds:  []byte(`[]`),
		MaxQueueDepth:       999,
		IsActive:            true,
		CreatedAt:           testTimestampFromInt(1700000000),
		UpdatedAt:           testTimestampFromInt(1700000000),
	}
	resp := runtimePolicyToResponse(p)

	if resp.MaxQueueDepth != 999 {
		t.Errorf("MaxQueueDepth = %d, want 999", resp.MaxQueueDepth)
	}
}

// helper: create a pgtype.Timestamptz from unix seconds (for deterministic tests)
func testTimestampFromInt(unix int64) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Unix(unix, 0), Valid: true}
}
