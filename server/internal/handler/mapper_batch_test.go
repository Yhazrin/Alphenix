package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/daemon"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// userToResponse
// ---------------------------------------------------------------------------

func TestUserToResponse_Basic(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	u := db.User{
		ID:    testUUID("11111111-1111-1111-1111-111111111111"),
		Name:  "Alice",
		Email: "alice@example.com",
		AvatarUrl: pgtype.Text{String: "https://img.example.com/alice.png", Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
	resp := userToResponse(u)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Name != "Alice" {
		t.Errorf("Name = %q", resp.Name)
	}
	if resp.Email != "alice@example.com" {
		t.Errorf("Email = %q", resp.Email)
	}
	if resp.AvatarURL == nil || *resp.AvatarURL != "https://img.example.com/alice.png" {
		t.Errorf("AvatarURL = %v", resp.AvatarURL)
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

func TestUserToResponse_NilAvatar(t *testing.T) {
	u := db.User{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		Name:      "Bob",
		Email:     "bob@example.com",
		AvatarUrl: pgtype.Text{Valid: false},
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := userToResponse(u)
	if resp.AvatarURL != nil {
		t.Errorf("AvatarURL should be nil, got %q", *resp.AvatarURL)
	}
}

// ---------------------------------------------------------------------------
// defaultWorkspaceName
// ---------------------------------------------------------------------------

func TestDefaultWorkspaceName_FromName(t *testing.T) {
	u := db.User{Name: "Alice", Email: "alice@example.com"}
	got := defaultWorkspaceName(u)
	if got != "Alice's Workspace" {
		t.Errorf("got %q, want %q", got, "Alice's Workspace")
	}
}

func TestDefaultWorkspaceName_FromEmail(t *testing.T) {
	u := db.User{Name: "", Email: "bob@example.com"}
	got := defaultWorkspaceName(u)
	if got != "bob's Workspace" {
		t.Errorf("got %q, want %q", got, "bob's Workspace")
	}
}

func TestDefaultWorkspaceName_Fallback(t *testing.T) {
	u := db.User{Name: "", Email: ""}
	got := defaultWorkspaceName(u)
	if got != "Personal's Workspace" {
		t.Errorf("got %q, want %q", got, "Personal's Workspace")
	}
}

func TestDefaultWorkspaceName_TrimsName(t *testing.T) {
	u := db.User{Name: "  Carol  ", Email: "c@example.com"}
	got := defaultWorkspaceName(u)
	if got != "Carol's Workspace" {
		t.Errorf("got %q, want %q", got, "Carol's Workspace")
	}
}

// ---------------------------------------------------------------------------
// defaultWorkspaceSlug — additional cases beyond auth_pure_test.go
// ---------------------------------------------------------------------------

func TestDefaultWorkspaceSlug_ShortUUID(t *testing.T) {
	// UUID with less than 8 chars should not append suffix
	u := db.User{
		ID:   pgtype.UUID{Bytes: [16]byte{0xAA, 0xBB}, Valid: true},
		Name: "Zoe",
	}
	got := defaultWorkspaceSlug(u)
	// uuidToString produces "aabb0000-0000-0000-0000-000000000000", first 8 = "aabb0000"
	// This is > 8 chars, so suffix should be present
	if got == "" {
		t.Error("slug should not be empty")
	}
}

// ---------------------------------------------------------------------------
// agentToResponse
// ---------------------------------------------------------------------------

func TestAgentToResponse_Basic(t *testing.T) {
	a := db.Agent{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		RuntimeID:   testUUID("33333333-3333-3333-3333-333333333333"),
		Name:        "TestAgent",
		Description: "An agent",
		Instructions: "Do things",
		RuntimeMode: "run",
		Visibility: "workspace",
		Status: "active",
		MaxConcurrentTasks: 3,
		OwnerID:    testUUID("44444444-4444-4444-4444-444444444444"),
		CreatedAt:  pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	resp := agentToResponse(a)

	if resp.Name != "TestAgent" {
		t.Errorf("Name = %q", resp.Name)
	}
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.MaxConcurrentTasks != 3 {
		t.Errorf("MaxConcurrentTasks = %d", resp.MaxConcurrentTasks)
	}
	// nil JSON fields → empty defaults
	if resp.RuntimeConfig == nil {
		t.Error("RuntimeConfig should not be nil")
	}
}

func TestAgentToResponse_ValidJSON(t *testing.T) {
	rc := []byte(`{"model":"claude-sonnet-4-5"}`)

	a := db.Agent{
		ID:            testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:   testUUID("22222222-2222-2222-2222-222222222222"),
		RuntimeID:     testUUID("33333333-3333-3333-3333-333333333333"),
		Name:          "JSONAgent",
		RuntimeMode:   "run",
		Visibility:    "workspace",
		Status:        "active",
		RuntimeConfig: rc,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := agentToResponse(a)

	// RuntimeConfig should unmarshal to a map
	rcMap, ok := resp.RuntimeConfig.(map[string]any)
	if !ok {
		t.Fatalf("RuntimeConfig type = %T", resp.RuntimeConfig)
	}
	if rcMap["model"] != "claude-sonnet-4-5" {
		t.Errorf("RuntimeConfig model = %v", rcMap["model"])
	}
}

func TestAgentToResponse_NilOwnerID(t *testing.T) {
	a := db.Agent{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		RuntimeID:   testUUID("33333333-3333-3333-3333-333333333333"),
		Name:        "NoOwner",
		OwnerID:     pgtype.UUID{Valid: false},
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := agentToResponse(a)
	if resp.OwnerID != nil {
		t.Errorf("OwnerID should be nil, got %q", *resp.OwnerID)
	}
}

// ---------------------------------------------------------------------------
// taskToResponse
// ---------------------------------------------------------------------------

func TestTaskToResponse_Basic(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	tq := db.AgentTaskQueue{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		AgentID:   testUUID("22222222-2222-2222-2222-222222222222"),
		RuntimeID: testUUID("33333333-3333-3333-3333-333333333333"),
		IssueID:   testUUID("44444444-4444-4444-4444-444444444444"),
		Status:    "completed",
		Priority:  5,
		Result:    []byte(`{"output":"done"}`),
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
	resp := taskToResponse(tq)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Status != "completed" {
		t.Errorf("Status = %q", resp.Status)
	}
	if resp.Priority != 5 {
		t.Errorf("Priority = %d", resp.Priority)
	}
	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("Result type = %T", resp.Result)
	}
	if m["output"] != "done" {
		t.Errorf("Result output = %v", m["output"])
	}
}

func TestTaskToResponse_NilResult(t *testing.T) {
	tq := db.AgentTaskQueue{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		AgentID:   testUUID("22222222-2222-2222-2222-222222222222"),
		RuntimeID: testUUID("33333333-3333-3333-3333-333333333333"),
		IssueID:   testUUID("44444444-4444-4444-4444-444444444444"),
		Status:    "pending",
		Result:    nil,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := taskToResponse(tq)
	if resp.Result != nil {
		t.Errorf("Result should be nil, got %v", resp.Result)
	}
}

func TestTaskToResponse_NilError(t *testing.T) {
	tq := db.AgentTaskQueue{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		AgentID:   testUUID("22222222-2222-2222-2222-222222222222"),
		RuntimeID: testUUID("33333333-3333-3333-3333-333333333333"),
		IssueID:   testUUID("44444444-4444-4444-4444-444444444444"),
		Status:    "completed",
		Error:     pgtype.Text{Valid: false},
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := taskToResponse(tq)
	if resp.Error != nil {
		t.Errorf("Error should be nil, got %q", *resp.Error)
	}
}

func TestTaskToResponse_WithError(t *testing.T) {
	tq := db.AgentTaskQueue{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		AgentID:   testUUID("22222222-2222-2222-2222-222222222222"),
		RuntimeID: testUUID("33333333-3333-3333-3333-333333333333"),
		IssueID:   testUUID("44444444-4444-4444-4444-444444444444"),
		Status:    "failed",
		Error:     pgtype.Text{String: "timeout exceeded", Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := taskToResponse(tq)
	if resp.Error == nil || *resp.Error != "timeout exceeded" {
		t.Errorf("Error = %v", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// reactionToResponse
// ---------------------------------------------------------------------------

func TestReactionToResponse_Basic(t *testing.T) {
	r := db.CommentReaction{
		ID:        testUUID("11111111-1111-1111-1111-111111111111"),
		CommentID: testUUID("22222222-2222-2222-2222-222222222222"),
		ActorType: "user",
		ActorID:   testUUID("33333333-3333-3333-3333-333333333333"),
		Emoji:     "+1",
		CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	resp := reactionToResponse(r)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.CommentID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("CommentID = %q", resp.CommentID)
	}
	if resp.ActorType != "user" {
		t.Errorf("ActorType = %q", resp.ActorType)
	}
	if resp.Emoji != "+1" {
		t.Errorf("Emoji = %q", resp.Emoji)
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}


// ---------------------------------------------------------------------------
// inboxToResponse
// ---------------------------------------------------------------------------

func TestInboxToResponse_Basic(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	item := db.InboxItem{
		ID:            testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID:   testUUID("22222222-2222-2222-2222-222222222222"),
		RecipientType: "agent",
		RecipientID:   testUUID("33333333-3333-3333-3333-333333333333"),
		Type:          "assignment",
		Severity:      "info",
		Title:         "New task assigned",
		Body:          pgtype.Text{String: "You have a new task", Valid: true},
		Read:          false,
		Archived:      false,
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		ActorType:     pgtype.Text{String: "user", Valid: true},
		ActorID:       testUUID("44444444-4444-4444-4444-444444444444"),
	}
	resp := inboxToResponse(item)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Type != "assignment" {
		t.Errorf("Type = %q", resp.Type)
	}
	if resp.Title != "New task assigned" {
		t.Errorf("Title = %q", resp.Title)
	}
	if resp.Body == nil || *resp.Body != "You have a new task" {
		t.Errorf("Body = %v", resp.Body)
	}
	if resp.ActorType == nil || *resp.ActorType != "user" {
		t.Errorf("ActorType = %v", resp.ActorType)
	}
	if resp.Read || resp.Archived {
		t.Error("Read and Archived should be false")
	}
}

func TestInboxToResponse_NilBody(t *testing.T) {
	item := db.InboxItem{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		Body:        pgtype.Text{Valid: false},
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := inboxToResponse(item)
	if resp.Body != nil {
		t.Errorf("Body should be nil, got %q", *resp.Body)
	}
}

func TestInboxToResponse_NilIssueID(t *testing.T) {
	item := db.InboxItem{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		IssueID:     pgtype.UUID{Valid: false},
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := inboxToResponse(item)
	if resp.IssueID != nil {
		t.Errorf("IssueID should be nil, got %q", *resp.IssueID)
	}
}

// ---------------------------------------------------------------------------
// webhookToResponse
// ---------------------------------------------------------------------------

func TestWebhookToResponse_Basic(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
	w := db.Webhook{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		Url:         "https://example.com/hook",
		EventTypes:  []string{"task.created", "task.updated"},
		IsActive:    true,
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}
	resp := webhookToResponse(w)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.URL != "https://example.com/hook" {
		t.Errorf("URL = %q", resp.URL)
	}
	if len(resp.EventTypes) != 2 {
		t.Errorf("EventTypes length = %d, want 2", len(resp.EventTypes))
	}
	if !resp.IsActive {
		t.Error("IsActive should be true")
	}
	if resp.CreatedAt != "2026-03-15T10:30:00Z" {
		t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, "2026-03-15T10:30:00Z")
	}
}

func TestWebhookToResponse_Inactive(t *testing.T) {
	w := db.Webhook{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		Url:         "https://example.com/hook",
		IsActive:    false,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	resp := webhookToResponse(w)
	if resp.IsActive {
		t.Error("IsActive should be false")
	}
}

// ---------------------------------------------------------------------------
// extractSections
// ---------------------------------------------------------------------------

func TestExtractSections_Basic(t *testing.T) {
	reg := daemon.NewPromptRegistry()
	reg.Register(daemon.PromptSection{
		Name:  "system",
		Phase: daemon.PhaseStatic,
		Order: 10,
		Compute: func() string {
			return "System prompt content"
		},
	})
	reg.Register(daemon.PromptSection{
		Name:  "skills",
		Phase: daemon.PhaseDynamic,
		Order: 20,
		Compute: func() string {
			return "Skills content"
		},
	})

	sections := extractSections(reg)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0].Name != "system" {
		t.Errorf("sections[0].Name = %q, want %q", sections[0].Name, "system")
	}
	if sections[0].Phase != "static" {
		t.Errorf("sections[0].Phase = %q, want %q", sections[0].Phase, "static")
	}
	if sections[0].Content != "System prompt content" {
		t.Errorf("sections[0].Content = %q", sections[0].Content)
	}
	if sections[0].Order != 10 {
		t.Errorf("sections[0].Order = %d, want 10", sections[0].Order)
	}
	if sections[1].Name != "skills" {
		t.Errorf("sections[1].Name = %q, want %q", sections[1].Name, "skills")
	}
	if sections[1].Phase != "dynamic" {
		t.Errorf("sections[1].Phase = %q, want %q", sections[1].Phase, "dynamic")
	}
	if sections[1].Order != 20 {
		t.Errorf("sections[1].Order = %d, want 20", sections[1].Order)
	}
}

func TestExtractSections_Empty(t *testing.T) {
	reg := daemon.NewPromptRegistry()
	sections := extractSections(reg)
	if len(sections) != 0 {
		t.Errorf("expected 0 sections from empty registry, got %d", len(sections))
	}
}

func TestExtractSections_PreservesOrder(t *testing.T) {
	reg := daemon.NewPromptRegistry()
	reg.Register(daemon.PromptSection{
		Name:    "third",
		Phase:   daemon.PhaseStatic,
		Order:   30,
		Compute: func() string { return "C" },
	})
	reg.Register(daemon.PromptSection{
		Name:    "first",
		Phase:   daemon.PhaseStatic,
		Order:   10,
		Compute: func() string { return "A" },
	})
	reg.Register(daemon.PromptSection{
		Name:    "second",
		Phase:   daemon.PhaseDynamic,
		Order:   20,
		Compute: func() string { return "B" },
	})

	sections := extractSections(reg)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	// Order should be 10, 20, 30
	for i, want := range []int{10, 20, 30} {
		if sections[i].Order != want {
			t.Errorf("sections[%d].Order = %d, want %d", i, sections[i].Order, want)
		}
	}
}

func TestExtractSections_NilCompute(t *testing.T) {
	reg := daemon.NewPromptRegistry()
	reg.Register(daemon.PromptSection{
		Name:    "nil-compute",
		Phase:   daemon.PhaseStatic,
		Order:   1,
		Compute: nil,
	})

	sections := extractSections(reg)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Content != "" {
		t.Errorf("Content should be empty for nil Compute, got %q", sections[0].Content)
	}
}
