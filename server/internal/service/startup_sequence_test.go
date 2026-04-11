package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

func TestBuildTagSet(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want map[string]bool
	}{
		{"nil", nil, map[string]bool{}},
		{"empty", []string{}, map[string]bool{}},
		{"single", []string{"gpu"}, map[string]bool{"gpu": true}},
		{"multiple", []string{"gpu", "linux", "x86"}, map[string]bool{"gpu": true, "linux": true, "x86": true}},
		{"duplicate", []string{"gpu", "gpu"}, map[string]bool{"gpu": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTagSet(tt.tags)
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.want))
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("missing key %q", k)
				}
			}
		})
	}
}

func TestRuntimeTagsSatisfyPolicy(t *testing.T) {
	tests := []struct {
		name     string
		runtime  []string
		required []string
		forbid   []string
		want     bool
	}{
		{"no constraints matches anything", []string{"gpu"}, nil, nil, true},
		{"required tag present", []string{"gpu", "linux"}, []string{"gpu"}, nil, true},
		{"required tag missing", []string{"linux"}, []string{"gpu"}, nil, false},
		{"forbidden tag absent", []string{"gpu"}, nil, []string{"windows"}, true},
		{"forbidden tag present", []string{"gpu", "windows"}, nil, []string{"windows"}, false},
		{"required present + forbidden absent", []string{"gpu", "linux"}, []string{"gpu"}, []string{"windows"}, true},
		{"required present + forbidden present", []string{"gpu", "windows"}, []string{"gpu"}, []string{"windows"}, false},
		{"empty runtime with required fails", []string{}, []string{"gpu"}, nil, false},
		{"empty runtime with no constraints passes", []string{}, nil, nil, true},
		{"multiple required all present", []string{"gpu", "linux", "x86"}, []string{"gpu", "linux"}, nil, true},
		{"multiple required one missing", []string{"gpu"}, []string{"gpu", "linux"}, nil, false},
		{"both nil constraints matches", nil, nil, nil, true},
		{"required and forbidden same tag", []string{"gpu"}, []string{"gpu"}, []string{"gpu"}, false},
		{"case sensitive tags", []string{"GPU"}, []string{"gpu"}, nil, false},
		{"duplicate required tags", []string{"gpu"}, []string{"gpu", "gpu"}, nil, true},
		{"empty slices match anything", []string{"anything"}, []string{}, []string{}, true},
		{"forbidden only blocks exact match", []string{"gpu-v2"}, nil, []string{"gpu"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tagSet := buildTagSet(tt.runtime)
			got := runtimeTagsSatisfyPolicy(tagSet, tt.required, tt.forbid)
			if got != tt.want {
				t.Errorf("runtimeTagsSatisfyPolicy(%v, required=%v, forbid=%v) = %v, want %v",
					tt.runtime, tt.required, tt.forbid, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// RunStartupSequence nil Queries
// ---------------------------------------------------------------------------

func TestRunStartupSequence_NilQueries(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil Queries, got none")
		}
	}()
	svc := &TaskService{Queries: nil}
	_, _ = svc.RunStartupSequence(
		context.Background(),
		pgtype.UUID{},
		pgtype.UUID{},
		[]string{"gpu"},
	)
}

// ---------------------------------------------------------------------------
// StartupSequenceResult struct tests
// ---------------------------------------------------------------------------

func TestStartupSequenceResult_EmptyMatchedAgents(t *testing.T) {
	result := StartupSequenceResult{}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents, got %d", len(result.MatchedAgents))
	}
}

func TestMatchedAgent_PreservesPolicy(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	policy := db.RuntimeAssignmentPolicy{
		AgentID:  id,
		IsActive: true,
	}
	ma := MatchedAgent{
		AgentID: id,
		Policy:  policy,
	}
	if ma.AgentID != id {
		t.Error("AgentID not preserved")
	}
	if !ma.Policy.IsActive {
		t.Error("Policy not preserved")
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence integration tests (with taskStubDBTX)
// ---------------------------------------------------------------------------

func makePolicyJSON(tags []string) []byte {
	if tags == nil {
		return nil
	}
	b, _ := json.Marshal(tags)
	return b
}

func TestRunStartupSequence_NoPolicies(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := &TaskService{Queries: db.New(stub)}
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")

	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents, got %d", len(result.MatchedAgents))
	}
}

func TestRunStartupSequence_MatchingPolicy(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")
	policyID := util.ParseUUID("00000000-0000-0000-0000-0000000000p1")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:           policyID,
		WorkspaceID:  wsID,
		AgentID:      agentID,
		RequiredTags: makePolicyJSON([]string{"gpu"}),
		IsActive:     true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu", "linux"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 1 {
		t.Fatalf("expected 1 matched agent, got %d", len(result.MatchedAgents))
	}
	if result.MatchedAgents[0].AgentID != agentID {
		t.Errorf("expected agent ID %v, got %v", agentID, result.MatchedAgents[0].AgentID)
	}
}

func TestRunStartupSequence_MissingRequiredTag(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:           util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID:  wsID,
		AgentID:      agentID,
		RequiredTags: makePolicyJSON([]string{"gpu", "tpu"}),
		IsActive:     true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	// Runtime has gpu but not tpu
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents (missing tpu), got %d", len(result.MatchedAgents))
	}
}

func TestRunStartupSequence_ForbiddenTag(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:            util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID:   wsID,
		AgentID:       agentID,
		ForbiddenTags: makePolicyJSON([]string{"windows"}),
		IsActive:      true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu", "windows"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents (forbidden tag windows), got %d", len(result.MatchedAgents))
	}
}

