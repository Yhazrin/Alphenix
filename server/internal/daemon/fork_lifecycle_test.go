package daemon

import (
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multicode/server/internal/events"
	"github.com/multica-ai/multicode/server/pkg/agent"
	"github.com/multica-ai/multicode/server/pkg/protocol"
)

// --- Fork Lifecycle Event Broadcasting ---

// ForkLifecycleBroadcaster broadcasts fork lifecycle events to the event bus.
// This is the interface under test — implementation comes after tests pass.
type ForkLifecycleBroadcaster struct {
	bus         *events.Bus
	workspaceID string
	taskID      string
	agentID     string
}

// NewForkLifecycleBroadcaster creates a broadcaster for fork lifecycle events.
func NewForkLifecycleBroadcaster(bus *events.Bus, workspaceID, taskID, agentID string) *ForkLifecycleBroadcaster {
	return &ForkLifecycleBroadcaster{
		bus:         bus,
		workspaceID: workspaceID,
		taskID:      taskID,
		agentID:     agentID,
	}
}

// PublishStarted broadcasts an agent:fork_started event.
func (b *ForkLifecycleBroadcaster) PublishStarted(forkID, prompt string) {
	b.bus.PublishAsync(events.Event{
		Type:        protocol.EventForkStarted,
		WorkspaceID: b.workspaceID,
		ActorType:   "system",
		ActorID:     b.agentID,
		Payload: map[string]any{
			"fork_id":    forkID,
			"task_id":    b.taskID,
			"agent_id":   b.agentID,
			"prompt_len": len(prompt),
		},
	})
}

// PublishCompleted broadcasts an agent:fork_completed event.
func (b *ForkLifecycleBroadcaster) PublishCompleted(forkID string, result agent.ForkResult) {
	b.bus.PublishAsync(events.Event{
		Type:        protocol.EventForkCompleted,
		WorkspaceID: b.workspaceID,
		ActorType:   "system",
		ActorID:     b.agentID,
		Payload: map[string]any{
			"fork_id":     forkID,
			"task_id":     b.taskID,
			"agent_id":    b.agentID,
			"status":      result.Status,
			"duration_ms": result.DurationMs,
		},
	})
}

// PublishFailed broadcasts an agent:fork_failed event.
func (b *ForkLifecycleBroadcaster) PublishFailed(forkID string, errMsg string) {
	b.bus.PublishAsync(events.Event{
		Type:        protocol.EventForkFailed,
		WorkspaceID: b.workspaceID,
		ActorType:   "system",
		ActorID:     b.agentID,
		Payload: map[string]any{
			"fork_id":  forkID,
			"task_id":  b.taskID,
			"agent_id": b.agentID,
			"error":    errMsg,
		},
	})
}

// --- Tests ---

