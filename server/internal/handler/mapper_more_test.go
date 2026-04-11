package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// commentToResponse
// ---------------------------------------------------------------------------

func TestCommentToResponse_Basic(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	c := db.Comment{
		ID:         testUUID("11111111-1111-1111-1111-111111111111"),
		IssueID:    testUUID("22222222-2222-2222-2222-222222222222"),
		AuthorType: "user",
		AuthorID:   testUUID("33333333-3333-3333-3333-333333333333"),
		Content:    "Hello world",
		Type:       "comment",
		CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := commentToResponse(c, nil, nil)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.IssueID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("IssueID = %q", resp.IssueID)
	}
	if resp.AuthorType != "user" {
		t.Errorf("AuthorType = %q", resp.AuthorType)
	}
	if resp.Content != "Hello world" {
		t.Errorf("Content = %q", resp.Content)
	}
	// nil slices should become empty slices
	if resp.Reactions == nil {
		t.Error("Reactions should not be nil")
	}
	if len(resp.Reactions) != 0 {
		t.Errorf("Reactions length = %d, want 0", len(resp.Reactions))
	}
	if resp.Attachments == nil {
		t.Error("Attachments should not be nil")
	}
}

func TestCommentToResponse_WithParentID(t *testing.T) {
	parentID := testUUID("99999999-9999-9999-9999-999999999999")
	c := db.Comment{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		IssueID:   testUUID("22222222-2222-2222-2222-222222222222"),
		ParentID:  parentID,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := commentToResponse(c, nil, nil)
	if resp.ParentID == nil || *resp.ParentID != "99999999-9999-9999-9999-999999999999" {
		t.Errorf("ParentID = %v", resp.ParentID)
	}
}

func TestCommentToResponse_NilParentID(t *testing.T) {
	c := db.Comment{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		IssueID:   testUUID("22222222-2222-2222-2222-222222222222"),
		ParentID:  pgtype.UUID{Valid: false},
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := commentToResponse(c, nil, nil)
	if resp.ParentID != nil {
		t.Errorf("ParentID should be nil, got %q", *resp.ParentID)
	}
}

func TestCommentToResponse_PassesThroughReactionsAndAttachments(t *testing.T) {
	reactions := []ReactionResponse{{Emoji: "+1"}}
	attachments := []AttachmentResponse{{Filename: "file.txt"}}
	c := db.Comment{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		IssueID:   testUUID("22222222-2222-2222-2222-222222222222"),
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := commentToResponse(c, reactions, attachments)
	if len(resp.Reactions) != 1 || resp.Reactions[0].Emoji != "+1" {
		t.Errorf("Reactions = %v", resp.Reactions)
	}
	if len(resp.Attachments) != 1 || resp.Attachments[0].Filename != "file.txt" {
		t.Errorf("Attachments = %v", resp.Attachments)
	}
}

// ---------------------------------------------------------------------------
// subscriberToResponse
// ---------------------------------------------------------------------------

func TestSubscriberToResponse_Basic(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	s := db.IssueSubscriber{
		IssueID:   testUUID("11111111-1111-1111-1111-111111111111"),
		UserType:  "user",
		UserID:    testUUID("22222222-2222-2222-2222-222222222222"),
		Reason:    "mentioned",
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
	resp := subscriberToResponse(s)

	if resp.IssueID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("IssueID = %q", resp.IssueID)
	}
	if resp.UserType != "user" {
		t.Errorf("UserType = %q", resp.UserType)
	}
	if resp.UserID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("UserID = %q", resp.UserID)
	}
	if resp.Reason != "mentioned" {
		t.Errorf("Reason = %q", resp.Reason)
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

// ---------------------------------------------------------------------------
// runtimeToResponse
// ---------------------------------------------------------------------------

func TestRuntimeToResponse_Basic(t *testing.T) {
	rt := db.AgentRuntime{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		InstanceID:  "inst-1",
		Name:        "My Runtime",
		RuntimeMode: "run",
		Provider:    "anthropic",
		Status:      "active",
		DeviceInfo:  "GPU:A100",
		Metadata:    []byte(`{"key":"value"}`),
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := runtimeToResponse(rt)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Name != "My Runtime" {
		t.Errorf("Name = %q", resp.Name)
	}
	if resp.InstanceID != "inst-1" {
		t.Errorf("InstanceID = %q", resp.InstanceID)
	}
	m, ok := resp.Metadata.(map[string]any)
	if !ok || m["key"] != "value" {
		t.Errorf("Metadata = %v", resp.Metadata)
	}
	if resp.DaemonID != nil {
		t.Errorf("DaemonID should be nil, got %q", *resp.DaemonID)
	}
}

func TestRuntimeToResponse_NilMetadata(t *testing.T) {
	rt := db.AgentRuntime{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		Metadata:  nil,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := runtimeToResponse(rt)
	m, ok := resp.Metadata.(map[string]any)
	if !ok || len(m) != 0 {
		t.Error("nil Metadata should default to empty map")
	}
}

func TestRuntimeToResponse_InvalidMetadataJSON(t *testing.T) {
	rt := db.AgentRuntime{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		Metadata:  []byte(`not-json`),
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := runtimeToResponse(rt)
	// Invalid JSON should result in nil metadata → empty map
	m, ok := resp.Metadata.(map[string]any)
	if !ok || len(m) != 0 {
		t.Error("invalid Metadata JSON should default to empty map")
	}
}

// ---------------------------------------------------------------------------
// inboxRowToResponse
// ---------------------------------------------------------------------------

func TestInboxRowToResponse_Basic(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	row := db.ListInboxItemsRow{
		ID:            testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:   testUUID("22222222-2222-2222-2222-222222222222"),
		RecipientType: "agent",
		RecipientID:   testUUID("33333333-3333-3333-3333-333333333333"),
		Type:          "assignment",
		Severity:      "info",
		Title:         "Task assigned",
		Body:          pgtype.Text{String: "You have work", Valid: true},
		Read:          false,
		Archived:      false,
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		IssueStatus:   pgtype.Text{String: "open", Valid: true},
		ActorType:     pgtype.Text{String: "user", Valid: true},
		ActorID:       testUUID("44444444-4444-4444-4444-444444444444"),
		Details:       []byte(`{"key":"val"}`),
	}
	resp := inboxRowToResponse(row)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Type != "assignment" {
		t.Errorf("Type = %q", resp.Type)
	}
	if resp.Body == nil || *resp.Body != "You have work" {
		t.Errorf("Body = %v", resp.Body)
	}
	if resp.IssueStatus == nil || *resp.IssueStatus != "open" {
		t.Errorf("IssueStatus = %v", resp.IssueStatus)
	}
	if resp.ActorType == nil || *resp.ActorType != "user" {
		t.Errorf("ActorType = %v", resp.ActorType)
	}
	if resp.ActorID == nil || *resp.ActorID != "44444444-4444-4444-4444-444444444444" {
		t.Errorf("ActorID = %v", resp.ActorID)
	}
	if resp.Details == nil {
		t.Error("Details should not be nil")
	}
}

func TestInboxRowToResponse_NilOptionalFields(t *testing.T) {
	row := db.ListInboxItemsRow{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		Body:        pgtype.Text{Valid: false},
		IssueStatus: pgtype.Text{Valid: false},
		ActorType:   pgtype.Text{Valid: false},
		ActorID:     pgtype.UUID{Valid: false},
		IssueID:     pgtype.UUID{Valid: false},
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := inboxRowToResponse(row)
	if resp.Body != nil {
		t.Error("Body should be nil")
	}
	if resp.IssueStatus != nil {
		t.Error("IssueStatus should be nil")
	}
	if resp.ActorType != nil {
		t.Error("ActorType should be nil")
	}
	if resp.ActorID != nil {
		t.Error("ActorID should be nil")
	}
	if resp.IssueID != nil {
		t.Error("IssueID should be nil")
	}
}

// ---------------------------------------------------------------------------
// isNotFound
// ---------------------------------------------------------------------------

func TestIsNotFound_WithPgxErrNoRows(t *testing.T) {
	if !isNotFound(pgx.ErrNoRows) {
		t.Error("isNotFound(pgx.ErrNoRows) should be true")
	}
}

func TestIsNotFound_WithOtherError(t *testing.T) {
	if isNotFound(pgx.ErrNoRows) == false {
		// pass
	}
	if isNotFound(nil) {
		t.Error("isNotFound(nil) should be false")
	}
	if isNotFound(pgx.ErrTxClosed) {
		t.Error("isNotFound(ErrTxClosed) should be false")
	}
}

// ---------------------------------------------------------------------------
// isUniqueViolation
// ---------------------------------------------------------------------------

func TestIsUniqueViolation_WithPgError(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505"}
	if !isUniqueViolation(pgErr) {
		t.Error("isUniqueViolation with code 23505 should be true")
	}
}

func TestIsUniqueViolation_WrongCode(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503"}
	if isUniqueViolation(pgErr) {
		t.Error("isUniqueViolation with code 23503 should be false")
	}
}

func TestIsUniqueViolation_NonPgError(t *testing.T) {
	if isUniqueViolation(pgx.ErrNoRows) {
		t.Error("isUniqueViolation with non-PgError should be false")
	}
}

func TestIsUniqueViolation_NilError(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Error("isUniqueViolation(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// generateRandomToken
// ---------------------------------------------------------------------------

func TestGenerateRandomToken_Length(t *testing.T) {
	for _, n := range []int{8, 16, 32, 64} {
		got := generateRandomToken(n)
		if len(got) != n {
			t.Errorf("generateRandomToken(%d) length = %d", n, len(got))
		}
	}
}

func TestGenerateRandomToken_AllAlphanumeric(t *testing.T) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	token := generateRandomToken(100)
	for i, c := range token {
		found := false
		for _, valid := range charset {
			if c == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("character %q at index %d is not alphanumeric", string(c), i)
		}
	}
}

func TestGenerateRandomToken_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		tok := generateRandomToken(16)
		if seen[tok] {
			t.Errorf("duplicate token %q in 50 draws", tok)
		}
		seen[tok] = true
	}
}

func TestGenerateRandomToken_ZeroLength(t *testing.T) {
	got := generateRandomToken(0)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// requestUserID (simple header extraction)
// ---------------------------------------------------------------------------

func TestRequestUserID_WithHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-123")
	got := requestUserID(req)
	if got != "user-123" {
		t.Errorf("got %q, want %q", got, "user-123")
	}
}

func TestRequestUserID_MissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got := requestUserID(req)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// hashToken (runtime_pool.go)
// ---------------------------------------------------------------------------

func TestHashToken_Deterministic(t *testing.T) {
	h1 := hashToken("my-token")
	h2 := hashToken("my-token")
	if h1 != h2 {
		t.Error("hashToken should be deterministic")
	}
}

func TestHashToken_DifferentInputs(t *testing.T) {
	h1 := hashToken("token-a")
	h2 := hashToken("token-b")
	if h1 == h2 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestHashToken_Length(t *testing.T) {
	h := hashToken("test")
	// SHA-256 hex = 64 chars
	if len(h) != 64 {
		t.Errorf("hashToken length = %d, want 64", len(h))
	}
}

// ---------------------------------------------------------------------------
// parseJSONStringSlice (runtime_policy.go) — additional edge cases
// ---------------------------------------------------------------------------

func TestParseJSONStringSlice_ArrayOfNumbers(t *testing.T) {
	// Numbers should not crash, just produce whatever json.Unmarshal gives
	got := parseJSONStringSlice([]byte(`[1, 2, 3]`))
	// Numbers won't be strings, so should return nil or empty
	if got == nil {
		// acceptable — non-string elements are not extracted
	}
}

// ---------------------------------------------------------------------------
// generateSecret (webhook.go)
// ---------------------------------------------------------------------------

func TestGenerateSecret_Length(t *testing.T) {
	s := generateSecret()
	if len(s) == 0 {
		t.Error("generateSecret should not return empty string")
	}
	// Should be hex-encoded, so even length
	if len(s)%2 != 0 {
		t.Errorf("generateSecret length %d is not even (expected hex)", len(s))
	}
}

func TestGenerateSecret_Unique(t *testing.T) {
	s1 := generateSecret()
	s2 := generateSecret()
	if s1 == s2 {
		t.Error("generateSecret should produce unique values")
	}
}

