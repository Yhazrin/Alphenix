package daemon

import (
	"context"
	"log/slog"
	"testing"

	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/pkg/agent"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// ---------------------------------------------------------------------------
// fileModifyingTools — map correctness
// ---------------------------------------------------------------------------

func TestFileModifyingTools_WriteIsModifying(t *testing.T) {
	if !fileModifyingTools["write"] {
		t.Error("write should be file-modifying")
	}
}

func TestFileModifyingTools_EditIsModifying(t *testing.T) {
	if !fileModifyingTools["edit"] {
		t.Error("edit should be file-modifying")
	}
}

func TestFileModifyingTools_BashIsModifying(t *testing.T) {
	if !fileModifyingTools["bash"] {
		t.Error("bash should be file-modifying")
	}
}

func TestFileModifyingTools_NotebookIsModifying(t *testing.T) {
	if !fileModifyingTools["notebook"] {
		t.Error("notebook should be file-modifying")
	}
}

func TestFileModifyingTools_ReadIsNotModifying(t *testing.T) {
	if fileModifyingTools["read"] {
		t.Error("read should NOT be file-modifying")
	}
}

func TestFileModifyingTools_GrepIsNotModifying(t *testing.T) {
	if fileModifyingTools["grep"] {
		t.Error("grep should NOT be file-modifying")
	}
}

func TestFileModifyingTools_WebsearchIsNotModifying(t *testing.T) {
	if fileModifyingTools["websearch"] {
		t.Error("websearch should NOT be file-modifying")
	}
}

func TestFileModifyingTools_UnknownToolNotModifying(t *testing.T) {
	if fileModifyingTools["some_random_tool"] {
		t.Error("unknown tool should NOT be file-modifying")
	}
}

// ---------------------------------------------------------------------------
// HookService — event publishing
// ---------------------------------------------------------------------------

type hookEventCollector struct {
	events []events.Event
}

func (c *hookEventCollector) collect(e events.Event) {
	c.events = append(c.events, e)
}

func newTestHookService() (*HookService, *hookEventCollector, *events.Bus) {
	bus := events.New()
	collector := &hookEventCollector{}
	bus.SubscribeAll(collector.collect)
	hs := NewHookService(bus, slog.Default())
	return hs, collector, bus
}

func TestHookService_BuildToolHooks_PublishesPreToolUse(t *testing.T) {
	hs, collector, _ := newTestHookService()
	hooks := hs.BuildToolHooks("ws-1", "task-1", "agent-1")

	result := hooks.PreToolUse(context.Background(), "bash", map[string]any{"cmd": "ls"})

	if result.Deny {
		t.Error("default should allow, got deny")
	}
	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	ev := collector.events[0]
	if ev.Type != protocol.EventAgentToolUse {
		t.Errorf("event type = %q, want %q", ev.Type, protocol.EventAgentToolUse)
	}
	if ev.WorkspaceID != "ws-1" {
		t.Errorf("workspace = %q, want %q", ev.WorkspaceID, "ws-1")
	}
	if ev.ActorID != "agent-1" {
		t.Errorf("actor = %q, want %q", ev.ActorID, "agent-1")
	}
}

func TestHookService_BuildToolHooks_PublishesPostToolUse(t *testing.T) {
	hs, collector, _ := newTestHookService()
	hooks := hs.BuildToolHooks("ws-1", "task-1", "agent-1")

	hooks.PostToolUse(context.Background(), "write", map[string]any{"path": "x.go"}, "output")

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	ev := collector.events[0]
	if ev.Type != protocol.EventAgentToolResult {
		t.Errorf("event type = %q, want %q", ev.Type, protocol.EventAgentToolResult)
	}
}

func TestHookService_PublishAgentStarted(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishAgentStarted("ws-1", "task-1", "agent-1", "claude")

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	ev := collector.events[0]
	if ev.Type != protocol.EventAgentStarted {
		t.Errorf("type = %q, want %q", ev.Type, protocol.EventAgentStarted)
	}
	payload := ev.Payload.(map[string]any)
	if payload["provider"] != "claude" {
		t.Errorf("provider = %v, want claude", payload["provider"])
	}
}

func TestHookService_PublishAgentCompleted(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishAgentCompleted("ws-1", "task-1", "agent-1", 5000)

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	ev := collector.events[0]
	if ev.Type != protocol.EventAgentCompleted {
		t.Errorf("type = %q, want %q", ev.Type, protocol.EventAgentCompleted)
	}
	payload := ev.Payload.(map[string]any)
	if payload["duration_ms"] != int64(5000) {
		t.Errorf("duration_ms = %v, want 5000", payload["duration_ms"])
	}
}

func TestHookService_PublishAgentFailed(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishAgentFailed("ws-1", "task-1", "agent-1", "timeout")

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	ev := collector.events[0]
	if ev.Type != protocol.EventAgentFailed {
		t.Errorf("type = %q, want %q", ev.Type, protocol.EventAgentFailed)
	}
	payload := ev.Payload.(map[string]any)
	if payload["error"] != "timeout" {
		t.Errorf("error = %v, want timeout", payload["error"])
	}
}

func TestHookService_PublishAgentSessionStart(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishAgentSessionStart("ws-1", "task-1", "agent-1", "sess-123")

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	payload := collector.events[0].Payload.(map[string]any)
	if payload["session_id"] != "sess-123" {
		t.Errorf("session_id = %v, want sess-123", payload["session_id"])
	}
}

func TestHookService_PublishForkStarted(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishForkStarted("ws-1", "task-1", "agent-1", "fork-1")

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	if collector.events[0].Type != protocol.EventForkStarted {
		t.Errorf("type = %q, want %q", collector.events[0].Type, protocol.EventForkStarted)
	}
}

func TestHookService_PublishForkCompleted(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishForkCompleted("ws-1", "task-1", "agent-1", "fork-1", 3000)

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	payload := collector.events[0].Payload.(map[string]any)
	if payload["duration_ms"] != int64(3000) {
		t.Errorf("duration_ms = %v, want 3000", payload["duration_ms"])
	}
}

func TestHookService_PublishForkFailed(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishForkFailed("ws-1", "task-1", "agent-1", "fork-1", "oom")

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	payload := collector.events[0].Payload.(map[string]any)
	if payload["error"] != "oom" {
		t.Errorf("error = %v, want oom", payload["error"])
	}
}

func TestHookService_PublishAgentStop(t *testing.T) {
	hs, collector, _ := newTestHookService()

	hs.PublishAgentStop("ws-1", "task-1", "agent-1", agent.Result{
		Status:     "completed",
		DurationMs: 1000,
		SessionID:  "sess-42",
	})

	if len(collector.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collector.events))
	}
	if collector.events[0].Type != protocol.EventAgentStop {
		t.Errorf("type = %q, want %q", collector.events[0].Type, protocol.EventAgentStop)
	}
	payload := collector.events[0].Payload.(map[string]any)
	if payload["status"] != "completed" {
		t.Errorf("status = %v, want completed", payload["status"])
	}
	if payload["session_id"] != "sess-42" {
		t.Errorf("session_id = %v, want sess-42", payload["session_id"])
	}
}