func TestForkLifecycle_StartedEvent(t *testing.T) {
	bus := events.New()

	var mu sync.Mutex
	var received []events.Event

	bus.Subscribe(protocol.EventForkStarted, func(e events.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	b := NewForkLifecycleBroadcaster(bus, "ws-1", "task-1", "agent-1")
	b.PublishStarted("fork-alpha", "edit foo.go and add error handling")

	// PublishAsync is async — wait briefly for delivery.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 started event, got %d", len(received))
	}
	evt := received[0]
	if evt.Type != protocol.EventForkStarted {
		t.Errorf("expected type %s, got %s", protocol.EventForkStarted, evt.Type)
	}
	if evt.WorkspaceID != "ws-1" {
		t.Errorf("expected workspace ws-1, got %s", evt.WorkspaceID)
	}
	if evt.ActorType != "system" {
		t.Errorf("expected actor system, got %s", evt.ActorType)
	}

	payload, ok := evt.Payload.(map[string]any)
	if !ok {
		t.Fatal("payload is not map[string]any")
	}
	if payload["fork_id"] != "fork-alpha" {
		t.Errorf("expected fork_id fork-alpha, got %v", payload["fork_id"])
	}
	if payload["task_id"] != "task-1" {
		t.Errorf("expected task_id task-1, got %v", payload["task_id"])
	}
}

func TestForkLifecycle_CompletedEvent(t *testing.T) {
	bus := events.New()

	var mu sync.Mutex
	var received []events.Event

	bus.Subscribe(protocol.EventForkCompleted, func(e events.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	b := NewForkLifecycleBroadcaster(bus, "ws-2", "task-2", "agent-2")
	b.PublishCompleted("fork-beta", agent.ForkResult{
		Status:     "completed",
		Output:     "done",
		DurationMs: 1500,
	})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 completed event, got %d", len(received))
	}

	payload := received[0].Payload.(map[string]any)
	if payload["fork_id"] != "fork-beta" {
		t.Errorf("expected fork_id fork-beta, got %v", payload["fork_id"])
	}
	if payload["status"] != "completed" {
		t.Errorf("expected status completed, got %v", payload["status"])
	}
	if payload["duration_ms"] != int64(1500) {
		t.Errorf("expected duration_ms 1500, got %v", payload["duration_ms"])
	}
}

func TestForkLifecycle_FailedEvent(t *testing.T) {
	bus := events.New()

	var mu sync.Mutex
	var received []events.Event

	bus.Subscribe(protocol.EventForkFailed, func(e events.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	b := NewForkLifecycleBroadcaster(bus, "ws-3", "task-3", "agent-3")
	b.PublishFailed("fork-gamma", "process exited with code 1")

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 failed event, got %d", len(received))
	}

	payload := received[0].Payload.(map[string]any)
	if payload["fork_id"] != "fork-gamma" {
		t.Errorf("expected fork_id fork-gamma, got %v", payload["fork_id"])
	}
	if payload["error"] != "process exited with code 1" {
		t.Errorf("expected error message, got %v", payload["error"])
	}
}

func TestForkLifecycle_ParallelEvents(t *testing.T) {
	bus := events.New()

	var mu sync.Mutex
	started := 0
	completed := 0
	failed := 0

	bus.Subscribe(protocol.EventForkStarted, func(e events.Event) {
		mu.Lock()
		started++
		mu.Unlock()
	})
	bus.Subscribe(protocol.EventForkCompleted, func(e events.Event) {
		mu.Lock()
		completed++
		mu.Unlock()
	})
	bus.Subscribe(protocol.EventForkFailed, func(e events.Event) {
		mu.Lock()
		failed++
		mu.Unlock()
	})

	b := NewForkLifecycleBroadcaster(bus, "ws-4", "task-4", "agent-4")

	// Simulate 3 parallel forks: 2 complete, 1 fails.
	b.PublishStarted("fork-1", "task 1")
	b.PublishStarted("fork-2", "task 2")
	b.PublishStarted("fork-3", "task 3")

	b.PublishCompleted("fork-1", agent.ForkResult{Status: "completed"})
	b.PublishCompleted("fork-2", agent.ForkResult{Status: "completed"})
	b.PublishFailed("fork-3", "segfault")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if started != 3 {
		t.Errorf("expected 3 started events, got %d", started)
	}
	if completed != 2 {
		t.Errorf("expected 2 completed events, got %d", completed)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed event, got %d", failed)
	}
}

func TestForkLifecycle_NoListeners(t *testing.T) {
	// Publishing with no subscribers should not panic.
	bus := events.New()
	b := NewForkLifecycleBroadcaster(bus, "ws-5", "task-5", "agent-5")

	b.PublishStarted("fork-x", "test")
	b.PublishCompleted("fork-x", agent.ForkResult{Status: "completed"})
	b.PublishFailed("fork-y", "error")

	// Give async handlers time to run (none registered, but recovery should handle).
	time.Sleep(50 * time.Millisecond)

	// Test passes if no panic.
}

func TestForkLifecycle_GlobalSubscriber(t *testing.T) {
	bus := events.New()

	var mu sync.Mutex
	var allEvents []events.Event

	// Global subscriber receives all event types.
	bus.SubscribeAll(func(e events.Event) {
		mu.Lock()
		allEvents = append(allEvents, e)
		mu.Unlock()
	})

	b := NewForkLifecycleBroadcaster(bus, "ws-6", "task-6", "agent-6")
	b.PublishStarted("fork-z", "test")
	b.PublishCompleted("fork-z", agent.ForkResult{Status: "completed"})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(allEvents) != 2 {
		t.Fatalf("expected 2 events via global subscriber, got %d", len(allEvents))
	}

	// PublishAsync uses goroutines so ordering is not guaranteed.
	types := map[string]bool{}
	for _, e := range allEvents {
		types[e.Type] = true
	}
	if !types[protocol.EventForkStarted] {
		t.Error("missing started event in global subscriber")
	}
	if !types[protocol.EventForkCompleted] {
		t.Error("missing completed event in global subscriber")
	}
}
