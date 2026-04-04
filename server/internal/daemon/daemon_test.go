package daemon

import (
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multicode/server/pkg/protocol"
)

func TestNormalizeServerBaseURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("ws://localhost:8080/ws")
	if err != nil {
		t.Fatalf("NormalizeServerBaseURL returned error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Fatalf("expected http://localhost:8080, got %s", got)
	}
}

func TestBuildPromptContainsIssueID(t *testing.T) {
	t.Parallel()

	issueID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	prompt := BuildPrompt(Task{
		IssueID: issueID,
		Agent: &AgentData{
			Name: "Local Codex",
			Skills: []SkillData{
				{Name: "Concise", Content: "Be concise."},
			},
		},
	})

	// Prompt should contain the issue ID and CLI hint.
	for _, want := range []string{
		issueID,
		"multicode issue get",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}

	// Skills should NOT be inlined in the prompt (they're in runtime config).
	for _, absent := range []string{"## Agent Skills", "Be concise."} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q (skills are in runtime config)", absent)
		}
	}
}

func TestBuildPromptNoIssueDetails(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "test-id",
		Agent:   &AgentData{Name: "Test"},
	})

	// Prompt should not contain issue title/description (agent fetches via CLI).
	for _, absent := range []string{"**Issue:**", "**Summary:**"} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q — agent fetches details via CLI", absent)
		}
	}
}

func TestIsWorkspaceNotFoundError(t *testing.T) {
	t.Parallel()

	err := &requestError{
		Method:     http.MethodPost,
		Path:       "/api/daemon/register",
		StatusCode: http.StatusNotFound,
		Body:       `{"error":"workspace not found"}`,
	}
	if !isWorkspaceNotFoundError(err) {
		t.Fatal("expected workspace not found error to be recognized")
	}

	if isWorkspaceNotFoundError(&requestError{StatusCode: http.StatusInternalServerError, Body: `{"error":"workspace not found"}`}) {
		t.Fatal("did not expect 500 to be treated as workspace not found")
	}
}

func TestBuildPromptWithSharedContext(t *testing.T) {
	t.Parallel()

	sc := &protocol.SharedContext{
		Colleagues: []protocol.ColleagueInfo{
			{ID: "agent-1", Name: "Backend Dev", Description: "Handles API code", Status: "working"},
			{ID: "agent-2", Name: "Frontend Dev", Description: "Handles UI code", Status: "idle"},
		},
		PendingMessages: []protocol.AgentMessagePayload{
			{MessageID: "msg-1", FromAgentID: "agent-1", Content: "API is ready for integration", MessageType: "info"},
		},
		Dependencies: []protocol.TaskDependencyInfo{
			{TaskID: "task-2", DependsOnID: "task-1", DependencyStatus: "completed"},
			{TaskID: "task-3", DependsOnID: "task-2", DependencyStatus: "in_progress"},
		},
		WorkspaceMemory: []protocol.MemoryRecall{
			{ID: "mem-1", Content: "Always use snake_case for DB columns", Similarity: 0.92, AgentName: "Backend Dev"},
		},
		LastCheckpoint: &protocol.CheckpointInfo{
			ID:        "cp-1",
			Label:     "initial-scaffold",
			CreatedAt: "2026-04-04T10:00:00Z",
		},
	}

	prompt := BuildPrompt(Task{
		IssueID:       "issue-123",
		SharedContext: sc,
	})

	// Should contain the issue ID.
	if !strings.Contains(prompt, "issue-123") {
		t.Fatal("prompt missing issue ID")
	}

	// Should contain collaboration context header.
	if !strings.Contains(prompt, "Collaboration Context") {
		t.Fatal("prompt missing collaboration context header")
	}

	// Should list colleagues.
	if !strings.Contains(prompt, "Backend Dev") {
		t.Fatal("prompt missing colleague Backend Dev")
	}
	if !strings.Contains(prompt, "Frontend Dev") {
		t.Fatal("prompt missing colleague Frontend Dev")
	}

	// Should contain pending messages.
	if !strings.Contains(prompt, "Pending Messages") {
		t.Fatal("prompt missing pending messages section")
	}
	if !strings.Contains(prompt, "API is ready for integration") {
		t.Fatal("prompt missing message content")
	}

	// Should contain dependencies.
	if !strings.Contains(prompt, "Task Dependencies") {
		t.Fatal("prompt missing dependencies section")
	}

	// Should contain workspace memory.
	if !strings.Contains(prompt, "Relevant Memories") {
		t.Fatal("prompt missing workspace memory section")
	}
	if !strings.Contains(prompt, "snake_case") {
		t.Fatal("prompt missing memory content")
	}

	// Should contain checkpoint.
	if !strings.Contains(prompt, "Last Checkpoint") {
		t.Fatal("prompt missing checkpoint section")
	}
	if !strings.Contains(prompt, "initial-scaffold") {
		t.Fatal("prompt missing checkpoint label")
	}
}

func TestBuildPromptWithoutSharedContext(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "issue-456",
	})

	// Should still contain the issue ID.
	if !strings.Contains(prompt, "issue-456") {
		t.Fatal("prompt missing issue ID")
	}

	// Should NOT contain collaboration context header.
	if strings.Contains(prompt, "Collaboration Context") {
		t.Fatal("prompt should not have collaboration context when SharedContext is nil")
	}
}

func TestBuildPromptWithEmptySharedContext(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID:       "issue-789",
		SharedContext:  &protocol.SharedContext{},
	})

	// Should NOT contain collaboration context header (empty context has no content).
	if strings.Contains(prompt, "Collaboration Context") {
		t.Fatal("prompt should not have collaboration context when SharedContext is empty")
	}
}
