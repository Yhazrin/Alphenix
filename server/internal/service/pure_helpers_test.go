package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// priorityToInt
// ---------------------------------------------------------------------------

func TestPriorityToInt_Table(t *testing.T) {
	tests := []struct {
		input string
		want  int32
	}{
		{"urgent", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"", 0},
		{"unknown", 0},
		{"URGENT", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := priorityToInt(tt.input)
			if got != tt.want {
				t.Errorf("priorityToInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncateContent (edge cases beyond compactor_test.go)
// ---------------------------------------------------------------------------

func TestTruncateContent_NoTruncation(t *testing.T) {
	got := truncateContent("short", 10)
	if got != "short" {
		t.Errorf("got %q, want %q", got, "short")
	}
}

func TestTruncateContent_Exact(t *testing.T) {
	got := truncateContent("exact", 5)
	if got != "exact" {
		t.Errorf("got %q, want %q", got, "exact")
	}
}

// ---------------------------------------------------------------------------
// defaultSummary (edge cases beyond compactor_test.go)
// ---------------------------------------------------------------------------

func TestDefaultSummary_SingleMessage(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hello"}}
	got := defaultSummary(msgs)
	if got == "" {
		t.Error("summary should not be empty")
	}
	if len(got) < len("## Earlier conversation summary") {
		t.Error("summary should contain header")
	}
}

func TestDefaultSummary_TruncatesLongPreview(t *testing.T) {
	longContent := make([]byte, 200)
	for i := range longContent {
		longContent[i] = 'a'
	}
	msgs := []Message{{Role: "user", Content: string(longContent)}}
	got := defaultSummary(msgs)
	// The preview should contain at most 100 chars of content + "..."
	// Total output should be shorter than if all 200 chars were included
	if len(got) > 300 {
		t.Errorf("summary too long (%d chars), long content should be truncated", len(got))
	}
	// Verify the summary contains the header
	if len(got) < len("## Earlier conversation summary") {
		t.Error("summary should contain header")
	}
}

func TestDefaultSummary_Empty(t *testing.T) {
	got := defaultSummary(nil)
	if got == "" {
		t.Error("empty messages should still produce header")
	}
}

// ---------------------------------------------------------------------------
// buildTagSet
// ---------------------------------------------------------------------------

func TestBuildTagSet_Empty(t *testing.T) {
	s := buildTagSet(nil)
	if len(s) != 0 {
		t.Error("nil tags should produce empty set")
	}
}

func TestBuildTagSet_Basic(t *testing.T) {
	s := buildTagSet([]string{"gpu", "fast"})
	if !s["gpu"] || !s["fast"] {
		t.Error("tags should be present in set")
	}
	if len(s) != 2 {
		t.Errorf("set length = %d, want 2", len(s))
	}
}

func TestBuildTagSet_Duplicates(t *testing.T) {
	s := buildTagSet([]string{"gpu", "gpu"})
	if len(s) != 1 {
		t.Errorf("duplicate tags should produce 1 entry, got %d", len(s))
	}
}

// ---------------------------------------------------------------------------
// runtimeTagsSatisfyPolicy
// ---------------------------------------------------------------------------

func TestRuntimeTagsSatisfyPolicy_NoRequirements(t *testing.T) {
	if !runtimeTagsSatisfyPolicy(nil, nil, nil) {
		t.Error("no requirements should satisfy")
	}
}

func TestRuntimeTagsSatisfyPolicy_RequiredPresent(t *testing.T) {
	set := map[string]bool{"gpu": true, "fast": true}
	if !runtimeTagsSatisfyPolicy(set, []string{"gpu"}, nil) {
		t.Error("required tag present should satisfy")
	}
}

func TestRuntimeTagsSatisfyPolicy_RequiredMissing(t *testing.T) {
	set := map[string]bool{"fast": true}
	if runtimeTagsSatisfyPolicy(set, []string{"gpu"}, nil) {
		t.Error("missing required tag should not satisfy")
	}
}

func TestRuntimeTagsSatisfyPolicy_ForbiddenPresent(t *testing.T) {
	set := map[string]bool{"gpu": true}
	if runtimeTagsSatisfyPolicy(set, nil, []string{"gpu"}) {
		t.Error("forbidden tag present should not satisfy")
	}
}

func TestRuntimeTagsSatisfyPolicy_ForbiddenAbsent(t *testing.T) {
	set := map[string]bool{"fast": true}
	if !runtimeTagsSatisfyPolicy(set, nil, []string{"gpu"}) {
		t.Error("forbidden tag absent should satisfy")
	}
}

// ---------------------------------------------------------------------------
// agentMemoryRowsToSearchResults
// ---------------------------------------------------------------------------

func TestAgentMemoryRowsToSearchResults_Empty(t *testing.T) {
	got := agentMemoryRowsToSearchResults(nil)
	if got == nil || len(got) != 0 {
		t.Error("nil input should return empty slice")
	}
}

func TestAgentMemoryRowsToSearchResults_Basic(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	rows := []db.SearchAgentMemoryRow{
		{
			ID:         id,
			Content:    "test memory",
			Similarity: 42,
		},
	}
	got := agentMemoryRowsToSearchResults(rows)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Memory.Content != "test memory" {
		t.Errorf("Content = %q", got[0].Memory.Content)
	}
	if got[0].Score != 42 {
		t.Errorf("Score = %f, want 42", got[0].Score)
	}
	if got[0].Memory.ID != id {
		t.Error("ID should be preserved")
	}
}

// ---------------------------------------------------------------------------
// workspaceMemoryRowsToSearchResults
// ---------------------------------------------------------------------------

func TestWorkspaceMemoryRowsToSearchResults_Empty(t *testing.T) {
	got := workspaceMemoryRowsToSearchResults(nil)
	if got == nil || len(got) != 0 {
		t.Error("nil input should return empty slice")
	}
}

func TestWorkspaceMemoryRowsToSearchResults_Basic(t *testing.T) {
	rows := []db.SearchWorkspaceMemoryRow{
		{Content: "ws memory", Similarity: 99},
	}
	got := workspaceMemoryRowsToSearchResults(rows)
	if len(got) != 1 || got[0].Memory.Content != "ws memory" {
		t.Errorf("got %v", got)
	}
}

// ---------------------------------------------------------------------------
// recentMemoryRowsToSearchResults
// ---------------------------------------------------------------------------

func TestRecentMemoryRowsToSearchResults_Empty(t *testing.T) {
	got := recentMemoryRowsToSearchResults(nil)
	if got == nil || len(got) != 0 {
		t.Error("nil input should return empty slice")
	}
}

func TestRecentMemoryRowsToSearchResults_Basic(t *testing.T) {
	rows := []db.ListRecentWorkspaceMemoryRow{
		{Content: "recent", Similarity: 1.5},
	}
	got := recentMemoryRowsToSearchResults(rows)
	if len(got) != 1 || got[0].Score != 1.5 {
		t.Errorf("Score = %f, want 1.5", got[0].Score)
	}
}

// ---------------------------------------------------------------------------
// bm25RowToAgentMemory
// ---------------------------------------------------------------------------

func TestBm25RowToAgentMemory_Basic(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{5}, Valid: true}
	row := db.SearchWorkspaceMemoryBM25Row{
		ID:      id,
		Content: "bm25 result",
	}
	got := bm25RowToAgentMemory(row)
	if got.ID != id {
		t.Error("ID should be preserved")
	}
	if got.Content != "bm25 result" {
		t.Errorf("Content = %q", got.Content)
	}
}

// ---------------------------------------------------------------------------
// nullUUIDToString
// ---------------------------------------------------------------------------

func TestNullUUIDToString_Valid(t *testing.T) {
	u := pgtype.UUID{Bytes: [16]byte{1, 2, 3}, Valid: true}
	got := nullUUIDToString(u)
	if got == "" {
		t.Error("valid UUID should produce non-empty string")
	}
}

func TestNullUUIDToString_Invalid(t *testing.T) {
	u := pgtype.UUID{Valid: false}
	got := nullUUIDToString(u)
	if got != "" {
		t.Errorf("invalid UUID should produce empty string, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// parseNullUUID
// ---------------------------------------------------------------------------

func TestParseNullUUID_Valid(t *testing.T) {
	got := parseNullUUID("11111111-1111-1111-1111-111111111111")
	if !got.Valid {
		t.Error("valid UUID string should produce valid UUID")
	}
}

func TestParseNullUUID_Empty(t *testing.T) {
	got := parseNullUUID("")
	if got.Valid {
		t.Error("empty string should produce invalid UUID")
	}
}

func TestParseNullUUID_Invalid(t *testing.T) {
	got := parseNullUUID("not-a-uuid")
	if got.Valid {
		t.Error("invalid UUID string should produce invalid UUID")
	}
}

// ---------------------------------------------------------------------------
// parseJSONStringSlice
// ---------------------------------------------------------------------------

func TestParseJSONStringSlice_Nil(t *testing.T) {
	got := parseJSONStringSlice(nil)
	if got != nil {
		t.Errorf("nil input should return nil, got %v", got)
	}
}

func TestParseJSONStringSlice_Empty(t *testing.T) {
	got := parseJSONStringSlice([]byte("[]"))
	if len(got) != 0 {
		t.Errorf("empty array should return empty slice, got %v", got)
	}
}

func TestParseJSONStringSlice_Basic(t *testing.T) {
	got := parseJSONStringSlice([]byte(`["gpu","linux"]`))
	if len(got) != 2 || got[0] != "gpu" || got[1] != "linux" {
		t.Errorf("expected [gpu linux], got %v", got)
	}
}

func TestParseJSONStringSlice_InvalidJSON(t *testing.T) {
	got := parseJSONStringSlice([]byte(`not-json`))
	if got != nil {
		t.Errorf("invalid JSON should return nil, got %v", got)
	}
}

func TestParseJSONStringSlice_NullJSON(t *testing.T) {
	got := parseJSONStringSlice([]byte(`null`))
	if got != nil {
		t.Errorf("null JSON should return nil, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// isRuntimeEligible
// ---------------------------------------------------------------------------

func TestIsRuntimeEligible_Online(t *testing.T) {
	rt := db.AgentRuntime{Status: "online"}
	if !isRuntimeEligible(rt) {
		t.Error("online runtime should be eligible")
	}
}

func TestIsRuntimeEligible_Offline(t *testing.T) {
	rt := db.AgentRuntime{Status: "offline"}
	if isRuntimeEligible(rt) {
		t.Error("offline runtime should not be eligible")
	}
}

func TestIsRuntimeEligible_Paused(t *testing.T) {
	rt := db.AgentRuntime{Status: "online", Paused: true}
	if isRuntimeEligible(rt) {
		t.Error("paused runtime should not be eligible")
	}
}

func TestIsRuntimeEligible_Draining(t *testing.T) {
	rt := db.AgentRuntime{Status: "online", DrainMode: true}
	if isRuntimeEligible(rt) {
		t.Error("draining runtime should not be eligible")
	}
}

func TestIsRuntimeEligible_NotApproved(t *testing.T) {
	rt := db.AgentRuntime{Status: "online", ApprovalStatus: "pending"}
	if isRuntimeEligible(rt) {
		t.Error("pending approval runtime should not be eligible")
	}
}

func TestIsRuntimeEligible_Approved(t *testing.T) {
	rt := db.AgentRuntime{Status: "online", ApprovalStatus: "approved"}
	if !isRuntimeEligible(rt) {
		t.Error("approved runtime should be eligible")
	}
}

func TestIsRuntimeEligible_EmptyApprovalStatus(t *testing.T) {
	rt := db.AgentRuntime{Status: "online", ApprovalStatus: ""}
	if !isRuntimeEligible(rt) {
		t.Error("empty approval status should be treated as eligible")
	}
}

// ---------------------------------------------------------------------------
// computeLoadScore
// ---------------------------------------------------------------------------

func TestComputeLoadScore_ZeroHistory(t *testing.T) {
	rt := db.AgentRuntime{}
	score := computeLoadScore(rt)
	if score != 0 {
		t.Errorf("fresh runtime should have score 0, got %d", score)
	}
}

func TestComputeLoadScore_WithDuration(t *testing.T) {
	rt := db.AgentRuntime{AvgTaskDurationMs: 5000}
	score := computeLoadScore(rt)
	if score != 5 {
		t.Errorf("5000ms / 1000 = 5, got %d", score)
	}
}

func TestComputeLoadScore_WithFailures(t *testing.T) {
	rt := db.AgentRuntime{
		SuccessCount24h: 70,
		FailureCount24h: 30,
	}
	score := computeLoadScore(rt)
	// failRate = 30 * 100 / 100 = 30, penalty = 30 * 10 = 300
	if score != 300 {
		t.Errorf("expected 300 failure penalty, got %d", score)
	}
}

func TestComputeLoadScore_DurationAndFailures(t *testing.T) {
	rt := db.AgentRuntime{
		AvgTaskDurationMs: 10000,
		SuccessCount24h:   90,
		FailureCount24h:   10,
	}
	score := computeLoadScore(rt)
	// base = 10000/1000 = 10, failRate = 10*100/100 = 10, penalty = 10*10 = 100
	if score != 110 {
		t.Errorf("expected 110, got %d", score)
	}
}

// ---------------------------------------------------------------------------
// tagsMatch
// ---------------------------------------------------------------------------

func TestTagsMatch_NoConstraints(t *testing.T) {
	rt := db.AgentRuntime{Tags: []byte(`["gpu"]`)}
	if !tagsMatch(rt, nil, nil) {
		t.Error("no constraints should match any runtime")
	}
}

func TestTagsMatch_RequiredPresent(t *testing.T) {
	rt := db.AgentRuntime{Tags: []byte(`["gpu","linux"]`)}
	if !tagsMatch(rt, []string{"gpu"}, nil) {
		t.Error("required tag present should match")
	}
}

func TestTagsMatch_RequiredMissing(t *testing.T) {
	rt := db.AgentRuntime{Tags: []byte(`["linux"]`)}
	if tagsMatch(rt, []string{"gpu"}, nil) {
		t.Error("missing required tag should not match")
	}
}

func TestTagsMatch_ForbiddenPresent(t *testing.T) {
	rt := db.AgentRuntime{Tags: []byte(`["gpu","windows"]`)}
	if tagsMatch(rt, nil, []string{"windows"}) {
		t.Error("forbidden tag present should not match")
	}
}

func TestTagsMatch_NilTags(t *testing.T) {
	rt := db.AgentRuntime{Tags: nil}
	if !tagsMatch(rt, nil, nil) {
		t.Error("nil tags with no constraints should match")
	}
}

func TestTagsMatch_NilTagsWithRequired(t *testing.T) {
	rt := db.AgentRuntime{Tags: nil}
	if tagsMatch(rt, []string{"gpu"}, nil) {
		t.Error("nil tags should not satisfy required constraint")
	}
}

func TestTagsMatch_MalformedJSON(t *testing.T) {
	rt := db.AgentRuntime{Tags: []byte(`not-json`)}
	if tagsMatch(rt, []string{"gpu"}, nil) {
		t.Error("malformed JSON tags should not satisfy required constraint")
	}
}

func TestTagsMatch_MalformedJSONNoConstraints(t *testing.T) {
	rt := db.AgentRuntime{Tags: []byte(`not-json`)}
	if !tagsMatch(rt, nil, nil) {
		t.Error("malformed JSON with no constraints should match (no constraints = always true)")
	}
}