func TestRunStartupSequence_InactivePolicy(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:          util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID: wsID,
		AgentID:     agentID,
		IsActive:    false, // inactive
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Inactive policies are filtered by the SQL query (is_active = true), so none returned.
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents (inactive policy), got %d", len(result.MatchedAgents))
	}
}

func TestRunStartupSequence_MultiplePolicies(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentA := util.ParseUUID("00000000-0000-0000-0000-00000000000a")
	agentB := util.ParseUUID("00000000-0000-0000-0000-00000000000b")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:          util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID: wsID,
		AgentID:     agentA,
		IsActive:    true,
		// No required tags — matches anything
	}
	stub.runtimePolicies["agent-b"] = db.RuntimeAssignmentPolicy{
		ID:           util.ParseUUID("00000000-0000-0000-0000-0000000000p2"),
		WorkspaceID:  wsID,
		AgentID:      agentB,
		RequiredTags: makePolicyJSON([]string{"gpu"}),
		IsActive:     true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 2 {
		t.Errorf("expected 2 matched agents, got %d", len(result.MatchedAgents))
	}
}

func TestRunStartupSequence_DifferentWorkspace(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID1 := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	wsID2 := util.ParseUUID("00000000-0000-0000-0000-000000000002")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:          util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID: wsID1,
		AgentID:     util.ParseUUID("00000000-0000-0000-0000-00000000000a"),
		IsActive:    true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	// Query for a different workspace
	result, err := svc.RunStartupSequence(context.Background(), wsID2, makeTestUUID("rt-1"), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents for different workspace, got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — DB error propagation
// ---------------------------------------------------------------------------

func TestRunStartupSequence_DBError(t *testing.T) {
	stub := newTaskStubDBTX()
	stub.queryErr = context.DeadlineExceeded
	svc := &TaskService{Queries: db.New(stub)}

	_, err := svc.RunStartupSequence(context.Background(),
		util.ParseUUID("00000000-0000-0000-0000-000000000001"),
		makeTestUUID("rt-1"), []string{"gpu"})
	if err == nil {
		t.Fatal("expected error from DB failure")
	}
	if !strings.Contains(err.Error(), "list active policies") {
		t.Errorf("expected 'list active policies' in error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — combined required + forbidden tags
// ---------------------------------------------------------------------------

func TestRunStartupSequence_CombinedRequiredAndForbidden(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:            util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID:   wsID,
		AgentID:       agentID,
		RequiredTags:  makePolicyJSON([]string{"gpu"}),
		ForbiddenTags: makePolicyJSON([]string{"windows"}),
		IsActive:      true,
	}

	svc := &TaskService{Queries: db.New(stub)}

	// Runtime has gpu + linux (no windows) — should match
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu", "linux"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 1 {
		t.Errorf("expected 1 match (gpu present, windows absent), got %d", len(result.MatchedAgents))
	}

	// Runtime has gpu + windows — should NOT match (forbidden tag)
	result, err = svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu", "windows"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matches (forbidden windows present), got %d", len(result.MatchedAgents))
	}

	// Runtime has linux only (no gpu) — should NOT match (missing required)
	result, err = svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"linux"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matches (missing required gpu), got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — empty runtime tags with required tags
// ---------------------------------------------------------------------------

func TestRunStartupSequence_EmptyRuntimeTags_WithRequired(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:           util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID:  wsID,
		AgentID:      agentID,
		RequiredTags: makePolicyJSON([]string{"gpu"}),
		IsActive:     true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matches (empty runtime tags, required gpu), got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — empty runtime tags, no constraints
// ---------------------------------------------------------------------------

func TestRunStartupSequence_EmptyRuntimeTags_NoConstraints(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:          util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID: wsID,
		AgentID:     agentID,
		IsActive:    true,
		// No required or forbidden tags
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 1 {
		t.Errorf("expected 1 match (no constraints), got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — nil runtime tags
// ---------------------------------------------------------------------------

func TestRunStartupSequence_NilRuntimeTags(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:          util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID: wsID,
		AgentID:     agentID,
		IsActive:    true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 1 {
		t.Errorf("expected 1 match (nil tags, no constraints), got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — malformed JSON in policy tags
// ---------------------------------------------------------------------------

func TestRunStartupSequence_MalformedRequiredTags(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:           util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID:  wsID,
		AgentID:      agentID,
		RequiredTags: []byte(`not-json`),
		IsActive:     true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Malformed required tags parse to nil → no constraints → matches
	if len(result.MatchedAgents) != 1 {
		t.Errorf("expected 1 match (malformed required → nil → no constraints), got %d", len(result.MatchedAgents))
	}
}

func TestRunStartupSequence_MalformedForbiddenTags(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:            util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID:   wsID,
		AgentID:       agentID,
		ForbiddenTags: []byte(`not-json`),
		IsActive:      true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Malformed forbidden tags parse to nil → no forbidden → matches
	if len(result.MatchedAgents) != 1 {
		t.Errorf("expected 1 match (malformed forbidden → nil → no forbidden), got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — duplicate required tags
// ---------------------------------------------------------------------------

func TestRunStartupSequence_DuplicateRequiredTags(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")
	agentID := util.ParseUUID("00000000-0000-0000-0000-00000000000a")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:           util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID:  wsID,
		AgentID:      agentID,
		RequiredTags: makePolicyJSON([]string{"gpu", "gpu"}),
		IsActive:     true,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 1 {
		t.Errorf("expected 1 match (duplicate required tags), got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — empty workspace (no policies at all)
// ---------------------------------------------------------------------------

func TestRunStartupSequence_EmptyWorkspace(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000999") // no policies for this workspace

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu", "linux"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents (empty workspace), got %d", len(result.MatchedAgents))
	}
}

// ---------------------------------------------------------------------------
// RunStartupSequence — all policies inactive
// ---------------------------------------------------------------------------

func TestRunStartupSequence_AllInactive(t *testing.T) {
	stub := newTaskStubDBTX()
	wsID := util.ParseUUID("00000000-0000-0000-0000-000000000001")

	stub.runtimePolicies["agent-a"] = db.RuntimeAssignmentPolicy{
		ID:          util.ParseUUID("00000000-0000-0000-0000-0000000000p1"),
		WorkspaceID: wsID,
		AgentID:     util.ParseUUID("00000000-0000-0000-0000-00000000000a"),
		IsActive:    false,
	}
	stub.runtimePolicies["agent-b"] = db.RuntimeAssignmentPolicy{
		ID:          util.ParseUUID("00000000-0000-0000-0000-0000000000p2"),
		WorkspaceID: wsID,
		AgentID:     util.ParseUUID("00000000-0000-0000-0000-00000000000b"),
		IsActive:    false,
	}

	svc := &TaskService{Queries: db.New(stub)}
	result, err := svc.RunStartupSequence(context.Background(), wsID, makeTestUUID("rt-1"), []string{"gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MatchedAgents) != 0 {
		t.Errorf("expected 0 matched agents (all inactive), got %d", len(result.MatchedAgents))
	}
}
