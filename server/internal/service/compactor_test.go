package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func makeMessages(n int, size int) []Message {
	msgs := make([]Message, n)
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = Message{
			Role:    role,
			Content: strings.Repeat("x", size),
		}
	}
	return msgs
}

func TestCompactor_NeedsCompaction(t *testing.T) {
	c := NewCompactor()
	c.MaxChars = 100

	msgs := makeMessages(5, 30) // 150 chars total
	if !c.NeedsCompaction(msgs) {
		t.Error("should need compaction at 150 chars with max 100")
	}

	msgs = makeMessages(2, 20) // 40 chars total
	if c.NeedsCompaction(msgs) {
		t.Error("should not need compaction at 40 chars with max 100")
	}
}

func TestMicroCompact(t *testing.T) {
	c := NewCompactor()
	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "tool", Content: strings.Repeat("a", 1000)},
		{Role: "assistant", Content: "done"},
	}

	result, err := c.Compact(context.Background(), msgs, MicroCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	if result.Messages[1].Content != truncateContent(strings.Repeat("a", 1000), 200) {
		t.Error("tool output should be truncated")
	}
	if result.Messages[0].Content != "hello" {
		t.Error("user message should be unchanged")
	}
	if result.CompactedLen >= result.OriginalLen {
		t.Error("compacted length should be less than original")
	}
}

func TestAutoCompact(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 3
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "reply1"},
		{Role: "user", Content: "msg2"},
		{Role: "assistant", Content: "reply2"},
		{Role: "user", Content: "msg3"},
		{Role: "assistant", Content: "reply3"},
	}

	result, err := c.Compact(context.Background(), msgs, AutoCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	// Should have summary + 3 recent messages
	if len(result.Messages) != 4 { // 1 summary system + 3 recent
		t.Errorf("expected 4 messages, got %d", len(result.Messages))
	}
	if result.Summary == "" {
		t.Error("summary should not be empty after auto compact")
	}
	if !strings.Contains(result.Messages[0].Content, "Earlier conversation") {
		t.Error("first message should be the summary")
	}
}

func TestAutoCompact_NoCompactionNeeded(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 10
	msgs := []Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "reply1"},
	}

	result, err := c.Compact(context.Background(), msgs, AutoCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	if len(result.Messages) != 2 {
		t.Errorf("should not compact when messages < KeepRecent, got %d", len(result.Messages))
	}
}

func TestSnipCompact(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 2
	msgs := []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "old1"},
		{Role: "assistant", Content: "old2"},
		{Role: "user", Content: "old3"},
		{Role: "user", Content: "recent1"},
		{Role: "assistant", Content: "recent2"},
	}

	result, err := c.Compact(context.Background(), msgs, SnipCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	// Should have: system + compaction marker + 2 recent
	if len(result.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(result.Messages))
	}
	// First should be system message preserved
	if result.Messages[0].Content != "system prompt" {
		t.Error("system message should be preserved")
	}
	// Second should be compaction summary
	if !strings.Contains(result.Messages[1].Content, "Compacted") {
		t.Error("should have compaction summary")
	}
}

func TestCompactionResult_Ratio(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 2
	msgs := []Message{
		{Role: "user", Content: strings.Repeat("a", 10000)},
		{Role: "assistant", Content: strings.Repeat("b", 10000)},
		{Role: "user", Content: strings.Repeat("c", 10000)},
		{Role: "assistant", Content: strings.Repeat("d", 10000)},
		{Role: "user", Content: "recent1"},
		{Role: "assistant", Content: "recent2"},
	}

	result, err := c.Compact(context.Background(), msgs, SnipCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	ratio := float64(result.CompactedLen) / float64(result.OriginalLen)
	if ratio > 0.5 {
		t.Errorf("compacted ratio = %.2f, expected significant reduction", ratio)
	}
}

// ---------------------------------------------------------------------------
// Compact unknown mode
// ---------------------------------------------------------------------------

func TestCompact_UnknownModeReturnsError(t *testing.T) {
	c := NewCompactor()
	_, err := c.Compact(context.Background(), []Message{{Role: "user", Content: "hi"}}, CompactionMode(99))
	if err == nil {
		t.Fatal("expected error for unknown compaction mode")
	}
	if !strings.Contains(err.Error(), "unknown compaction mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AutoCompact with custom Summarizer
// ---------------------------------------------------------------------------

func TestAutoCompact_CustomSummarizer(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 1
	c.Summarizer = func(ctx context.Context, msgs []Message) (string, error) {
		return fmt.Sprintf("custom summary of %d messages", len(msgs)), nil
	}

	msgs := []Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "reply1"},
		{Role: "user", Content: "msg2"},
	}

	result, err := c.Compact(context.Background(), msgs, AutoCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	if result.Summary != "custom summary of 2 messages" {
		t.Errorf("expected custom summary, got %q", result.Summary)
	}
	if !strings.Contains(result.Messages[0].Content, "custom summary") {
		t.Error("first message should be the custom summary")
	}
}

func TestAutoCompact_SummarizerError(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 1
	c.Summarizer = func(ctx context.Context, msgs []Message) (string, error) {
		return "", fmt.Errorf("LLM unavailable")
	}

	msgs := []Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "reply1"},
		{Role: "user", Content: "msg2"},
	}

	_, err := c.Compact(context.Background(), msgs, AutoCompact)
	if err == nil {
		t.Fatal("expected error when summarizer fails")
	}
	if !strings.Contains(err.Error(), "LLM unavailable") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SnipCompact edge cases
// ---------------------------------------------------------------------------

func TestSnipCompact_AllSystemMessages(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 3 // keep all messages
	msgs := []Message{
		{Role: "system", Content: "sys1"},
		{Role: "system", Content: "sys2"},
		{Role: "system", Content: "sys3"},
	}

	result, err := c.Compact(context.Background(), msgs, SnipCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	// All 3 system messages preserved, nothing removed since KeepRecent >= len.
	if len(result.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result.Messages))
	}
	for _, m := range result.Messages {
		if m.Role != "system" {
			t.Errorf("expected only system messages, got role %q", m.Role)
		}
	}
}

