package realtime

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	ticketTTL     = 60 * time.Second
	ticketCleanup = 30 * time.Second
	ticketPrefix  = "wst_"
)

// ticketEntry stores a ticket with its expiration and workspace context.
type ticketEntry struct {
	workspaceID string
	userID      string
	expiresAt   time.Time
}

// TicketStore holds one-time-use short-lived tickets for WebSocket auth.
// It uses an in-memory sync.Map with a background goroutine that cleans up
// expired tickets every 30 seconds. If Redis becomes available, this can be
// replaced with a Redis-backed implementation.
type TicketStore struct {
	tickets sync.Map // map[string]*ticketEntry
	stopCh  chan struct{}
	wg      sync.Once
}

var globalTicketStore *TicketStore

// InitTicketStore initializes the global ticket store and starts the cleanup goroutine.
func InitTicketStore() {
	globalTicketStore = &TicketStore{
		stopCh: make(chan struct{}),
	}
	go globalTicketStore.cleanupLoop()
}

// StopTicketStore stops the cleanup goroutine. For testing/ graceful shutdown.
func StopTicketStore() {
	if globalTicketStore != nil {
		close(globalTicketStore.stopCh)
	}
}

func (ts *TicketStore) cleanupLoop() {
	ticker := time.NewTicker(ticketCleanup)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ts.cleanup()
		case <-ts.stopCh:
			return
		}
	}
}

func (ts *TicketStore) cleanup() {
	now := time.Now()
	ts.tickets.Range(func(key, value any) bool {
		entry := value.(*ticketEntry)
		if now.After(entry.expiresAt) {
			ts.tickets.Delete(key)
		}
		return true
	})
}

// Generate creates a new ticket for the given workspace and user, returns the ticket string.
func (ts *TicketStore) Generate(workspaceID, userID string) string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	ticket := ticketPrefix + hex.EncodeToString(bytes)

	ts.tickets.Store(ticket, &ticketEntry{
		workspaceID: workspaceID,
		userID:      userID,
		expiresAt:   time.Now().Add(ticketTTL),
	})
	return ticket
}

// Validate checks a ticket: returns (workspaceID, userID, true) if valid and not expired,
// or ("", "", false) if missing, expired, or workspace mismatch.
func (ts *TicketStore) Validate(ticket, workspaceID string) (string, string, bool) {
	value, ok := ts.tickets.Load(ticket)
	if !ok {
		return "", "", false
	}
	entry := value.(*ticketEntry)
	if time.Now().After(entry.expiresAt) {
		ts.tickets.Delete(ticket)
		return "", "", false
	}
	if entry.workspaceID != workspaceID {
		return "", "", false
	}
	// One-time use: delete after successful validation
	ts.tickets.Delete(ticket)
	return entry.workspaceID, entry.userID, true
}

// TicketStoreFor returns the global ticket store instance.
func TicketStoreFor() *TicketStore {
	return globalTicketStore
}
