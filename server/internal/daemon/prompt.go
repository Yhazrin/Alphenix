package daemon

import (
	"fmt"
	"strings"

	"github.com/multica-ai/multicode/server/pkg/protocol"
)

// BuildPrompt constructs the task prompt for an agent CLI.
//
// The prompt is split into two layers:
//  1. System prompt (via PromptRegistry) — static rules + dynamic context
//  2. Task instruction — what to do right now
//
// When SharedContext is present, collaborative awareness is injected as a
// registered dynamic section in the prompt registry.
func BuildPrompt(task Task) string {
	agentName := "agent"
	var instructions string
	if task.Agent != nil {
		agentName = task.Agent.Name
		instructions = task.Agent.Instructions
	}

	cfg := SystemPromptConfig{
		AgentName:         agentName,
		AgentInstructions: instructions,
	}

	// Build registry with default sections + collaboration context.
	registry := NewPromptRegistry()
	registerDefaultSections(registry, cfg)

	if task.SharedContext != nil {
		registerCollaborationSection(registry, task.SharedContext)
	}

	// Register the task instruction as the final dynamic section.
	registry.Register(PromptSection{
		Name:  "task-instruction",
		Phase: PhaseDynamic,
		Order: 100,
		Compute: func() string {
			var b strings.Builder
			b.WriteString("---\n")
			b.WriteString("## Current Task\n\n")
			fmt.Fprintf(&b, "Your assigned issue ID is: **%s**\n\n", task.IssueID)
			fmt.Fprintf(&b, "Start by running `multicode issue get %s --output json` to understand your task, then follow the Execution Protocol above.\n", task.IssueID)
			return b.String()
		},
	})

	return AssembleWithRegistry(registry, cfg)
}

// registerCollaborationSection adds the collaboration context as a dynamic section.
func registerCollaborationSection(registry *PromptRegistry, sc *protocol.SharedContext) {
	registry.Register(PromptSection{
		Name:  "collaboration-context",
		Phase: PhaseDynamic,
		Order: 50,
		Compute: func() string {
			return formatCollaborationContext(sc)
		},
	})
}

// formatCollaborationContext renders the shared multi-agent context as a string.
func formatCollaborationContext(sc *protocol.SharedContext) string {
	hasContent := len(sc.Colleagues) > 0 || len(sc.PendingMessages) > 0 ||
		len(sc.Dependencies) > 0 || len(sc.WorkspaceMemory) > 0 ||
		sc.LastCheckpoint != nil
	if !hasContent {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Collaboration Context\n\n")

	formatColleagues(&b, sc)
	formatPendingMessages(&b, sc)
	formatDependencies(&b, sc)
	formatWorkspaceMemory(&b, sc)
	formatLastCheckpoint(&b, sc)

	return b.String()
}

// appendCollaborationContext writes the shared multi-agent context into the prompt.
// Deprecated: Use registerCollaborationSection for registry-based assembly.
func appendCollaborationContext(b *strings.Builder, sc *protocol.SharedContext) {
	content := formatCollaborationContext(sc)
	if content == "" {
		return
	}
	b.WriteString("\n---\n## Collaboration Context\n\n")
	b.WriteString(content)
}

func formatColleagues(b *strings.Builder, sc *protocol.SharedContext) {
	if len(sc.Colleagues) == 0 {
		return
	}
	b.WriteString("### Workspace Agents\n")
	b.WriteString("Other agents you can collaborate with (send messages, chain tasks, mention in issues):\n\n")
	for _, c := range sc.Colleagues {
		status := c.Status
		if status == "" {
			status = "unknown"
		}
		fmt.Fprintf(b, "- **%s** (ID: %s, status: %s)", c.Name, c.ID, status)
		if c.Description != "" {
			fmt.Fprintf(b, " — %s", c.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nUse `multicode agent message --to <agent_id> --message \"...\"` to send a message to a colleague.\n")
	b.WriteString("Use `multicode task chain <task_id> --target <agent_id> --reason \"...\"` to delegate follow-up work.\n\n")
}

func formatPendingMessages(b *strings.Builder, sc *protocol.SharedContext) {
	if len(sc.PendingMessages) == 0 {
		return
	}
	fmt.Fprintf(b, "### Pending Messages (%d unread)\n", len(sc.PendingMessages))
	b.WriteString("Messages from other agents waiting for your attention:\n\n")
	for _, m := range sc.PendingMessages {
		fmt.Fprintf(b, "- From agent %s", m.FromAgentID)
		if m.MessageType != "" {
			fmt.Fprintf(b, " [%s]", m.MessageType)
		}
		fmt.Fprintf(b, ": %s\n", m.Content)
	}
	b.WriteString("\nConsider these messages when deciding your approach.\n\n")
}

func formatDependencies(b *strings.Builder, sc *protocol.SharedContext) {
	if len(sc.Dependencies) == 0 {
		return
	}
	b.WriteString("### Task Dependencies\n")
	b.WriteString("Your task depends on other tasks that must complete first:\n\n")
	for _, d := range sc.Dependencies {
		statusIcon := "⏳"
		if d.DependencyStatus == "completed" {
			statusIcon = "✅"
		} else if d.DependencyStatus == "failed" {
			statusIcon = "❌"
		}
		fmt.Fprintf(b, "- %s Task %s depends on %s (status: %s)\n", statusIcon, d.TaskID, d.DependsOnID, d.DependencyStatus)
	}
	b.WriteString("\nIf any dependency has not completed, consider waiting or checking its status before proceeding.\n\n")
}

func formatWorkspaceMemory(b *strings.Builder, sc *protocol.SharedContext) {
	if len(sc.WorkspaceMemory) == 0 {
		return
	}
	b.WriteString("### Relevant Memories\n")
	b.WriteString("Past observations and patterns from your workspace that may help:\n\n")
	for _, m := range sc.WorkspaceMemory {
		fmt.Fprintf(b, "- [%.0f%% similar", m.Similarity*100)
		if m.AgentName != "" {
			fmt.Fprintf(b, ", by %s", m.AgentName)
		}
		fmt.Fprintf(b, "]: %s\n", m.Content)
	}
	b.WriteString("\n")
}

func formatLastCheckpoint(b *strings.Builder, sc *protocol.SharedContext) {
	if sc.LastCheckpoint == nil {
		return
	}
	b.WriteString("### Last Checkpoint\n")
	fmt.Fprintf(b, "A previous execution checkpoint exists: **%s** (saved %s)\n", sc.LastCheckpoint.Label, sc.LastCheckpoint.CreatedAt)
	if sc.LastCheckpoint.FilesChanged != nil {
		b.WriteString("Files changed at checkpoint: ")
		fmt.Fprintf(b, "%v\n", sc.LastCheckpoint.FilesChanged)
	}
	b.WriteString("\nYou can resume from this checkpoint state if appropriate.\n\n")
}
