package service

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// issueToMap
// ---------------------------------------------------------------------------

func TestIssueToMap_AllFieldsPresent(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	id := makeTestUUID("issue-id01")
	wsID := makeTestUUID("ws-000001")
	assigneeID := makeTestUUID("assigne01")
	creatorID := makeTestUUID("creator01")
	parentID := makeTestUUID("parent-01")

	issue := db.Issue{
		ID:            id,
		WorkspaceID:   wsID,
		Title:         "Test issue",
		Description:   pgtype.Text{String: "A description", Valid: true},
		Status:        "open",
		Priority:      "high",
		AssigneeType:  pgtype.Text{String: "agent", Valid: true},
		AssigneeID:    assigneeID,
		CreatorType:   "user",
		CreatorID:     creatorID,
		ParentIssueID: parentID,
		Position:      1.5,
		DueDate:       pgtype.Timestamptz{Time: now, Valid: true},
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		Number:        42,
	}

	m := issueToMap(issue, "PRJ")

	if m["id"] == "" {
		t.Error("id should not be empty")
	}
	if m["workspace_id"] == "" {
		t.Error("workspace_id should not be empty")
	}
	if m["number"] != int32(42) {
		t.Errorf("number = %v, want 42", m["number"])
	}
	if m["identifier"] != "PRJ-42" {
		t.Errorf("identifier = %v, want PRJ-42", m["identifier"])
	}
	if m["title"] != "Test issue" {
		t.Errorf("title = %v, want 'Test issue'", m["title"])
	}
	if isNil(m["description"]) {
		t.Error("description should not be nil for valid text")
	}
	if m["status"] != "open" {
		t.Errorf("status = %v, want 'open'", m["status"])
	}
	if m["priority"] != "high" {
		t.Errorf("priority = %v, want 'high'", m["priority"])
	}
	if isNil(m["assignee_type"]) {
		t.Error("assignee_type should not be nil")
	}
	if isNil(m["assignee_id"]) {
		t.Error("assignee_id should not be nil for valid UUID")
	}
	if m["creator_type"] != "user" {
		t.Errorf("creator_type = %v, want 'user'", m["creator_type"])
	}
	if m["creator_id"] == "" {
		t.Error("creator_id should not be empty")
	}
	if isNil(m["parent_issue_id"]) {
		t.Error("parent_issue_id should not be nil for valid UUID")
	}
	if m["position"] != 1.5 {
		t.Errorf("position = %v, want 1.5", m["position"])
	}
	if isNil(m["due_date"]) {
		t.Error("due_date should not be nil for valid timestamptz")
	}
	if m["created_at"] == "" {
		t.Error("created_at should not be empty")
	}
	if m["updated_at"] == "" {
		t.Error("updated_at should not be empty")
	}
}

func TestIssueToMap_IdentifierFormat(t *testing.T) {
	tests := []struct {
		prefix string
		number int32
		want   string
	}{
		{"PRJ", 1, "PRJ-1"},
		{"ABC", 999, "ABC-999"},
		{"x", 0, "x-0"},
	}
	for _, tt := range tests {
		issue := db.Issue{Number: tt.number}
		m := issueToMap(issue, tt.prefix)
		if m["identifier"] != tt.want {
			t.Errorf("issueToMap(%q, %d) identifier = %v, want %v",
				tt.prefix, tt.number, m["identifier"], tt.want)
		}
	}
}

func TestIssueToMap_NilOptionalFields(t *testing.T) {
	// Zero-value issue: all pgtype fields invalid → pointers should be nil.
	issue := db.Issue{}
	m := issueToMap(issue, "X")

	if !isNil(m["description"]) {
		t.Error("description should be nil for invalid pgtype.Text")
	}
	if !isNil(m["assignee_type"]) {
		t.Error("assignee_type should be nil for invalid pgtype.Text")
	}
	if !isNil(m["assignee_id"]) {
		t.Error("assignee_id should be nil for invalid pgtype.UUID")
	}
	if !isNil(m["parent_issue_id"]) {
		t.Error("parent_issue_id should be nil for invalid pgtype.UUID")
	}
	if !isNil(m["due_date"]) {
		t.Error("due_date should be nil for invalid pgtype.Timestamptz")
	}
	if m["created_at"] != "" {
		t.Errorf("created_at should be empty string for invalid timestamptz, got %v", m["created_at"])
	}
	if m["updated_at"] != "" {
		t.Errorf("updated_at should be empty string for invalid timestamptz, got %v", m["updated_at"])
	}
}

