package service

import (
	"context"
	"fmt"
	"strings"
)

// Message represents a single conversation turn for compaction.
type Message struct {
	Role    string // "system", "user", "assistant", "tool"
	Content string
}

// CompactionResult is the output of a compaction pass.
type CompactionResult struct {
	Messages    []Message
	Summary     string
	OriginalLen int // total characters before compaction
	CompactedLen int // total characters after compaction
}

// CompactionMode controls which strategy to use.
type CompactionMode int

const (
	// MicroCompact drops tool_output from assistant messages, keeping only
	// the tool name + summary. Best for light trimming.
	MicroCompact CompactionMode = iota

	// AutoCompact replaces the oldest N messages with a summary string.
	// Keeps the most recent messages intact.
	AutoCompact

	// SnipCompact removes messages between markers (e.g. checkpoint boundaries).
	// Preserves system + recent context only.
	SnipCompact
)

// Compactor manages conversation context compaction.
type Compactor struct {
	// MaxChars is the target character budget. Compaction only runs if
	// total message length exceeds this.
	MaxChars int

	// KeepRecent is the number of recent messages to always preserve.
	KeepRecent int

	// Summarizer is an optional function that produces a summary from messages.
	// If nil, a simple truncation-based summary is used.
	Summarizer func(ctx context.Context, messages []Message) (string, error)
}

// NewCompactor creates a compactor with sensible defaults.
func NewCompactor() *Compactor {
	return &Compactor{
		MaxChars:   100000, // ~25K tokens
		KeepRecent: 10,
	}
}

// NeedsCompaction returns true if the message list exceeds the character budget.
func (c *Compactor) NeedsCompaction(messages []Message) bool {
	return totalChars(messages) > c.MaxChars
}

// Compact runs the specified compaction mode on the message list.
func (c *Compactor) Compact(ctx context.Context, messages []Message, mode CompactionMode) (*CompactionResult, error) {
	originalLen := totalChars(messages)

	switch mode {
	case MicroCompact:
		return c.microCompact(messages, originalLen)
	case AutoCompact:
		return c.autoCompact(ctx, messages, originalLen)
	case SnipCompact:
		return c.snipCompact(messages, originalLen)
	default:
		return nil, fmt.Errorf("unknown compaction mode: %d", mode)
	}
}

// microCompact strips tool output from assistant messages, keeping only
// the tool name reference. This is the lightest compaction — it reduces
// token usage by removing large tool results while preserving the
// conversation structure.
func (c *Compactor) microCompact(messages []Message, originalLen int) (*CompactionResult, error) {
	var compacted []Message
	for _, msg := range messages {
		if msg.Role == "tool" && len(msg.Content) > 500 {
			// Summarize large tool outputs
			compacted = append(compacted, Message{
				Role:    msg.Role,
				Content: truncateContent(msg.Content, 200),
			})
		} else {
			compacted = append(compacted, msg)
		}
	}

	return &CompactionResult{
		Messages:     compacted,
		Summary:      "",
		OriginalLen:  originalLen,
		CompactedLen: totalChars(compacted),
	}, nil
}

// autoCompact replaces the oldest messages with a summary, keeping only
// the most recent KeepRecent messages intact. If a Summarizer function is
// provided, it's used to generate the summary; otherwise, a simple
// concatenation-and-truncate approach is used.
func (c *Compactor) autoCompact(ctx context.Context, messages []Message, originalLen int) (*CompactionResult, error) {
	if len(messages) <= c.KeepRecent {
		return &CompactionResult{
			Messages:     messages,
			Summary:      "",
			OriginalLen:  originalLen,
			CompactedLen: originalLen,
		}, nil
	}

	splitIdx := len(messages) - c.KeepRecent
	toSummarize := messages[:splitIdx]
	toKeep := messages[splitIdx:]

	var summary string
	if c.Summarizer != nil {
		var err error
		summary, err = c.Summarizer(ctx, toSummarize)
		if err != nil {
			return nil, fmt.Errorf("summarizer failed: %w", err)
		}
	} else {
		summary = defaultSummary(toSummarize)
	}

	// Insert summary as a system message, then keep recent messages
	result := []Message{
		{Role: "system", Content: summary},
	}
	result = append(result, toKeep...)

	return &CompactionResult{
		Messages:     result,
		Summary:      summary,
		OriginalLen:  originalLen,
		CompactedLen: totalChars(result),
	}, nil
}

// snipCompact removes messages between checkpoint boundaries. It preserves
// the system messages and the most recent messages, removing everything in
// between (typically old tool interactions).
func (c *Compactor) snipCompact(messages []Message, originalLen int) (*CompactionResult, error) {
	var systemMsgs []Message
	var recentMsgs []Message

	// Collect system messages (always keep)
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsgs = append(systemMsgs, msg)
		}
	}

	// Keep the most recent messages
	start := len(messages) - c.KeepRecent
	if start < 0 {
		start = 0
	}
	recentMsgs = messages[start:]

	// Build summary of removed section
	summary := ""
	if start > len(systemMsgs) {
		removed := messages[len(systemMsgs):start]
		if len(removed) > 0 {
			summary = fmt.Sprintf("[Compacted: %d messages removed]", len(removed))
		}
	}

	result := make([]Message, 0, len(systemMsgs)+len(recentMsgs))
	result = append(result, systemMsgs...)
	if summary != "" {
		result = append(result, Message{Role: "system", Content: summary})
	}
	// Only append recent messages that aren't already included as system messages.
	for _, m := range recentMsgs {
		if m.Role != "system" {
			result = append(result, m)
		}
	}

	return &CompactionResult{
		Messages:     result,
		Summary:      summary,
		OriginalLen:  originalLen,
		CompactedLen: totalChars(result),
	}, nil
}

func totalChars(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content)
	}
	return total
}

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "\n... [truncated]"
}

func defaultSummary(messages []Message) string {
	var parts []string
	for _, msg := range messages {
		role := msg.Role
		preview := msg.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", role, preview))
	}
	joined := strings.Join(parts, "\n")
	if len(joined) > 2000 {
		joined = joined[:2000] + "\n... [summary truncated]"
	}
	return "## Earlier conversation summary\n\n" + joined
}
