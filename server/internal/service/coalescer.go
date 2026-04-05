package service

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// EventClass controls how the coalescer batches events.
type EventClass int

const (
	// ClassFold accumulates events of the same type within the window.
	// When the window expires, all accumulated events are merged into one.
	ClassFold EventClass = iota
	// ClassFlush immediately emits the event, flushing any pending fold buffer first.
	ClassFlush
)

// CoalescedEvent is a merged event produced by the coalescer.
type CoalescedEvent struct {
	Type    string
	Content string
	CallID  string
	Tool    string
	Input   any
	Output  string
}

// Coalescer batches events within a sliding window.
// ClassFold events are accumulated; the timer starts on the first fold event
// and resets on each new fold event (sliding window). When the window expires,
// all accumulated fold events are merged and emitted via onDone.
// ClassFlush events and tool events immediately flush any pending fold buffer
// before emitting themselves.
type Coalescer struct {
	mu     sync.Mutex
	window time.Duration
	onDone func(CoalescedEvent)

	// fold buffer: accumulates ClassFold events by type
	pendingType string
	pending     []string

	// timer management
	timer    *time.Timer
	timerSet  bool
	stopped  bool
}

// NewCoalescer creates a coalescer with the given window and callback.
func NewCoalescer(window time.Duration, onDone func(CoalescedEvent)) *Coalescer {
	return &Coalescer{window: window, onDone: onDone}
}

// Push adds an event to the coalescer.
func (c *Coalescer) Push(eventType, content string, class EventClass) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	switch class {
	case ClassFold:
		// If the buffer has events of a different type, flush it first.
		if len(c.pending) > 0 && c.pendingType != eventType {
			c.flushLocked()
		}
		c.pendingType = eventType
		c.pending = append(c.pending, content)

		// Start or reset the sliding window timer.
		if !c.timerSet {
			c.timerSet = true
			c.timer = time.AfterFunc(c.window, func() {
				c.mu.Lock()
				defer c.mu.Unlock()
				c.flushLocked()
			})
		} else {
			c.timer.Reset(c.window)
		}

	case ClassFlush:
		// Flush any pending fold buffer first, then emit the flush event immediately.
		c.flushLocked()
		if c.onDone != nil {
			c.onDone(CoalescedEvent{Type: eventType, Content: content})
		}
	}
}

// PushToolUse records a tool-use event.
// Flushes any pending fold buffer, then emits the tool-use immediately.
func (c *Coalescer) PushToolUse(callID, name string, input any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	c.flushLocked()
	if c.onDone != nil {
		c.onDone(CoalescedEvent{
			Type:   "tool_use",
			CallID: callID,
			Tool:   name,
			Input:  input,
		})
	}
}

// PushToolResult records a tool-result event.
// Flushes any pending fold buffer, then emits the tool-result immediately.
func (c *Coalescer) PushToolResult(callID, name, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	c.flushLocked()
	if c.onDone != nil {
		c.onDone(CoalescedEvent{
			Type:    "tool_result",
			CallID:  callID,
			Tool:    name,
			Output:  content,
		})
	}
}

// Close flushes any remaining events and stops the coalescer.
func (c *Coalescer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}
	c.stopped = true

	if c.timer != nil {
		c.timer.Stop()
	}
	c.flushLocked()
}

// flushLocked emits all pending fold events as a single merged CoalescedEvent.
// Must be called with c.mu held.
func (c *Coalescer) flushLocked() {
	if len(c.pending) == 0 {
		return
	}

	merged := strings.Join(c.pending, "")
	evtType := c.pendingType
	c.pending = nil
	c.pendingType = ""

	if c.onDone != nil {
		c.onDone(CoalescedEvent{Type: evtType, Content: merged})
	}
}

