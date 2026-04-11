package service

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCoalescer_FoldMerge(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(50*time.Millisecond, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	// Push three text chunks — should merge into one event.
	c.Push("text", "hello ", ClassFold)
	c.Push("text", "world", ClassFold)
	c.Push("text", "!", ClassFold)

	// Wait for the sliding window to expire.
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 1 {
		t.Fatalf("expected 1 merged event, got %d", len(events))
	}
	if events[0].Type != "text" {
		t.Errorf("expected type text, got %s", events[0].Type)
	}
	if events[0].Content != "hello world!" {
		t.Errorf("expected content 'hello world!', got %q", events[0].Content)
	}
}

func TestCoalescer_TypeSwitchFlushes(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(200*time.Millisecond, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	// Push text, then thinking — type switch should flush text immediately.
	c.Push("text", "some text", ClassFold)
	c.Push("thinking", "thinking...", ClassFold)

	mu.Lock()
	if len(events) != 1 {
		mu.Unlock()
		t.Fatalf("expected 1 event after type switch, got %d", len(events))
	}
	if events[0].Type != "text" {
		mu.Unlock()
		t.Errorf("expected flushed type text, got %s", events[0].Type)
	}
	mu.Unlock()

	// Wait for thinking to flush.
	time.Sleep(350 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 2 {
		t.Fatalf("expected 2 total events, got %d", len(events))
	}
	if events[1].Type != "thinking" {
		t.Errorf("expected second type thinking, got %s", events[1].Type)
	}
	if events[1].Content != "thinking..." {
		t.Errorf("expected content 'thinking...', got %q", events[1].Content)
	}
}

func TestCoalescer_FlushInterruptsFold(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(500*time.Millisecond, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	// Push some fold events, then a flush event.
	c.Push("text", "partial ", ClassFold)
	c.Push("text", "message", ClassFold)
	c.Push("error", "something broke", ClassFlush)

	mu.Lock()
	defer mu.Unlock()

	// Should have: merged text + error = 2 events.
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "text" || events[0].Content != "partial message" {
		t.Errorf("event 0: expected text 'partial message', got %s %q", events[0].Type, events[0].Content)
	}
	if events[1].Type != "error" || events[1].Content != "something broke" {
		t.Errorf("event 1: expected error 'something broke', got %s %q", events[1].Type, events[1].Content)
	}
}

func TestCoalescer_ToolUseFlushesFold(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(500*time.Millisecond, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	c.Push("thinking", "hmm...", ClassFold)
	c.PushToolUse("call-1", "read_file", map[string]any{"path": "foo.go"})

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "thinking" {
		t.Errorf("event 0: expected thinking, got %s", events[0].Type)
	}
	if events[1].Type != "tool_use" || events[1].Tool != "read_file" || events[1].CallID != "call-1" {
		t.Errorf("event 1: expected tool_use read_file call-1, got %s %s %s", events[1].Type, events[1].Tool, events[1].CallID)
	}
}

func TestCoalescer_ToolResultFlushesFold(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(500*time.Millisecond, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	c.Push("text", "partial", ClassFold)
	c.PushToolResult("call-1", "read_file", "file contents")

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].Type != "tool_result" || events[1].Tool != "read_file" || events[1].Output != "file contents" {
		t.Errorf("event 1: expected tool_result, got %+v", events[1])
	}
}

func TestCoalescer_CloseFlushes(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(10*time.Second, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	c.Push("text", "buffered", ClassFold)
	c.Close()

	mu.Lock()
	if len(events) != 1 {
		mu.Unlock()
		t.Fatalf("expected 1 event after Close, got %d", len(events))
	}
	if events[0].Content != "buffered" {
		t.Errorf("expected 'buffered', got %q", events[0].Content)
	}
	mu.Unlock()

	// Close is idempotent.
	c.Close()
	c.Push("text", "ignored", ClassFold)

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 {
		t.Errorf("expected still 1 event after second Close, got %d", len(events))
	}
}

func TestSummarizer_Empty(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSummarizer_Basic(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize([]string{"hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty summary")
	}
	// Should contain both messages.
	if !contains(result, "hello") || !contains(result, "world") {
		t.Errorf("summary missing expected content: %q", result)
	}
}

func TestSummarizer_SingleMessage(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize([]string{"only one"})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result, "only one") {
		t.Errorf("expected single message in summary, got %q", result)
	}
	if !contains(result, "1 messages") {
		t.Errorf("expected '1 messages' prefix, got %q", result)
	}
}

func TestSummarizer_WhitespaceFiltered(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize([]string{"", "   ", "\t\n"})
	if err != nil {
		t.Fatal(err)
	}
	// All whitespace/empty → should return empty (no valid parts).
	if result != "" {
		t.Errorf("expected empty for all-whitespace input, got %q", result)
	}
}

func TestSummarizer_WhitespaceMixedWithContent(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize([]string{"", "real message", "  "})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result, "real message") {
		t.Errorf("expected 'real message' in summary, got %q", result)
	}
	// Only 1 valid message, so "1 messages".
	if !contains(result, "1 messages") {
		t.Errorf("expected '1 messages', got %q", result)
	}
}

func TestSummarizer_LongMessageTruncated(t *testing.T) {
	s := &Summarizer{}
	long := strings.Repeat("x", 300)
	result, err := s.Summarize([]string{long})
	if err != nil {
		t.Fatal(err)
	}
	// Should be truncated to 200 chars + "..." in the summary.
	if !contains(result, strings.Repeat("x", 200)+"...") {
		t.Error("long message should be truncated to 200 + '...'")
	}
	if contains(result, strings.Repeat("x", 300)) {
		t.Error("full 300-char message should not appear in summary")
	}
}

func TestSummarizer_ExactBoundary(t *testing.T) {
	s := &Summarizer{}
	exact := strings.Repeat("a", 200)
	result, err := s.Summarize([]string{exact})
	if err != nil {
		t.Fatal(err)
	}
	// Exactly 200 chars — no truncation, no "...".
	if !contains(result, exact) {
		t.Error("200-char message should appear untruncated")
	}
}

func TestSummarizer_MessageIndexing(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize([]string{"first", "second", "third"})
	if err != nil {
		t.Fatal(err)
	}
	// Messages should be numbered [1], [2], [3].
	if !contains(result, "[1]") || !contains(result, "[2]") || !contains(result, "[3]") {
		t.Errorf("expected numbered messages [1][2][3], got %q", result)
	}
}

func TestSummarizer_201CharTruncation(t *testing.T) {
	s := &Summarizer{}
	msg := strings.Repeat("a", 201)
	result, err := s.Summarize([]string{msg})
	if err != nil {
		t.Fatal(err)
	}
	// 201 chars > 200, so should be truncated with "..."
	if !contains(result, "...") {
		t.Error("201-char message should be truncated with ...")
	}
	// Should contain exactly 200 'a's before the "..."
	if !contains(result, strings.Repeat("a", 200)+"...") {
		t.Error("truncated message should have 200 chars + ...")
	}
}

func TestSummarizer_MixedWhitespaceInterleaved(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize([]string{"", "real", "", "another", ""})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result, "2 messages") {
		t.Errorf("expected 2 messages, got %q", result)
	}
	if !contains(result, "real") || !contains(result, "another") {
		t.Error("content should be preserved")
	}
	// Indexing should skip blanks: "real" was msg #2, "another" was msg #4.
	if !contains(result, "[2] real") {
		t.Errorf("expected [2] real, got %q", result)
	}
	if !contains(result, "[4] another") {
		t.Errorf("expected [4] another, got %q", result)
	}
}

func TestSummarizer_200CharIsUnicodeSafe(t *testing.T) {
	s := &Summarizer{}
	// 100 Chinese chars = 300 bytes but 100 runes. Truncation uses byte-length,
	// so this should NOT be truncated (100 runes < 200 bytes? no — 300 bytes > 200).
	msg := strings.Repeat("中", 100)
	result, err := s.Summarize([]string{msg})
	if err != nil {
		t.Fatal(err)
	}
	// 100 * 3 bytes = 300 bytes > 200 bytes → truncated. Should have "..."
	if !contains(result, "...") {
		t.Errorf("100 Chinese chars (300 bytes) should trigger truncation, got %q", result)
	}
}

func TestSummarizer_AllWhitespaceAfterContent(t *testing.T) {
	s := &Summarizer{}
	result, err := s.Summarize([]string{"keep", "   ", "\t", "  \n  "})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result, "1 messages") {
		t.Errorf("expected 1 message, got %q", result)
	}
	if !contains(result, "keep") {
		t.Error("content should be preserved")
	}
}

// --- Post-close guard tests ---

func TestCoalescer_PushToolUseAfterClose(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(50*time.Millisecond, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	c.Close()
	c.PushToolUse("call-1", "read_file", nil)

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 0 {
		t.Errorf("expected 0 events after Close + PushToolUse, got %d", len(events))
	}
}

func TestCoalescer_PushToolResultAfterClose(t *testing.T) {
	var mu sync.Mutex
	var events []CoalescedEvent

	c := NewCoalescer(50*time.Millisecond, func(ev CoalescedEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	c.Close()
	c.PushToolResult("call-1", "read_file", "output")

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 0 {
		t.Errorf("expected 0 events after Close + PushToolResult, got %d", len(events))
	}
}

func TestStepCoalescer_FlushToolUseAfterClose(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.Close()
	sc.FlushToolUse("call-1", "read_file", nil)

	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("expected 0 writes after Close + FlushToolUse, got %d", len(writes))
	}
}

func TestStepCoalescer_FlushToolResultAfterClose(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.Close()
	sc.FlushToolResult("call-1", "read_file", "output")

	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("expected 0 writes after Close + FlushToolResult, got %d", len(writes))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- StepCoalescer tests ---

type stepWrite struct {
	StepType string
	ToolName string
	CallID   string
	Content  string
}

func TestStepCoalescer_SameTypeMerge(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.PushThinking("first thought")
	sc.PushThinking("second thought")
	sc.PushThinking("third thought")

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != 1 {
		t.Fatalf("expected 1 merged write, got %d", len(writes))
	}
	if writes[0].StepType != "thinking" {
		t.Errorf("expected step type 'thinking', got %q", writes[0].StepType)
	}
	if writes[0].Content != "first thought\nsecond thought\nthird thought" {
		t.Errorf("expected merged content, got %q", writes[0].Content)
	}
}

func TestStepCoalescer_TypeSwitchFlushes(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(200*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.PushThinking("thinking...")
	sc.PushText("some text")

	mu.Lock()
	if len(writes) != 1 {
		mu.Unlock()
		t.Fatalf("expected 1 write after type switch, got %d", len(writes))
	}
	if writes[0].StepType != "thinking" {
		mu.Unlock()
		t.Errorf("expected flushed step type 'thinking', got %q", writes[0].StepType)
	}
	mu.Unlock()

	time.Sleep(350 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != 2 {
		t.Fatalf("expected 2 total writes, got %d", len(writes))
	}
	if writes[1].StepType != "text" || writes[1].Content != "some text" {
		t.Errorf("expected text write, got %s %q", writes[1].StepType, writes[1].Content)
	}
}

func TestStepCoalescer_ToolFlushesImmediate(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(500*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.PushThinking("hmm...")
	sc.FlushToolUse("call-1", "read_file", []byte(`{"path":"foo.go"}`))

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != 2 {
		t.Fatalf("expected 2 writes (flushed thinking + tool), got %d", len(writes))
	}
	if writes[0].StepType != "thinking" {
		t.Errorf("expected first write step type 'thinking', got %q", writes[0].StepType)
	}
	if writes[1].ToolName != "read_file" {
		t.Errorf("expected second write 'read_file', got %q", writes[1].ToolName)
	}
}

func TestStepCoalescer_ToolResultFlushesImmediate(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(500*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.PushText("partial text")
	sc.FlushToolResult("call-1", "read_file", "file contents")

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != 2 {
		t.Fatalf("expected 2 writes, got %d", len(writes))
	}
	if writes[0].StepType != "text" {
		t.Errorf("expected first step type 'text', got %q", writes[0].StepType)
	}
	if writes[1].ToolName != "read_file" || writes[1].Content != "file contents" {
		t.Errorf("expected tool_result write, got %+v", writes[1])
	}
}

func TestStepCoalescer_WindowReset(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(100*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.PushThinking("a")
	time.Sleep(60 * time.Millisecond) // within window, timer resets
	sc.PushThinking("b")
	time.Sleep(60 * time.Millisecond) // within window again, timer resets
	sc.PushThinking("c")
	time.Sleep(150 * time.Millisecond) // now window expires

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != 1 {
		t.Fatalf("expected 1 merged write, got %d", len(writes))
	}
	if writes[0].Content != "a\nb\nc" {
		t.Errorf("expected 'a\\nb\\nc', got %q", writes[0].Content)
	}
}

func TestStepCoalescer_Truncation(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(50*time.Millisecond, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})
	sc.MaxChars = 10

	sc.PushText("abcdefghijklmnop") // 16 chars, should truncate to 10

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != 1 {
		t.Fatalf("expected 1 write, got %d", len(writes))
	}
	if len(writes[0].Content) != 10 {
		t.Errorf("expected truncated to 10 chars, got %d: %q", len(writes[0].Content), writes[0].Content)
	}
}

func TestStepCoalescer_CloseFlushes(t *testing.T) {
	var mu sync.Mutex
	var writes []stepWrite

	sc := NewStepCoalescer(10*time.Second, func(stepType, toolName, callID, content string) {
		mu.Lock()
		writes = append(writes, stepWrite{stepType, toolName, callID, content})
		mu.Unlock()
	})

	sc.PushThinking("buffered")
	sc.Close()

	mu.Lock()
	if len(writes) != 1 {
		mu.Unlock()
		t.Fatalf("expected 1 write after Close, got %d", len(writes))
	}
	if writes[0].Content != "buffered" {
		t.Errorf("expected 'buffered', got %q", writes[0].Content)
	}
	mu.Unlock()

	// Close is idempotent.
	sc.Close()
	sc.PushThinking("ignored")

	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 1 {
		t.Errorf("expected still 1 write after second Close, got %d", len(writes))
	}
}
