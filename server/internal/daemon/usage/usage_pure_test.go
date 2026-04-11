package usage

import (
	"testing"
)

// ---------------------------------------------------------------------------
// mergeRecords
// ---------------------------------------------------------------------------

func TestMergeRecords_Empty(t *testing.T) {
	got := mergeRecords(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 records, got %d", len(got))
	}
}

func TestMergeRecords_Single(t *testing.T) {
	in := []Record{{Date: "2026-03-15", Provider: "claude", Model: "claude-sonnet-4-5", InputTokens: 100, OutputTokens: 50}}
	got := mergeRecords(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].InputTokens != 100 || got[0].OutputTokens != 50 {
		t.Errorf("single record not preserved: %+v", got[0])
	}
}

func TestMergeRecords_SameKeySums(t *testing.T) {
	in := []Record{
		{Date: "2026-03-15", Provider: "claude", Model: "claude-sonnet-4-5", InputTokens: 100, OutputTokens: 50, CacheReadTokens: 10, CacheWriteTokens: 5},
		{Date: "2026-03-15", Provider: "claude", Model: "claude-sonnet-4-5", InputTokens: 200, OutputTokens: 75, CacheReadTokens: 20, CacheWriteTokens: 10},
	}
	got := mergeRecords(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 merged record, got %d", len(got))
	}
	r := got[0]
	if r.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", r.InputTokens)
	}
	if r.OutputTokens != 125 {
		t.Errorf("OutputTokens = %d, want 125", r.OutputTokens)
	}
	if r.CacheReadTokens != 30 {
		t.Errorf("CacheReadTokens = %d, want 30", r.CacheReadTokens)
	}
	if r.CacheWriteTokens != 15 {
		t.Errorf("CacheWriteTokens = %d, want 15", r.CacheWriteTokens)
	}
}

func TestMergeRecords_DifferentKeys(t *testing.T) {
	in := []Record{
		{Date: "2026-03-15", Provider: "claude", Model: "claude-sonnet-4-5", InputTokens: 100},
		{Date: "2026-03-16", Provider: "claude", Model: "claude-sonnet-4-5", InputTokens: 200},
		{Date: "2026-03-15", Provider: "codex", Model: "gpt-5", InputTokens: 300},
	}
	got := mergeRecords(in)
	if len(got) != 3 {
		t.Errorf("expected 3 records, got %d", len(got))
	}
}

func TestMergeRecords_DifferentModelSameDate(t *testing.T) {
	in := []Record{
		{Date: "2026-03-15", Provider: "claude", Model: "claude-sonnet-4-5", InputTokens: 100},
		{Date: "2026-03-15", Provider: "claude", Model: "claude-haiku-4-5", InputTokens: 200},
	}
	got := mergeRecords(in)
	if len(got) != 2 {
		t.Errorf("expected 2 records (different models), got %d", len(got))
	}
}