func TestSnipCompact_ShorterThanKeepRecent(t *testing.T) {
	c := NewCompactor()
	c.KeepRecent = 10
	msgs := []Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "reply1"},
	}

	result, err := c.Compact(context.Background(), msgs, SnipCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	// When fewer messages than KeepRecent, all are in "recent" section.
	// System msgs = 0, removed = 0 (start >= len), so no compaction marker.
	// Result should be just the recent messages.
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
}

// ---------------------------------------------------------------------------
// truncateContent boundary tests
// ---------------------------------------------------------------------------

func TestTruncateContent_AtBoundary(t *testing.T) {
	// Exactly at maxLen — no truncation.
	content := strings.Repeat("a", 200)
	got := truncateContent(content, 200)
	if got != content {
		t.Error("content at exact boundary should not be truncated")
	}
}

func TestTruncateContent_AboveBoundary(t *testing.T) {
	content := strings.Repeat("a", 300)
	got := truncateContent(content, 200)
	if len(got) > 220 { // 200 + "\n... [truncated]"
		t.Errorf("truncated content too long: %d chars", len(got))
	}
	if !strings.Contains(got, "[truncated]") {
		t.Error("truncated content should contain truncation marker")
	}
}

// ---------------------------------------------------------------------------
// defaultSummary truncation
// ---------------------------------------------------------------------------

func TestDefaultSummary_LongContent(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: strings.Repeat("x", 5000)},
	}
	summary := defaultSummary(msgs)

	// The summary function truncates individual previews to 100 chars,
	// then the joined result to 2000 chars.
	if len(summary) > 2100 { // some overhead for prefix
		t.Errorf("summary too long: %d chars", len(summary))
	}
	if !strings.Contains(summary, "Earlier conversation") {
		t.Error("summary should have standard prefix")
	}
}

func TestDefaultSummary_ManyMessages(t *testing.T) {
	var msgs []Message
	for i := 0; i < 100; i++ {
		msgs = append(msgs, Message{Role: "user", Content: strings.Repeat("a", 50)})
	}
	summary := defaultSummary(msgs)

	// 100 messages × ~60 chars each = ~6000 chars → must be truncated to ~2000.
	if len(summary) > 2100 {
		t.Errorf("summary for many messages too long: %d chars", len(summary))
	}
	if !strings.Contains(summary, "summary truncated") {
		t.Error("expected summary truncation marker for many messages")
	}
}

// ---------------------------------------------------------------------------
// totalChars edge cases
// ---------------------------------------------------------------------------

func TestTotalChars_EmptySlice(t *testing.T) {
	if totalChars(nil) != 0 {
		t.Error("totalChars(nil) should be 0")
	}
	if totalChars([]Message{}) != 0 {
		t.Error("totalChars([]Message{}) should be 0")
	}
}

func TestTotalChars_MixedRoles(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "abc"},
		{Role: "user", Content: "de"},
		{Role: "tool", Content: ""},
	}
	if got := totalChars(msgs); got != 5 {
		t.Errorf("totalChars = %d, want 5", got)
	}
}

// ---------------------------------------------------------------------------
// NeedsCompaction boundary
// ---------------------------------------------------------------------------

func TestNeedsCompaction_ExactBoundary(t *testing.T) {
	c := NewCompactor()
	c.MaxChars = 100

	msgs := makeMessages(2, 50) // exactly 100 chars
	if c.NeedsCompaction(msgs) {
		t.Error("should NOT need compaction when exactly at MaxChars")
	}

	msgs = makeMessages(2, 51) // 102 chars
	if !c.NeedsCompaction(msgs) {
		t.Error("should need compaction when 1 char over MaxChars")
	}
}

// ---------------------------------------------------------------------------
// MicroCompact edge cases
// ---------------------------------------------------------------------------

func TestMicroCompact_ShortToolOutput(t *testing.T) {
	c := NewCompactor()
	msgs := []Message{
		{Role: "tool", Content: "short output"},
		{Role: "assistant", Content: "done"},
	}

	result, err := c.Compact(context.Background(), msgs, MicroCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	// Tool output under 500 chars should not be truncated.
	if result.Messages[0].Content != "short output" {
		t.Errorf("short tool output should be preserved, got %q", result.Messages[0].Content)
	}
}

func TestMicroCompact_EmptyMessages(t *testing.T) {
	c := NewCompactor()
	result, err := c.Compact(context.Background(), nil, MicroCompact)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	if len(result.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result.Messages))
	}
}
