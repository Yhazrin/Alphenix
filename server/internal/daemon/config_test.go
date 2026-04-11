package daemon

import (
	"strings"
	"testing"

	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

func TestNormalizeServerBaseURL_WS(t *testing.T) {
	got, err := NormalizeServerBaseURL("ws://localhost:8080/ws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Errorf("got %q, want %q", got, "http://localhost:8080")
	}
}

func TestNormalizeServerBaseURL_WSS(t *testing.T) {
	got, err := NormalizeServerBaseURL("wss://api.example.com/ws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://api.example.com" {
		t.Errorf("got %q, want %q", got, "https://api.example.com")
	}
}

func TestNormalizeServerBaseURL_HTTP(t *testing.T) {
	got, err := NormalizeServerBaseURL("http://localhost:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Errorf("got %q, want %q", got, "http://localhost:8080")
	}
}

func TestNormalizeServerBaseURL_HTTPS(t *testing.T) {
	got, err := NormalizeServerBaseURL("https://api.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://api.example.com" {
		t.Errorf("got %q, want %q", got, "https://api.example.com")
	}
}

func TestNormalizeServerBaseURL_TrailingSlash(t *testing.T) {
	got, err := NormalizeServerBaseURL("http://localhost:8080/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Errorf("got %q, want %q", got, "http://localhost:8080")
	}
}

func TestNormalizeServerBaseURL_InvalidScheme(t *testing.T) {
	_, err := NormalizeServerBaseURL("ftp://localhost:8080")
	if err == nil {
		t.Fatal("expected error for invalid scheme")
	}
}

func TestNormalizeServerBaseURL_InvalidURL(t *testing.T) {
	_, err := NormalizeServerBaseURL("://broken")
	if err == nil {
		t.Fatal("expected error for malformed URL")
	}
}

func TestBuildPrompt_Basic(t *testing.T) {
	task := Task{
		IssueID: "issue-123",
		Agent: &AgentData{
			Name:         "test-agent",
			Instructions: "Be helpful",
		},
		WorkspaceName: "my-workspace",
	}
	prompt := BuildPrompt(task)
	if !strings.Contains(prompt, "issue-123") {
		t.Error("expected prompt to contain issue ID")
	}
	if !strings.Contains(prompt, "test-agent") {
		t.Error("expected prompt to contain agent name")
	}
	if !strings.Contains(prompt, "my-workspace") {
		t.Error("expected prompt to contain workspace name")
	}
}

func TestBuildPrompt_NilAgent(t *testing.T) {
	task := Task{
		IssueID: "issue-456",
		Agent:   nil,
	}
	prompt := BuildPrompt(task)
	if !strings.Contains(prompt, "issue-456") {
		t.Error("expected prompt to contain issue ID even with nil agent")
	}
	if !strings.Contains(prompt, "agent") {
		t.Error("expected default agent name 'agent'")
	}
}

func TestBuildPrompt_WithSharedContext(t *testing.T) {
	sc := &protocol.SharedContext{
		Colleagues: []protocol.ColleagueInfo{
			{ID: "agent-2", Name: "colleague", Status: "idle"},
		},
	}
	task := Task{
		IssueID:       "issue-789",
		Agent:         &AgentData{Name: "agent-1"},
		SharedContext: sc,
	}
	prompt := BuildPrompt(task)
	if !strings.Contains(prompt, "Collaboration Context") {
		t.Error("expected collaboration context in prompt")
	}
	if !strings.Contains(prompt, "colleague") {
		t.Error("expected colleague name in prompt")
	}
}

func TestBuildPrompt_WithDependencies(t *testing.T) {
	sc := &protocol.SharedContext{
		Dependencies: []protocol.TaskDependencyInfo{
			{TaskID: "task-1", DependsOnID: "task-0", DependencyStatus: "completed"},
		},
	}
	task := Task{
		IssueID:       "issue-dep",
		Agent:         &AgentData{Name: "agent-1"},
		SharedContext: sc,
	}
	prompt := BuildPrompt(task)
	if !strings.Contains(prompt, "Task Dependencies") {
		t.Error("expected dependencies section in prompt")
	}
}

func TestFormatCollaborationContext_Empty(t *testing.T) {
	sc := &protocol.SharedContext{}
	result := formatCollaborationContext(sc)
	if result != "" {
		t.Errorf("expected empty string for empty context, got %q", result)
	}
}

func TestFormatCollaborationContext_WithPendingMessages(t *testing.T) {
	sc := &protocol.SharedContext{
		PendingMessages: []protocol.AgentMessagePayload{
			{FromAgentID: "agent-2", MessageType: "review", Content: "LGTM"},
		},
	}
	result := formatCollaborationContext(sc)
	if !strings.Contains(result, "Pending Messages") {
		t.Error("expected pending messages section")
	}
	if !strings.Contains(result, "LGTM") {
		t.Error("expected message content")
	}
}

func TestFormatColleagues(t *testing.T) {
	sc := &protocol.SharedContext{
		Colleagues: []protocol.ColleagueInfo{
			{ID: "a1", Name: "Alice", Status: "busy", Description: "frontend dev"},
		},
	}
	var b strings.Builder
	formatColleagues(&b, sc)
	out := b.String()
	if !strings.Contains(out, "Alice") {
		t.Error("expected colleague name")
	}
	if !strings.Contains(out, "busy") {
		t.Error("expected colleague status")
	}
	if !strings.Contains(out, "frontend dev") {
		t.Error("expected colleague description")
	}
}

func TestFormatDependencies_StatusIcons(t *testing.T) {
	sc := &protocol.SharedContext{
		Dependencies: []protocol.TaskDependencyInfo{
			{TaskID: "t1", DependsOnID: "t0", DependencyStatus: "completed"},
			{TaskID: "t2", DependsOnID: "t0", DependencyStatus: "failed"},
			{TaskID: "t3", DependsOnID: "t0", DependencyStatus: "running"},
		},
	}
	var b strings.Builder
	formatDependencies(&b, sc)
	out := b.String()
	if !strings.Contains(out, "✅") {
		t.Error("expected checkmark for completed dependency")
	}
	if !strings.Contains(out, "❌") {
		t.Error("expected cross for failed dependency")
	}
}

func TestFormatWorkspaceMemory(t *testing.T) {
	sc := &protocol.SharedContext{
		WorkspaceMemory: []protocol.MemoryRecall{
			{Content: "remember this", Similarity: 0.95, AgentName: "agent-1"},
		},
	}
	var b strings.Builder
	formatWorkspaceMemory(&b, sc)
	out := b.String()
	if !strings.Contains(out, "remember this") {
		t.Error("expected memory content")
	}
	if !strings.Contains(out, "95%") {
		t.Error("expected similarity percentage")
	}
}

func TestFormatLastCheckpoint(t *testing.T) {
	sc := &protocol.SharedContext{
		LastCheckpoint: &protocol.CheckpointInfo{
			Label:        "initial-setup",
			CreatedAt:    "2026-04-07T10:00:00Z",
			FilesChanged: []string{"main.go", "README.md"},
		},
	}
	var b strings.Builder
	formatLastCheckpoint(&b, sc)
	out := b.String()
	if !strings.Contains(out, "initial-setup") {
		t.Error("expected checkpoint label")
	}
	if !strings.Contains(out, "main.go") {
		t.Error("expected changed file")
	}
}

func TestFormatLastCheckpoint_Nil(t *testing.T) {
	sc := &protocol.SharedContext{LastCheckpoint: nil}
	var b strings.Builder
	formatLastCheckpoint(&b, sc)
	if b.String() != "" {
		t.Error("expected empty output for nil checkpoint")
	}
}
