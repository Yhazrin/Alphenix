package realtime

import (
	"testing"
	"time"
)

func TestMemoryTicketStore_GenerateAndValidate(t *testing.T) {
	store := &MemoryTicketStore{stopCh: make(chan struct{})}
	defer close(store.stopCh)

	ticket := store.Generate("ws-1", "user-1")
	if ticket == "" {
		t.Fatal("Generate returned empty ticket")
	}
	if len(ticket) < len(ticketPrefix)+1 {
		t.Fatalf("ticket too short: %q", ticket)
	}
	if ticket[:len(ticketPrefix)] != ticketPrefix {
		t.Fatalf("ticket missing prefix: %q", ticket)
	}

	wsID, uid, ok := store.Validate(ticket, "ws-1")
	if !ok {
		t.Fatal("Validate returned false for valid ticket")
	}
	if wsID != "ws-1" {
		t.Errorf("workspaceID = %q, want %q", wsID, "ws-1")
	}
	if uid != "user-1" {
		t.Errorf("userID = %q, want %q", uid, "user-1")
	}
}

func TestMemoryTicketStore_OneTimeUse(t *testing.T) {
	store := &MemoryTicketStore{stopCh: make(chan struct{})}
	defer close(store.stopCh)

	ticket := store.Generate("ws-1", "user-1")

	// First validate should succeed
	_, _, ok := store.Validate(ticket, "ws-1")
	if !ok {
		t.Fatal("first Validate should succeed")
	}

	// Second validate should fail (ticket consumed)
	_, _, ok = store.Validate(ticket, "ws-1")
	if ok {
		t.Fatal("second Validate should fail — one-time use")
	}
}

func TestMemoryTicketStore_WrongWorkspace(t *testing.T) {
	store := &MemoryTicketStore{stopCh: make(chan struct{})}
	defer close(store.stopCh)

	ticket := store.Generate("ws-1", "user-1")

	_, _, ok := store.Validate(ticket, "ws-2")
	if ok {
		t.Fatal("Validate should fail for wrong workspace")
	}
}

func TestMemoryTicketStore_ExpiredTicket(t *testing.T) {
	store := &MemoryTicketStore{stopCh: make(chan struct{})}
	defer close(store.stopCh)

	// Manually store an expired entry
	ticket := "wst_expired"
	store.tickets.Store(ticket, &ticketEntry{
		workspaceID: "ws-1",
		userID:      "user-1",
		expiresAt:   time.Now().Add(-1 * time.Second),
	})

	_, _, ok := store.Validate(ticket, "ws-1")
	if ok {
		t.Fatal("Validate should fail for expired ticket")
	}

	// Expired ticket should be cleaned up
	if _, exists := store.tickets.Load(ticket); exists {
		t.Fatal("expired ticket should be deleted after failed validation")
	}
}

func TestMemoryTicketStore_UnknownTicket(t *testing.T) {
	store := &MemoryTicketStore{stopCh: make(chan struct{})}
	defer close(store.stopCh)

	_, _, ok := store.Validate("wst_nonexistent", "ws-1")
	if ok {
		t.Fatal("Validate should fail for unknown ticket")
	}
}

func TestMemoryTicketStore_Cleanup(t *testing.T) {
	store := &MemoryTicketStore{stopCh: make(chan struct{})}
	defer close(store.stopCh)

	// Add an expired entry
	store.tickets.Store("wst_old", &ticketEntry{
		workspaceID: "ws-1",
		userID:      "user-1",
		expiresAt:   time.Now().Add(-1 * time.Second),
	})

	// Add a valid entry
	store.tickets.Store("wst_fresh", &ticketEntry{
		workspaceID: "ws-1",
		userID:      "user-1",
		expiresAt:   time.Now().Add(60 * time.Second),
	})

	store.cleanup()

	if _, ok := store.tickets.Load("wst_old"); ok {
		t.Error("expired ticket should be removed by cleanup")
	}
	if _, ok := store.tickets.Load("wst_fresh"); !ok {
		t.Error("valid ticket should survive cleanup")
	}
}