func TestMergeRecords_TripleSame(t *testing.T) {
	in := []Record{
		{Date: "2026-01-01", Provider: "claude", Model: "m", InputTokens: 10},
		{Date: "2026-01-01", Provider: "claude", Model: "m", InputTokens: 20},
		{Date: "2026-01-01", Provider: "claude", Model: "m", InputTokens: 30},
	}
	got := mergeRecords(in)
	if len(got) != 1 || got[0].InputTokens != 60 {
		t.Errorf("triple merge: got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// normalizeClaudeModel
// ---------------------------------------------------------------------------

func TestNormalizeClaudeModel_AnthropicPrefix(t *testing.T) {
	got := normalizeClaudeModel("anthropic.claude-sonnet-4-5-20250514")
	if got != "claude-sonnet-4-5-20250514" {
		t.Errorf("got %q, want %q", got, "claude-sonnet-4-5-20250514")
	}
}

func TestNormalizeClaudeModel_VertexPrefix(t *testing.T) {
	got := normalizeClaudeModel("us.anthropic.claude-sonnet-4-5-20250514")
	if got != "claude-sonnet-4-5-20250514" {
		t.Errorf("got %q, want %q", got, "claude-sonnet-4-5-20250514")
	}
}

func TestNormalizeClaudeModel_EuVertex(t *testing.T) {
	got := normalizeClaudeModel("eu.anthropic.claude-haiku-4-5")
	if got != "claude-haiku-4-5" {
		t.Errorf("got %q, want %q", got, "claude-haiku-4-5")
	}
}

func TestNormalizeClaudeModel_NoPrefix(t *testing.T) {
	got := normalizeClaudeModel("claude-opus-4-6")
	if got != "claude-opus-4-6" {
		t.Errorf("got %q, want %q", got, "claude-opus-4-6")
	}
}

func TestNormalizeClaudeModel_Empty(t *testing.T) {
	got := normalizeClaudeModel("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestNormalizeClaudeModel_JustAnthropic(t *testing.T) {
	got := normalizeClaudeModel("anthropic.")
	if got != "" {
		t.Errorf("got %q, want empty (anthropic. with nothing after)", got)
	}
}

func TestNormalizeClaudeModel_DeepVertex(t *testing.T) {
	got := normalizeClaudeModel("asia-east1.anthropic.claude-3-opus")
	if got != "claude-3-opus" {
		t.Errorf("got %q, want %q", got, "claude-3-opus")
	}
}

// ---------------------------------------------------------------------------
// extractDateFromPath
// ---------------------------------------------------------------------------

func TestExtractDateFromPath_Valid(t *testing.T) {
	got := extractDateFromPath("/home/user/.codex/sessions/2026/03/26/rollout-abc.jsonl")
	if got != "2026-03-26" {
		t.Errorf("got %q, want %q", got, "2026-03-26")
	}
}

func TestExtractDateFromPath_NoSessionsDir(t *testing.T) {
	got := extractDateFromPath("/home/user/file.jsonl")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractDateFromPath_Empty(t *testing.T) {
	got := extractDateFromPath("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractDateFromPath_WindowsSeparators(t *testing.T) {
	got := extractDateFromPath(`C:\Users\foo\.codex\sessions\2026\01\14\rollout.jsonl`)
	if got != "2026-01-14" {
		t.Errorf("got %q, want %q (Windows path separators)", got, "2026-01-14")
	}
}

func TestExtractDateFromPath_SessionsAtEnd(t *testing.T) {
	got := extractDateFromPath("/a/sessions")
	if got != "" {
		t.Errorf("got %q, want empty (not enough segments after sessions)", got)
	}
}

func TestExtractDateFromPath_MultipleSessionsDirs(t *testing.T) {
	// First matching "sessions" wins
	got := extractDateFromPath("/a/sessions/2025/12/01/b/sessions/2026/03/26/file.jsonl")
	if got != "2025-12-01" {
		t.Errorf("got %q, want %q (first sessions match)", got, "2025-12-01")
	}
}

// ---------------------------------------------------------------------------
// bytesContains
// ---------------------------------------------------------------------------

func TestBytesContains_Match(t *testing.T) {
	if !bytesContains([]byte(`{"type":"assistant"}`), `"type":"assistant"`) {
		t.Error("expected match")
	}
}

func TestBytesContains_NoMatch(t *testing.T) {
	if bytesContains([]byte(`{"type":"user"}`), `"type":"assistant"`) {
		t.Error("expected no match")
	}
}

func TestBytesContains_EmptyData(t *testing.T) {
	if bytesContains([]byte{}, "anything") {
		t.Error("empty data should not match non-empty substr")
	}
}

func TestBytesContains_EmptySubstr(t *testing.T) {
	if !bytesContains([]byte("hello"), "") {
		t.Error("empty substr should match any data")
	}
}

func TestBytesContains_BothEmpty(t *testing.T) {
	if !bytesContains([]byte{}, "") {
		t.Error("both empty should match")
	}
}
