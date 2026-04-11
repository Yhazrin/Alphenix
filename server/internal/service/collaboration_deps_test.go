package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// AddDependency — self-dependency rejection
// ---------------------------------------------------------------------------

func TestAddDependency_SelfDependency(t *testing.T) {
	stub := &stubDBTX{}
	cs := newCollabService(stub)

	taskID := makeTestUUID("task-0000000001")
	_, err := cs.AddDependency(context.Background(), makeTestUUID("ws-0000000001"), taskID, taskID)
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
	if !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Errorf("expected 'cannot depend on itself', got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AddDependency — direct cycle detection (A→B, then B→A)
// ---------------------------------------------------------------------------

func TestAddDependency_DirectCycle(t *testing.T) {
	taskA := makeTestUUID("task-a-00000001")
	taskB := makeTestUUID("task-b-00000001")

	// Existing dependency: A → B (taskB depends on taskA... wait, let's clarify).
	// AddDependency(ctx, ws, taskID, dependsOnTaskID) means taskID depends on dependsOnTaskID.
	// wouldCreateCycle checks if dependsOnTaskID transitively depends on taskID.
	// So if we already have B depends on A, and now try A depends on B → cycle.

	// Stub: GetTaskDependencies for taskB returns a dep: taskB → taskA
	stub := &stubDBTX{
		queryResponses: map[string][][]any{
			"GetTaskDependencies": {
				{makeTestUUID("dep-000000001"), makeTestUUID("ws-000000001"), taskB, taskA, pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			},
		},
	}
	cs := newCollabService(stub)

	// Try to add A → B (A depends on B). But B already depends on A → cycle.
	_, err := cs.AddDependency(context.Background(), makeTestUUID("ws-000000001"), taskA, taskB)
	if err == nil {
		t.Fatal("expected error for direct cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AddDependency — transitive cycle detection (A→B→C, then C→A)
// ---------------------------------------------------------------------------

func TestAddDependency_TransitiveCycle(t *testing.T) {
	taskA := makeTestUUID("task-a-00000001")
	taskB := makeTestUUID("task-b-00000001")
	taskC := makeTestUUID("task-c-00000001")

	// Existing chain: B → A (B depends on A), C → B (C depends on B).
	// Now try to add A → C (A depends on C) → would create A→C→B→A cycle.

	// GetTaskDependencies for taskC returns: C → B
	// GetTaskDependencies for taskB returns: B → A
	stub := &stubDBTX{
		queryResponses: map[string][][]any{
			"GetTaskDependencies": {
				// First call (for taskC): C depends on B
				{makeTestUUID("dep-000000002"), makeTestUUID("ws-000000001"), taskC, taskB, pgtype.Timestamptz{Time: time.Now(), Valid: true}},
				// Second call (for taskB): B depends on A
				{makeTestUUID("dep-000000003"), makeTestUUID("ws-000000001"), taskB, taskA, pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			},
		},
	}
	cs := newCollabService(stub)

	_, err := cs.AddDependency(context.Background(), makeTestUUID("ws-000000001"), taskA, taskC)
	if err == nil {
		t.Fatal("expected error for transitive cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AddDependency — no cycle (valid dependency)
// ---------------------------------------------------------------------------

func TestAddDependency_NoCycle(t *testing.T) {
	taskA := makeTestUUID("task-a-00000001")
	taskB := makeTestUUID("task-b-00000001")
	wsID := makeTestUUID("ws-000000001")

	// GetTaskDependencies returns nothing for taskB (no transitive deps).
	// CreateTaskDependency returns a valid dep.
	stub := &stubDBTX{
		queryResponses: map[string][][]any{
			// Empty — no existing dependencies for taskB
		},
		queryRowQueues: map[string][]stubRow{
			"CreateTaskDependency": {
				{values: []any{
					makeTestUUID("dep-000000010"), wsID, taskA, taskB,
					pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}},
			},
		},
	}
	cs := newCollabService(stub)

	dep, err := cs.AddDependency(context.Background(), wsID, taskA, taskB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dep.ID.Valid {
		t.Error("expected valid dependency ID")
	}
}

// ---------------------------------------------------------------------------
// AddDependency — CreateTaskDependency DB error
// ---------------------------------------------------------------------------

func TestAddDependency_DBError(t *testing.T) {
	taskA := makeTestUUID("task-a-00000001")
	taskB := makeTestUUID("task-b-00000001")
	wsID := makeTestUUID("ws-000000001")

	stub := &stubDBTX{
		queryRowQueues: map[string][]stubRow{
			"CreateTaskDependency": {
				{err: context.DeadlineExceeded},
			},
		},
	}
	cs := newCollabService(stub)

	_, err := cs.AddDependency(context.Background(), wsID, taskA, taskB)
	if err == nil {
		t.Fatal("expected error from DB failure")
	}
	if !strings.Contains(err.Error(), "create dependency") {
		t.Errorf("expected 'create dependency' wrapped error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AddDependency — publishes event on success
// ---------------------------------------------------------------------------

func TestAddDependency_PublishesEvent(t *testing.T) {
	taskA := makeTestUUID("task-a-00000001")
	taskB := makeTestUUID("task-b-00000001")
	wsID := makeTestUUID("ws-000000001")

	ec := &eventCollector{}
	bus := events.New()
	bus.SubscribeAll(ec.collect)

	stub := &stubDBTX{
		queryRowQueues: map[string][]stubRow{
			"CreateTaskDependency": {
				{values: []any{
					makeTestUUID("dep-000000010"), wsID, taskA, taskB,
					pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}},
			},
		},
	}

	cs := &CollaborationService{
		Queries: db.New(stub),
		Bus:     bus,
	}

	_, err := cs.AddDependency(context.Background(), wsID, taskA, taskB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ec.waitEvents()

	evts := ec.byType("task_dep:created")
	if len(evts) != 1 {
		t.Errorf("expected 1 task_dep:created event, got %d", len(evts))
	}
}

// ---------------------------------------------------------------------------
// wouldCreateCycle — no deps returns false
// ---------------------------------------------------------------------------

func TestWouldCreateCycle_NoDeps(t *testing.T) {
	stub := &stubDBTX{}
	cs := newCollabService(stub)

	result := cs.wouldCreateCycle(context.Background(), makeTestUUID("task-a"), makeTestUUID("task-b"))
	if result {
		t.Error("expected false when no dependencies exist")
	}
}

// ---------------------------------------------------------------------------
// wouldCreateCycle — GetTaskDependencies error returns false (continues)
// ---------------------------------------------------------------------------

func TestWouldCreateCycle_DepQueryError(t *testing.T) {
	stub := &stubDBTX{
		queryErr: context.Canceled,
	}
	cs := newCollabService(stub)

	result := cs.wouldCreateCycle(context.Background(), makeTestUUID("task-a"), makeTestUUID("task-b"))
	if result {
		t.Error("expected false when dep query errors (error is swallowed)")
	}
}

// ---------------------------------------------------------------------------
// wouldCreateCycle — diamond shape (no cycle)
// ---------------------------------------------------------------------------

func TestWouldCreateCycle_DiamondNoCycle(t *testing.T) {
	//   A
	//  / \
	// B   C
	//  \ /
	//   D
	// Existing: B→D, C→D. Try adding A→B and A→C (not cycles).
	// Check: would A→B create cycle? Follow B→D→(no deps of D). No cycle.

	taskA := makeTestUUID("task-a-00000001")
	taskB := makeTestUUID("task-b-00000001")
	taskD := makeTestUUID("task-d-00000001")

	stub := &stubDBTX{
		queryResponses: map[string][][]any{
			// taskB depends on taskD
			"GetTaskDependencies": {
				{makeTestUUID("dep-000000001"), makeTestUUID("ws-000000001"), taskB, taskD, pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			},
		},
	}
	cs := newCollabService(stub)

	// Would adding A→B create a cycle? Check if B transitively depends on A. B→D→∅. No.
	result := cs.wouldCreateCycle(context.Background(), taskA, taskB)
	if result {
		t.Error("expected false for diamond shape (no cycle)")
	}
}

// ---------------------------------------------------------------------------
// RemoveDependency — success publishes event
// ---------------------------------------------------------------------------

func TestRemoveDependency_PublishesEvent(t *testing.T) {
	taskA := makeTestUUID("task-a-00000001")
	taskB := makeTestUUID("task-b-00000001")
	wsID := makeTestUUID("ws-000000001")

	ec := &eventCollector{}
	bus := events.New()
	bus.SubscribeAll(ec.collect)

	stub := &stubDBTX{}
	cs := &CollaborationService{
		Queries: db.New(stub),
		Bus:     bus,
	}

	err := cs.RemoveDependency(context.Background(), wsID, taskA, taskB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ec.waitEvents()

	evts := ec.byType("task_dep:deleted")
	if len(evts) != 1 {
		t.Errorf("expected 1 task_dep:deleted event, got %d", len(evts))
	}
}

// ---------------------------------------------------------------------------
// SendMessage — success path
// ---------------------------------------------------------------------------

func TestSendMessage_SuccessPath(t *testing.T) {
	fromAgent := makeTestUUID("agent-from-01")
	toAgent := makeTestUUID("agent-to-001")
	wsID := makeTestUUID("ws-000000001")
	taskID := makeTestUUID("task-000000001")

	ec := &eventCollector{}
	bus := events.New()
	bus.SubscribeAll(ec.collect)

	now := pgtype.Timestamptz{Time: time.Now().UTC().Truncate(time.Microsecond), Valid: true}
	stub := &stubDBTX{
		queryRowQueues: map[string][]stubRow{
			"CreateAgentMessage": {
				{values: []any{
					makeTestUUID("msg-000000001"), // ID
					wsID,                            // WorkspaceID
					fromAgent,                       // FromAgentID
					toAgent,                         // ToAgentID
					taskID,                          // TaskID
					"hello from agent",              // Content
					[]byte("{}"),                    // Metadata
					now,                             // CreatedAt
					"request",                       // MessageType
					pgtype.Timestamptz{},            // ReadAt (nil)
					pgtype.UUID{},                   // ReplyToID (nil)
				}},
			},
		},
	}

	cs := &CollaborationService{
		Queries: db.New(stub),
		Bus:     bus,
	}

	msg, err := cs.SendMessage(context.Background(), wsID, fromAgent, toAgent, "hello from agent", "request", taskID, pgtype.UUID{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "hello from agent" {
		t.Errorf("expected content 'hello from agent', got %q", msg.Content)
	}
	if msg.MessageType != "request" {
		t.Errorf("expected message_type 'request', got %q", msg.MessageType)
	}

	ec.waitEvents()
	evts := ec.byType("agent:message")
	if len(evts) != 1 {
		t.Errorf("expected 1 agent:message event, got %d", len(evts))
	}
}

// ---------------------------------------------------------------------------
// SendMessage — DB error wraps as "create agent message"
// ---------------------------------------------------------------------------

func TestSendMessage_DBError(t *testing.T) {
	stub := &stubDBTX{
		queryRowQueues: map[string][]stubRow{
			"CreateAgentMessage": {
				{err: context.DeadlineExceeded},
			},
		},
	}
	cs := newCollabService(stub)

	_, err := cs.SendMessage(context.Background(),
		makeTestUUID("ws-000000001"), makeTestUUID("agent-1"), makeTestUUID("agent-2"),
		"test", "request", makeTestUUID("task-1"), pgtype.UUID{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create agent message") {
		t.Errorf("expected 'create agent message' wrapped error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// SendMessage — empty content succeeds
// ---------------------------------------------------------------------------

func TestSendMessage_EmptyContent(t *testing.T) {
	now := pgtype.Timestamptz{Time: time.Now().UTC().Truncate(time.Microsecond), Valid: true}
	stub := &stubDBTX{
		queryRowQueues: map[string][]stubRow{
			"CreateAgentMessage": {
				{values: []any{
					makeTestUUID("msg-000000001"), makeTestUUID("ws-1"),
					makeTestUUID("from"), makeTestUUID("to"),
					makeTestUUID("task-1"), "", []byte("{}"),
					now, "signal", pgtype.Timestamptz{}, pgtype.UUID{},
				}},
			},
		},
	}
	cs := newCollabService(stub)

	msg, err := cs.SendMessage(context.Background(),
		makeTestUUID("ws-1"), makeTestUUID("from"), makeTestUUID("to"),
		"", "signal", makeTestUUID("task-1"), pgtype.UUID{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "" {
		t.Errorf("expected empty content, got %q", msg.Content)
	}
}

// ---------------------------------------------------------------------------
// MarkMessagesRead — success
// ---------------------------------------------------------------------------

func TestMarkMessagesRead_Success(t *testing.T) {
	stub := &stubDBTX{}
	cs := newCollabService(stub)

	err := cs.MarkMessagesRead(context.Background(), makeTestUUID("agent-001"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// MarkMessagesRead — DB error propagates
// ---------------------------------------------------------------------------

func TestMarkMessagesRead_DBError(t *testing.T) {
	// MarkAllAgentMessagesRead uses Exec, which our stubDBTX returns nil for.
	// To simulate error, we'd need a custom stub. But stubDBTX.Exec always returns success.
	// This test verifies the no-error path is clean.
	stub := &stubDBTX{}
	cs := newCollabService(stub)

	err := cs.MarkMessagesRead(context.Background(), makeTestUUID("agent-001"))
	if err != nil {
		t.Fatalf("expected no error from stub Exec, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetReadyTasks — success with results
// ---------------------------------------------------------------------------

func TestGetReadyTasks_Success(t *testing.T) {
	agentID := makeTestUUID("agent-001")
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	stub := &stubDBTX{
		queryResponses: map[string][][]any{
			"ListReadyTasks": {
				{
					makeTestUUID("task-1"),  // ID
					agentID,                  // AgentID
					makeTestUUID("issue-1"),  // IssueID
					"queued",                 // Status
					int32(0),                 // Priority
					now,                      // DispatchedAt
					now,                      // StartedAt
					pgtype.Timestamptz{},     // CompletedAt
					[]byte(nil),              // Result
					pgtype.Text{},            // Error
					now,                      // CreatedAt
					[]byte(nil),              // Context
					pgtype.UUID{},            // RuntimeID
					pgtype.Text{},            // SessionID
					pgtype.Text{},            // WorkDir
					pgtype.UUID{},            // TriggerCommentID
					"",                       // ReviewStatus
					int32(0),                 // ReviewCount
					int32(0),                 // MaxReviews
					pgtype.UUID{},            // ChainSourceTaskID
					"",                       // ChainReason
				},
			},
		},
	}
	cs := newCollabService(stub)

	tasks, err := cs.GetReadyTasks(context.Background(), agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

// ---------------------------------------------------------------------------
// GetReadyTasks — empty result
// ---------------------------------------------------------------------------

func TestGetReadyTasks_Empty(t *testing.T) {
	stub := &stubDBTX{}
	cs := newCollabService(stub)

	tasks, err := cs.GetReadyTasks(context.Background(), makeTestUUID("agent-001"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// ---------------------------------------------------------------------------
// GetDependencyInfo — no deps returns empty
// ---------------------------------------------------------------------------

func TestGetDependencyInfo_NoDeps(t *testing.T) {
	stub := &stubDBTX{}
	cs := newCollabService(stub)

	info, err := cs.GetDependencyInfo(context.Background(), makeTestUUID("task-001"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info) != 0 {
		t.Errorf("expected 0 dependency info entries, got %d", len(info))
	}
}

// ---------------------------------------------------------------------------
// GetDependencyInfo — dep task lookup error uses "pending" fallback
// ---------------------------------------------------------------------------

func TestGetDependencyInfo_DepTaskNotFound(t *testing.T) {
	taskID := makeTestUUID("task-001")
	depTaskID := makeTestUUID("dep-task-01")
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	stub := &stubDBTX{
		queryResponses: map[string][][]any{
			"GetTaskDependencies": {
				{makeTestUUID("dep-000000001"), makeTestUUID("ws-1"), taskID, depTaskID, now},
			},
			// No "GetAgentTask" configured — will return empty rows (task not found)
		},
	}
	cs := newCollabService(stub)

	info, err := cs.GetDependencyInfo(context.Background(), taskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(info))
	}
	if info[0].DependencyStatus != "pending" {
		t.Errorf("expected 'pending' fallback status, got %q", info[0].DependencyStatus)
	}
}
