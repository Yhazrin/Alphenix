package handler

import (
	"testing"
	"time"
)

func TestPingStore_Create(t *testing.T) {
	store := NewPingStore()
	ping := store.Create("runtime-1")
	if ping.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if ping.RuntimeID != "runtime-1" {
		t.Fatalf("expected runtime_id 'runtime-1', got %q", ping.RuntimeID)
	}
	if ping.Status != PingPending {
		t.Fatalf("expected status 'pending', got %q", ping.Status)
	}
	if ping.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}

func TestPingStore_CreateDifferentIDs(t *testing.T) {
	store := NewPingStore()
	p1 := store.Create("rt-1")
	p2 := store.Create("rt-1")
	if p1.ID == p2.ID {
		t.Fatal("expected different IDs for two pings")
	}
}

func TestPingStore_Get(t *testing.T) {
	store := NewPingStore()
	ping := store.Create("rt-1")

	got := store.Get(ping.ID)
	if got == nil {
		t.Fatal("expected to find ping")
	}
	if got.ID != ping.ID {
		t.Fatalf("expected ID %q, got %q", ping.ID, got.ID)
	}
}

func TestPingStore_GetNotFound(t *testing.T) {
	store := NewPingStore()
	got := store.Get("nonexistent")
	if got != nil {
		t.Fatal("expected nil for nonexistent ping")
	}
}

func TestPingStore_GetTimeout(t *testing.T) {
	store := NewPingStore()
	ping := store.Create("rt-1")
	// Artificially age the ping.
	ping.CreatedAt = time.Now().Add(-61 * time.Second)

	got := store.Get(ping.ID)
	if got.Status != PingTimeout {
		t.Fatalf("expected status 'timeout', got %q", got.Status)
	}
	if got.Error == "" {
		t.Fatal("expected timeout error message")
	}
}

func TestPingStore_GetNoTimeoutWhenCompleted(t *testing.T) {
	store := NewPingStore()
	ping := store.Create("rt-1")
	store.Complete(ping.ID, "ok", 100)

	// Age the ping past timeout threshold.
	ping.CreatedAt = time.Now().Add(-61 * time.Second)

	got := store.Get(ping.ID)
	if got.Status != PingCompleted {
		t.Fatalf("expected status 'completed', got %q", got.Status)
	}
}

func TestPingStore_PopPending(t *testing.T) {
	store := NewPingStore()
	store.Create("rt-1")

	popped := store.PopPending("rt-1")
	if popped == nil {
		t.Fatal("expected to pop pending ping")
	}
	if popped.Status != PingRunning {
		t.Fatalf("expected status 'running', got %q", popped.Status)
	}
}

func TestPingStore_PopPendingEmpty(t *testing.T) {
	store := NewPingStore()
	popped := store.PopPending("rt-1")
	if popped != nil {
		t.Fatal("expected nil when no pending pings")
	}
}

func TestPingStore_PopPendingWrongRuntime(t *testing.T) {
	store := NewPingStore()
	store.Create("rt-1")
	popped := store.PopPending("rt-2")
	if popped != nil {
		t.Fatal("expected nil for wrong runtime ID")
	}
}

func TestPingStore_PopPendingOldest(t *testing.T) {
	store := NewPingStore()
	p1 := store.Create("rt-1")
	time.Sleep(time.Millisecond)
	p2 := store.Create("rt-1")

	// Pop should return the oldest (p1).
	popped := store.PopPending("rt-1")
	if popped.ID != p1.ID {
		t.Fatalf("expected oldest ping %q, got %q", p1.ID, popped.ID)
	}

	// Second pop returns p2.
	popped2 := store.PopPending("rt-1")
	if popped2.ID != p2.ID {
		t.Fatalf("expected second oldest ping %q, got %q", p2.ID, popped2.ID)
	}
}

func TestPingStore_PopPendingDoesNotPopRunning(t *testing.T) {
	store := NewPingStore()
	store.Create("rt-1")
	store.PopPending("rt-1") // moves to running

	popped := store.PopPending("rt-1")
	if popped != nil {
		t.Fatal("should not pop running pings")
	}
}

func TestPingStore_Complete(t *testing.T) {
	store := NewPingStore()
	ping := store.Create("rt-1")
	store.Complete(ping.ID, "pong response", 42)

	got := store.Get(ping.ID)
	if got.Status != PingCompleted {
		t.Fatalf("expected status 'completed', got %q", got.Status)
	}
	if got.Output != "pong response" {
		t.Fatalf("expected output 'pong response', got %q", got.Output)
	}
	if got.DurationMs != 42 {
		t.Fatalf("expected duration_ms 42, got %d", got.DurationMs)
	}
}

func TestPingStore_CompleteUnknownID(t *testing.T) {
	store := NewPingStore()
	// Should not panic.
	store.Complete("nonexistent", "output", 10)
}

func TestPingStore_Fail(t *testing.T) {
	store := NewPingStore()
	ping := store.Create("rt-1")
	store.Fail(ping.ID, "connection refused", 100)

	got := store.Get(ping.ID)
	if got.Status != PingFailed {
		t.Fatalf("expected status 'failed', got %q", got.Status)
	}
	if got.Error != "connection refused" {
		t.Fatalf("expected error 'connection refused', got %q", got.Error)
	}
	if got.DurationMs != 100 {
		t.Fatalf("expected duration_ms 100, got %d", got.DurationMs)
	}
}

func TestPingStore_FailUnknownID(t *testing.T) {
	store := NewPingStore()
	// Should not panic.
	store.Fail("nonexistent", "error", 10)
}

func TestPingStore_CleanupOldPings(t *testing.T) {
	store := NewPingStore()
	old := store.Create("rt-1")
	old.CreatedAt = time.Now().Add(-3 * time.Minute)

	// Creating a new ping triggers cleanup.
	store.Create("rt-1")

	got := store.Get(old.ID)
	if got != nil {
		t.Fatal("expected old ping to be cleaned up")
	}
}

func TestPingStatusConstants(t *testing.T) {
	// Verify the string values are stable.
	tests := []struct {
		status PingStatus
		want   string
	}{
		{PingPending, "pending"},
		{PingRunning, "running"},
		{PingCompleted, "completed"},
		{PingFailed, "failed"},
		{PingTimeout, "timeout"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("PingStatus %v = %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}
