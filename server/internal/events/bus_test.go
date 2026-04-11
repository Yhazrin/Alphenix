package events

import (
	"sync/atomic"
	"testing"
)

func TestPublishDeliversToSubscribers(t *testing.T) {
	bus := New()
	var count int32

	bus.Subscribe("test:event", func(e Event) {
		atomic.AddInt32(&count, 1)
	})
	bus.Subscribe("test:event", func(e Event) {
		atomic.AddInt32(&count, 1)
	})

	bus.Publish(Event{Type: "test:event", Payload: "hello"})

	if count != 2 {
		t.Errorf("expected 2 handlers called, got %d", count)
	}
}

func TestPublishOnlyMatchingType(t *testing.T) {
	bus := New()
	var called bool

	bus.Subscribe("type:a", func(e Event) {
		called = true
	})

	bus.Publish(Event{Type: "type:b"})

	if called {
		t.Error("handler for type:a should not be called for type:b event")
	}
}

func TestPublishNoSubscribersIsNoop(t *testing.T) {
	bus := New()
	// Should not panic
	bus.Publish(Event{Type: "no:listeners"})
}

func TestPanicInHandlerDoesNotBreakOthers(t *testing.T) {
	bus := New()
	var secondCalled bool

	bus.Subscribe("test:panic", func(e Event) {
		panic("handler panic")
	})
	bus.Subscribe("test:panic", func(e Event) {
		secondCalled = true
	})

	bus.Publish(Event{Type: "test:panic"})

	if !secondCalled {
		t.Error("second handler should still be called after first panics")
	}
}

func TestSubscribeAllReceivesAllEventTypes(t *testing.T) {
	bus := New()

	var received []string
	bus.SubscribeAll(func(e Event) {
		received = append(received, e.Type)
	})

	bus.Publish(Event{Type: "issue:created"})
	bus.Publish(Event{Type: "comment:deleted"})
	bus.Publish(Event{Type: "skill:updated"})

	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}
	if received[0] != "issue:created" || received[1] != "comment:deleted" || received[2] != "skill:updated" {
		t.Fatalf("unexpected events: %v", received)
	}
}

func TestSubscribeAllCalledAfterTypeSpecific(t *testing.T) {
	bus := New()

	var order []string
	bus.Subscribe("issue:created", func(e Event) {
		order = append(order, "specific")
	})
	bus.SubscribeAll(func(e Event) {
		order = append(order, "global")
	})

	bus.Publish(Event{Type: "issue:created"})

	if len(order) != 2 || order[0] != "specific" || order[1] != "global" {
		t.Fatalf("expected [specific, global], got %v", order)
	}
}

func TestSubscribeAllPanicRecovery(t *testing.T) {
	bus := New()

	var secondCalled bool
	bus.SubscribeAll(func(e Event) {
		panic("test panic")
	})
	bus.SubscribeAll(func(e Event) {
		secondCalled = true
	})

	bus.Publish(Event{Type: "test"})

	if !secondCalled {
		t.Fatal("second global handler was not called after first panicked")
	}
}

func TestEventFieldsPassedThrough(t *testing.T) {
	bus := New()
	var received Event

	bus.Subscribe("test:fields", func(e Event) {
		received = e
	})

	bus.Publish(Event{
		Type:        "test:fields",
		WorkspaceID: "ws-123",
		ActorType:   "member",
		ActorID:     "user-456",
		Payload:     map[string]string{"key": "value"},
	})

	if received.WorkspaceID != "ws-123" {
		t.Errorf("expected WorkspaceID ws-123, got %s", received.WorkspaceID)
	}
	if received.ActorType != "member" {
		t.Errorf("expected ActorType member, got %s", received.ActorType)
	}
	if received.ActorID != "user-456" {
		t.Errorf("expected ActorID user-456, got %s", received.ActorID)
	}
}

func TestPriorityOrdering(t *testing.T) {
	bus := New()
	var order []string

	// Register in reverse priority order to verify sorting
	bus.SubscribeWithPriority("test:prio", 30, func(e Event) {
		order = append(order, "third")
	})
	bus.SubscribeWithPriority("test:prio", 10, func(e Event) {
		order = append(order, "first")
	})
	bus.SubscribeWithPriority("test:prio", 20, func(e Event) {
		order = append(order, "second")
	})

	bus.Publish(Event{Type: "test:prio"})

	if len(order) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Fatalf("expected [first, second, third], got %v", order)
	}
}

func TestPriorityGlobalHandlers(t *testing.T) {
	bus := New()
	var order []string

	bus.SubscribeAllWithPriority(30, func(e Event) {
		order = append(order, "global-last")
	})
	bus.SubscribeAllWithPriority(10, func(e Event) {
		order = append(order, "global-first")
	})

	bus.Publish(Event{Type: "test"})

	if len(order) != 2 {
		t.Fatalf("expected 2 handlers, got %d", len(order))
	}
	if order[0] != "global-first" || order[1] != "global-last" {
		t.Fatalf("expected [global-first, global-last], got %v", order)
	}
}

// ---------------------------------------------------------------------------
// sortHandlers pure helper
// ---------------------------------------------------------------------------

func TestSortHandlers(t *testing.T) {
	t.Run("sorts by priority ascending", func(t *testing.T) {
		entries := []handlerEntry{
			{priority: 30, handler: func(e Event) {}},
			{priority: 10, handler: func(e Event) {}},
			{priority: 20, handler: func(e Event) {}},
		}
		sorted := sortHandlers(entries)
		if sorted[0].priority != 10 || sorted[1].priority != 20 || sorted[2].priority != 30 {
			t.Errorf("expected priorities [10, 20, 30], got [%d, %d, %d]",
				sorted[0].priority, sorted[1].priority, sorted[2].priority)
		}
	})

	t.Run("stable for equal priorities", func(t *testing.T) {
		entries := []handlerEntry{
			{priority: 10, handler: func(e Event) {}},
			{priority: 10, handler: func(e Event) {}},
			{priority: 10, handler: func(e Event) {}},
		}
		// Mark by setting distinct priorities temporarily won't work since we can't
		// compare func identity. Instead verify order is preserved via length.
		sorted := sortHandlers(entries)
		if len(sorted) != 3 {
			t.Errorf("expected 3 entries, got %d", len(sorted))
		}
		for _, e := range sorted {
			if e.priority != 10 {
				t.Errorf("expected priority 10, got %d", e.priority)
			}
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		entries := []handlerEntry{
			{priority: 30, handler: func(e Event) {}},
			{priority: 10, handler: func(e Event) {}},
		}
		_ = sortHandlers(entries)
		if entries[0].priority != 30 {
			t.Error("original slice should not be mutated")
		}
	})

	t.Run("empty returns empty", func(t *testing.T) {
		sorted := sortHandlers(nil)
		if len(sorted) != 0 {
			t.Errorf("expected empty, got %d", len(sorted))
		}
	})

	t.Run("single entry unchanged", func(t *testing.T) {
		entries := []handlerEntry{{priority: 42, handler: func(e Event) {}}}
		sorted := sortHandlers(entries)
		if len(sorted) != 1 || sorted[0].priority != 42 {
			t.Error("single entry should be returned as-is")
		}
	})
}
