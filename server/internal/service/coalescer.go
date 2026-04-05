package service

import "time"

// EventClass controls how the coalescer batches events.
type EventClass int

const (
	ClassFold EventClass = iota
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
type Coalescer struct {
	window time.Duration
	onDone func(CoalescedEvent)
}

// NewCoalescer creates a coalescer with the given window and callback.
func NewCoalescer(window time.Duration, onDone func(CoalescedEvent)) *Coalescer {
	return &Coalescer{window: window, onDone: onDone}
}

// Push adds an event to the coalescer.
func (c *Coalescer) Push(eventType, content string, class EventClass) {
	// Stub: immediately flush for now.
	if c.onDone != nil {
		c.onDone(CoalescedEvent{Type: eventType, Content: content})
	}
}

// PushToolUse records a tool-use event.
func (c *Coalescer) PushToolUse(callID, name string, input any) {
	// Stub: no-op.
}

// PushToolResult records a tool-result event.
func (c *Coalescer) PushToolResult(callID, name, content string) {
	// Stub: no-op.
}

// Close flushes any remaining events and stops the coalescer.
func (c *Coalescer) Close() {
	// Stub: no-op.
}

// Summarizer produces a summary from accumulated messages.
type Summarizer struct{}

func (s *Summarizer) Summarize(messages []string) (string, error) {
	return "", nil
}