func TestIssueToMap_TimestampFormat(t *testing.T) {
	ts := time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
	issue := db.Issue{
		CreatedAt: pgtype.Timestamptz{Time: ts, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: ts, Valid: true},
		DueDate:   pgtype.Timestamptz{Time: ts, Valid: true},
	}
	m := issueToMap(issue, "T")

	// TimestampToString returns RFC3339.
	if m["created_at"] != "2026-03-15T10:30:00Z" {
		t.Errorf("created_at = %v, want RFC3339", m["created_at"])
	}
	// TimestampToPtr returns *string in RFC3339.
	if ds, ok := m["due_date"].(*string); !ok || *ds != "2026-03-15T10:30:00Z" {
		t.Errorf("due_date = %v, want pointer to RFC3339 string", m["due_date"])
	}
}

func TestIssueToMap_MapHasKeyCount(t *testing.T) {
	issue := db.Issue{}
	m := issueToMap(issue, "X")
	expectedKeys := 17
	if len(m) != expectedKeys {
		t.Errorf("expected %d keys, got %d: %v", expectedKeys, len(m), keysOf(m))
	}
}

// ---------------------------------------------------------------------------
// agentToMap
// ---------------------------------------------------------------------------

func TestAgentToMap_AllFieldsPresent(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	id := makeTestUUID("agent-id01")
	wsID := makeTestUUID("ws-000001")
	runtimeID := makeTestUUID("rt-000001")
	ownerID := makeTestUUID("owner-01")

	agent := db.Agent{
		ID:                 id,
		WorkspaceID:        wsID,
		RuntimeID:          runtimeID,
		Name:               "my-agent",
		Description:        "does things",
		AvatarUrl:          pgtype.Text{String: "https://img/avatar.png", Valid: true},
		RuntimeMode:        "cloud",
		RuntimeConfig:      []byte(`{"region":"us-east"}`),
		Visibility:         "public",
		Status:             "active",
		MaxConcurrentTasks: 5,
		OwnerID:            ownerID,
		CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		ArchivedAt:         pgtype.Timestamptz{},
		ArchivedBy:         pgtype.UUID{},
	}

	m := agentToMap(agent)

	if m["name"] != "my-agent" {
		t.Errorf("name = %v, want 'my-agent'", m["name"])
	}
	if m["description"] != "does things" {
		t.Errorf("description = %v", m["description"])
	}
	if m["runtime_mode"] != "cloud" {
		t.Errorf("runtime_mode = %v, want 'cloud'", m["runtime_mode"])
	}
	if m["visibility"] != "public" {
		t.Errorf("visibility = %v", m["visibility"])
	}
	if m["status"] != "active" {
		t.Errorf("status = %v", m["status"])
	}
	if m["max_concurrent_tasks"] != int32(5) {
		t.Errorf("max_concurrent_tasks = %v, want 5", m["max_concurrent_tasks"])
	}
	if isNil(m["avatar_url"]) {
		t.Error("avatar_url should not be nil for valid Text")
	}
	if isNil(m["owner_id"]) {
		t.Error("owner_id should not be nil for valid UUID")
	}
	if !isNil(m["archived_at"]) {
		t.Error("archived_at should be nil for invalid timestamptz")
	}
	if !isNil(m["archived_by"]) {
		t.Error("archived_by should be nil for invalid UUID")
	}
	if m["created_at"] == "" {
		t.Error("created_at should not be empty")
	}
	if m["skills"] == nil {
		t.Error("skills should always be present (empty slice)")
	}
}

func TestAgentToMap_JSONFieldsUnmarshal(t *testing.T) {
	agent := db.Agent{
		RuntimeConfig: []byte(`{"gpu":true}`),
	}
	m := agentToMap(agent)

	// runtime_config should be a map.
	rc, ok := m["runtime_config"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_config should be map[string]any, got %T", m["runtime_config"])
	}
	if rc["gpu"] != true {
		t.Errorf("runtime_config.gpu = %v, want true", rc["gpu"])
	}

	// tools should be a slice.
	tools, ok := m["tools"].([]any)
	if !ok {
		t.Fatalf("tools should be []any, got %T", m["tools"])
	}
	if len(tools) != 2 {
		t.Errorf("tools length = %d, want 2", len(tools))
	}

	// triggers should be a map.
	trig, ok := m["triggers"].(map[string]any)
	if !ok {
		t.Fatalf("triggers should be map[string]any, got %T", m["triggers"])
	}
	if trig["cron"] != "0 * * * *" {
		t.Errorf("triggers.cron = %v", trig["cron"])
	}
}