// StepCoalescer merges consecutive same-type thinking/text events at the
// RecordStep level to reduce DB writes. Tool events always flush immediately.
//
// Consecutive events of the same mergeable type (thinking, text) are accumulated
// in a buffer. A sliding window timer (default 350ms) resets on each new event.
// When the window expires or a different type arrives, the buffer is flushed:
// contents are joined with "\n", truncated to MaxChars (default 2000), and
// written to the DB via the provided writeFn.
type StepCoalescer struct {
	mu       sync.Mutex
	window   time.Duration
	MaxChars int
	writeFn  func(toolName string, content string)

	// fold buffer
	pendingType string
	pending     []string

	// timer management
	timer   *time.Timer
	set     bool
	stopped bool
}

// NewStepCoalescer creates a StepCoalescer with the given sliding window and
// write callback. MaxChars defaults to 2000.
func NewStepCoalescer(window time.Duration, writeFn func(string, string)) *StepCoalescer {
	return &StepCoalescer{
		window:   window,
		MaxChars: 2000,
		writeFn:  writeFn,
	}
}

// PushThinking adds a thinking event to the coalescer.
func (sc *StepCoalescer) PushThinking(content string) {
	sc.push("thinking", content)
}

// PushText adds a text event to the coalescer.
func (sc *StepCoalescer) PushText(content string) {
	sc.push("text", content)
}

// push is the internal method that handles fold/flush logic.
func (sc *StepCoalescer) push(eventType, content string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.stopped {
		return
	}

	// If buffer has events of a different type, flush first.
	if len(sc.pending) > 0 && sc.pendingType != eventType {
		sc.flushLocked()
	}

	sc.pendingType = eventType
	sc.pending = append(sc.pending, content)

	// Start or reset sliding window timer.
	if !sc.set {
		sc.set = true
		sc.timer = time.AfterFunc(sc.window, func() {
			sc.mu.Lock()
			defer sc.mu.Unlock()
			sc.flushLocked()
		})
	} else {
		sc.timer.Reset(sc.window)
	}
}

// FlushToolUse flushes any pending fold event, then writes a tool_use step immediately.
func (sc *StepCoalescer) FlushToolUse(callID, name string, inputJSON []byte) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.stopped {
		return
	}

	sc.flushLocked()
	if sc.writeFn != nil {
		sc.writeFn(name, string(inputJSON))
	}
}

// FlushToolResult flushes any pending fold event, then writes a tool_result step immediately.
func (sc *StepCoalescer) FlushToolResult(callID, name, output string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.stopped {
		return
	}

	sc.flushLocked()
	if sc.writeFn != nil {
		sc.writeFn(name, output)
	}
}

// Close flushes any remaining events and stops the coalescer.
func (sc *StepCoalescer) Close() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.stopped {
		return
	}
	sc.stopped = true

	if sc.timer != nil {
		sc.timer.Stop()
	}
	sc.flushLocked()
}

// flushLocked merges all pending events and writes via writeFn.
// Must be called with sc.mu held.
func (sc *StepCoalescer) flushLocked() {
	if len(sc.pending) == 0 {
		return
	}

	merged := strings.Join(sc.pending, "\n")
	if len(merged) > sc.MaxChars {
		merged = merged[:sc.MaxChars]
	}

	toolName := sc.pendingType
	sc.pending = nil
	sc.pendingType = ""

	if sc.writeFn != nil {
		sc.writeFn(toolName, merged)
	}
}

// Summarizer produces a template-based summary from accumulated messages.
type Summarizer struct{}

// Summarize produces a basic summary from the given messages.
// No LLM involved — pure template concatenation.
func (s *Summarizer) Summarize(messages []string) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	var parts []string
	for i, msg := range messages {
		trimmed := strings.TrimSpace(msg)
		if trimmed == "" {
			continue
		}
		// Truncate long messages for summary.
		if len(trimmed) > 200 {
			trimmed = trimmed[:200] + "..."
		}
		parts = append(parts, fmt.Sprintf("[%d] %s", i+1, trimmed))
	}

	if len(parts) == 0 {
		return "", nil
	}

	return fmt.Sprintf("%d messages:\n%s", len(parts), strings.Join(parts, "\n")), nil
}
