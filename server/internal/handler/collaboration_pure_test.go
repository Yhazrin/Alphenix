package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/memory"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	pgvector_go "github.com/pgvector/pgvector-go"
)

// --- agentMessageToResponse ---

func TestCollabAgentMessageToResponse_AllFields(t *testing.T) {
	id := testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	ws := testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901")
	from := testUUID("c3d4e5f6-a7b8-9012-cdef-123456789012")
	to := testUUID("d4e5f6a7-b8c9-0123-defa-234567890123")
	task := testUUID("e5f6a7b8-c9d0-1234-efab-345678901234")
	reply := testUUID("f6a7b8c9-d0e1-2345-fabc-456789012345")
	now := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	readAt := testTimestamp(now)

	m := db.AgentMessage{
		ID:          id,
		WorkspaceID: ws,
		FromAgentID: from,
		ToAgentID:   to,
		TaskID:      task,
		Content:     "hello agent",
		MessageType: "info",
		ReplyToID:   reply,
		ReadAt:      readAt,
		CreatedAt:   testTimestamp(now),
	}
	resp := agentMessageToResponse(m)

	if resp.ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.WorkspaceID != "b2c3d4e5-f6a7-8901-bcde-f12345678901" {
		t.Errorf("WorkspaceID = %q", resp.WorkspaceID)
	}
	if resp.FromAgentID != "c3d4e5f6-a7b8-9012-cdef-123456789012" {
		t.Errorf("FromAgentID = %q", resp.FromAgentID)
	}
	if resp.ToAgentID != "d4e5f6a7-b8c9-0123-defa-234567890123" {
		t.Errorf("ToAgentID = %q", resp.ToAgentID)
	}
	if resp.TaskID != "e5f6a7b8-c9d0-1234-efab-345678901234" {
		t.Errorf("TaskID = %q", resp.TaskID)
	}
	if resp.Content != "hello agent" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.MessageType != "info" {
		t.Errorf("MessageType = %q", resp.MessageType)
	}
	if resp.ReplyToID != "f6a7b8c9-d0e1-2345-fabc-456789012345" {
		t.Errorf("ReplyToID = %q", resp.ReplyToID)
	}
	if resp.ReadAt == nil {
		t.Fatal("ReadAt should not be nil")
	}
	if *resp.ReadAt == "" {
		t.Error("ReadAt should not be empty")
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

func TestCollabAgentMessageToResponse_NilUUIDs(t *testing.T) {
	m := db.AgentMessage{}
	resp := agentMessageToResponse(m)

	if resp.ID != "" {
		t.Errorf("ID should be empty, got %q", resp.ID)
	}
	if resp.TaskID != "" {
		t.Errorf("TaskID should be empty, got %q", resp.TaskID)
	}
	if resp.ReplyToID != "" {
		t.Errorf("ReplyToID should be empty, got %q", resp.ReplyToID)
	}
	if resp.ReadAt != nil {
		t.Error("ReadAt should be nil")
	}
}

// --- checkpointToResponse ---

func TestCollabCheckpointToResponse_WithStateAndFiles(t *testing.T) {
	cp := db.TaskCheckpoint{
		ID:           testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		TaskID:       testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901"),
		WorkspaceID:  testUUID("c3d4e5f6-a7b8-9012-cdef-123456789012"),
		Label:        "v1.0",
		State:        []byte(`{"step":3}`),
		FilesChanged: []byte(`["main.go","go.mod"]`),
		CreatedAt:    testTimestamp(time.Now()),
	}
	resp := checkpointToResponse(cp)

	if resp.ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.TaskID != "b2c3d4e5-f6a7-8901-bcde-f12345678901" {
		t.Errorf("TaskID = %q", resp.TaskID)
	}
	if resp.Label != "v1.0" {
		t.Errorf("Label = %q", resp.Label)
	}

	state, ok := resp.State.(map[string]any)
	if !ok {
		t.Fatal("State should be a map")
	}
	if state["step"].(float64) != 3 {
		t.Errorf("State step = %v", state["step"])
	}

	files, ok := resp.FilesChanged.([]any)
	if !ok {
		t.Fatal("FilesChanged should be a slice")
	}
	if len(files) != 2 {
		t.Errorf("FilesChanged len = %d, want 2", len(files))
	}
}

func TestCollabCheckpointToResponse_NilStateAndFiles(t *testing.T) {
	cp := db.TaskCheckpoint{
		ID:          testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		TaskID:      testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901"),
		WorkspaceID: testUUID("c3d4e5f6-a7b8-9012-cdef-123456789012"),
		Label:       "init",
		State:       nil,
		FilesChanged: nil,
		CreatedAt:   testTimestamp(time.Now()),
	}
	resp := checkpointToResponse(cp)

	if resp.Label != "init" {
		t.Errorf("Label = %q", resp.Label)
	}
	// nil JSON bytes → nil result from json.Unmarshal
	if resp.State != nil {
		t.Errorf("State should be nil, got %T", resp.State)
	}
	if resp.FilesChanged != nil {
		t.Errorf("FilesChanged should be nil, got %T", resp.FilesChanged)
	}
}

func TestCollabCheckpointToResponse_InvalidJSON(t *testing.T) {
	cp := db.TaskCheckpoint{
		ID:          testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		TaskID:      testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901"),
		WorkspaceID: testUUID("c3d4e5f6-a7b8-9012-cdef-123456789012"),
		Label:       "bad",
		State:       []byte(`not-json`),
		FilesChanged: []byte(`{broken`),
		CreatedAt:   testTimestamp(time.Now()),
	}
	resp := checkpointToResponse(cp)

	// Should not panic; unmarshal errors are silently ignored
	if resp.Label != "bad" {
		t.Errorf("Label = %q", resp.Label)
	}
}

// --- memoryToResponse ---

func TestCollabMemoryToResponse_WithExpiresAt(t *testing.T) {
	expTime := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	m := db.AgentMemory{
		ID:        testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		AgentID:   testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901"),
		Content:   "remember this",
		Metadata:  []byte(`{"key":"value"}`),
		CreatedAt: testTimestamp(time.Now()),
		ExpiresAt: pgtype.Timestamptz{Time: expTime, Valid: true},
	}
	resp := memoryToResponse(m, 0.95)

	if resp.ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Content != "remember this" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Similarity != 0.95 {
		t.Errorf("Similarity = %f", resp.Similarity)
	}
	if resp.ExpiresAt == nil {
		t.Fatal("ExpiresAt should not be nil")
	}
	if *resp.ExpiresAt != expTime.Format(time.RFC3339) {
		t.Errorf("ExpiresAt = %q, want %q", *resp.ExpiresAt, expTime.Format(time.RFC3339))
	}
	meta, ok := resp.Metadata.(map[string]any)
	if !ok {
		t.Fatal("Metadata should be a map")
	}
	if meta["key"] != "value" {
		t.Errorf("Metadata key = %v", meta["key"])
	}
}

func TestCollabMemoryToResponse_NilExpiresAt(t *testing.T) {
	m := db.AgentMemory{
		ID:        testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		AgentID:   testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901"),
		Content:   "no expiry",
		Metadata:  []byte(`{}`),
		CreatedAt: testTimestamp(time.Now()),
		ExpiresAt: pgtype.Timestamptz{Valid: false},
	}
	resp := memoryToResponse(m, 0.0)

	if resp.ExpiresAt != nil {
		t.Error("ExpiresAt should be nil when Valid=false")
	}
	if resp.Similarity != 0.0 {
		t.Errorf("Similarity = %f", resp.Similarity)
	}
}

func TestCollabMemoryToResponse_NilMetadata(t *testing.T) {
	m := db.AgentMemory{
		ID:        testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		AgentID:   testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901"),
		Content:   "no metadata",
		Metadata:  nil,
		CreatedAt: testTimestamp(time.Now()),
		ExpiresAt: pgtype.Timestamptz{Valid: false},
	}
	resp := memoryToResponse(m, 0.5)

	// nil JSON → nil from json.Unmarshal
	if resp.Metadata != nil {
		t.Errorf("Metadata should be nil, got %T", resp.Metadata)
	}
}

// --- searchResultToResponse ---

func TestCollabSearchResultToResponse_WrapsMemory(t *testing.T) {
	sr := memory.SearchResult{
		Memory: db.AgentMemory{
			ID:        testUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
			AgentID:   testUUID("b2c3d4e5-f6a7-8901-bcde-f12345678901"),
			Content:   "relevant",
			Metadata:  []byte(`{}`),
			CreatedAt: testTimestamp(time.Now()),
			ExpiresAt: pgtype.Timestamptz{Valid: false},
		},
		Score: 0.87,
		Rank:  1,
	}
	resp := searchResultToResponse(sr)

	if resp.ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Similarity != 0.87 {
		t.Errorf("Similarity = %f, want 0.87", resp.Similarity)
	}
	if resp.Content != "relevant" {
		t.Errorf("Content = %q", resp.Content)
	}
}

// --- optionalUUID ---

func TestCollabOptionalUUID_NilPointer(t *testing.T) {
	u := optionalUUID(nil)
	if u.Valid {
		t.Error("nil pointer should produce invalid UUID")
	}
}

func TestCollabOptionalUUID_EmptyString(t *testing.T) {
	s := ""
	u := optionalUUID(&s)
	if u.Valid {
		t.Error("empty string should produce invalid UUID")
	}
}

func TestCollabOptionalUUID_ValidString(t *testing.T) {
	s := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	u := optionalUUID(&s)
	if !u.Valid {
		t.Fatal("valid string should produce valid UUID")
	}
	// Verify round-trip
	got := uuidToString(u)
	if got != s {
		t.Errorf("round-trip = %q, want %q", got, s)
	}
}

func TestCollabOptionalUUID_InvalidString(t *testing.T) {
	s := "not-a-uuid"
	u := optionalUUID(&s)
	// parseUUID returns a UUID even for invalid strings; just verify no panic
	_ = u
}

// --- float64SliceToVector ---

func TestCollabFloat64SliceToVector_Basic(t *testing.T) {
	vals := []float64{1.0, 2.5, 3.75}
	v := float64SliceToVector(vals)

	if len(v.Slice()) != 3 {
		t.Fatalf("vector length = %d, want 3", len(v.Slice()))
	}
	slice := v.Slice()
	if slice[0] != float32(1.0) {
		t.Errorf("v[0] = %f", slice[0])
	}
	if slice[1] != float32(2.5) {
		t.Errorf("v[1] = %f", slice[1])
	}
	if slice[2] != float32(3.75) {
		t.Errorf("v[2] = %f", slice[2])
	}
}

func TestCollabFloat64SliceToVector_EmptySlice(t *testing.T) {
	v := float64SliceToVector([]float64{})
	if len(v.Slice()) != 0 {
		t.Errorf("empty slice should produce empty vector, got len=%d", len(v.Slice()))
	}
}

func TestCollabFloat64SliceToVector_PrecisionLoss(t *testing.T) {
	vals := []float64{1.0000000001}
	v := float64SliceToVector(vals)
	slice := v.Slice()
	// float64→float32 truncates precision
	if slice[0] == float32(1.0000000001) {
		// This is actually expected to happen sometimes due to IEEE 754
		// Just verify it doesn't panic
	}
	if slice[0] != float32(1.0) && slice[0] != float32(1.0000000001) {
		// The truncated value should be close to 1.0
		t.Logf("v[0] = %f (precision loss expected)", slice[0])
	}
}

func TestCollabFloat64SliceToVector_NilSlice(t *testing.T) {
	v := float64SliceToVector(nil)
	if len(v.Slice()) != 0 {
		t.Errorf("nil slice should produce empty vector, got len=%d", len(v.Slice()))
	}
}

// helper: create a pgtype.Timestamptz from time.Time
func testTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// Ensure pgvector_go import is used (via AgentMemory)
var _ = pgvector_go.NewVector