func TestAgentToMap_NilJSONFields(t *testing.T) {
	agent := db.Agent{
		RuntimeConfig: nil,
	}
	m := agentToMap(agent)

	// When nil, JSON unmarshal is skipped; fields should be nil.
	if m["runtime_config"] != nil {
		t.Errorf("runtime_config should be nil, got %T: %v", m["runtime_config"], m["runtime_config"])
	}
	if m["tools"] != nil {
		t.Errorf("tools should be nil, got %T: %v", m["tools"], m["tools"])
	}
	if m["triggers"] != nil {
		t.Errorf("triggers should be nil, got %T: %v", m["triggers"], m["triggers"])
	}
}

func TestAgentToMap_EmptyJSONFields(t *testing.T) {
	// Empty JSON objects/arrays should unmarshal without error.
	agent := db.Agent{
		RuntimeConfig: []byte(`{}`),
	}
	m := agentToMap(agent)

	if m["runtime_config"] == nil {
		t.Error("runtime_config should not be nil for empty JSON object")
	}
	if m["tools"] == nil {
		t.Error("tools should not be nil for empty JSON array")
	}
}

func TestAgentToMap_InvalidJSONHandled(t *testing.T) {
	// Invalid JSON should not panic — just logs a warning and leaves nil.
	agent := db.Agent{
		RuntimeConfig: []byte(`not-json{{{`),
	}
	m := agentToMap(agent)
	// Should not panic; fields may be nil or zero.
	_ = m["runtime_config"]
	_ = m["tools"]
	_ = m["triggers"]
}

func TestAgentToMap_MapHasKeyCount(t *testing.T) {
	agent := db.Agent{}
	m := agentToMap(agent)
	expectedKeys := 19
	if len(m) != expectedKeys {
		t.Errorf("expected %d keys, got %d: %v", expectedKeys, len(m), keysOf(m))
	}
}

func TestAgentToMap_NilOptionalFields(t *testing.T) {
	agent := db.Agent{
		AvatarUrl:  pgtype.Text{},
		OwnerID:    pgtype.UUID{},
		ArchivedAt: pgtype.Timestamptz{},
		ArchivedBy: pgtype.UUID{},
	}
	m := agentToMap(agent)

	if !isNil(m["avatar_url"]) {
		t.Error("avatar_url should be nil for invalid Text")
	}
	if !isNil(m["owner_id"]) {
		t.Error("owner_id should be nil for invalid UUID")
	}
	if !isNil(m["archived_at"]) {
		t.Error("archived_at should be nil for invalid Timestamptz")
	}
	if !isNil(m["archived_by"]) {
		t.Error("archived_by should be nil for invalid UUID")
	}
}

func TestAgentToMap_SkillsAlwaysEmptySlice(t *testing.T) {
	agent := db.Agent{}
	m := agentToMap(agent)
	skills, ok := m["skills"].([]any)
	if !ok {
		t.Fatalf("skills should be []any, got %T", m["skills"])
	}
	if len(skills) != 0 {
		t.Errorf("skills should be empty, got %d", len(skills))
	}
}

// ---------------------------------------------------------------------------
// NewTaskService constructor
// ---------------------------------------------------------------------------

func TestNewTaskService_NilArgs(t *testing.T) {
	svc := NewTaskService(nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.Queries != nil {
		t.Error("expected nil Queries")
	}
}

// ---------------------------------------------------------------------------
// AgentSkillData struct
// ---------------------------------------------------------------------------

func TestAgentSkillData_Fields(t *testing.T) {
	skill := AgentSkillData{
		Name:    "code-review",
		Content: "review instructions",
		Files: []AgentSkillFileData{
			{Path: "src/main.py", Content: "print('hello')"},
		},
	}
	if skill.Name != "code-review" {
		t.Errorf("Name = %q, want %q", skill.Name, "code-review")
	}
	if len(skill.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(skill.Files))
	}
	if skill.Files[0].Path != "src/main.py" {
		t.Errorf("File path = %q", skill.Files[0].Path)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// isNil checks whether v is nil or a typed nil pointer stored in an interface.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsNil()
}

// Verify json import is used (needed for agentToMap tests).
var _ = json.Unmarshal
