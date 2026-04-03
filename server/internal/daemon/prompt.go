package daemon

import (
	"fmt"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// BuildPrompt constructs the task prompt for an agent CLI.
// Keep this minimal — detailed instructions live in CLAUDE.md / AGENTS.md
// injected by execenv.InjectRuntimeConfig.
//
// When SharedContext is present, collaborative awareness is injected so the
// agent knows about colleagues, pending messages, task dependencies, relevant
// memories, and the last checkpoint.
func BuildPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	fmt.Fprintf(&b, "Start by running `multica issue get %s --output json` to understand your task, then complete it.\n", task.IssueID)

	if task.SharedContext != nil {
		appendCollaborationContext(&b, task.SharedContext)
	}

	return b.String()
}

// appendCollaborationContext writes the shared multi-agent context into the prompt.
func appendCollaborationContext(b *strings.Builder, sc *protocol.SharedContext) {
	hasContent := len(sc.Colleagues) > 0 || len(sc.PendingMessages) > 0 ||
		len(sc.Dependencies) > 0 || len(sc.WorkspaceMemory) > 0 ||
		sc.LastCheckpoint != nil
	if !hasContent {
		return
	}

	b.WriteString("\n---\n## Collaboration Context\n\n")

	// Colleagues
	if len(sc.Colleagues) > 0 {
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
		b.WriteString("\nUse `multica agent message --to <agent_id> --message \"...\"` to send a message to a colleague.\n")
		b.WriteString("Use `multica task chain <task_id> --target <agent_id> --reason \"...\"` to delegate follow-up work.\n\n")
	}

	// Pending messages
	if len(sc.PendingMessages) > 0 {
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

	// Task dependencies
	if len(sc.Dependencies) > 0 {
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

	// Workspace memory
	if len(sc.WorkspaceMemory) > 0 {
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

	// Last checkpoint
	if sc.LastCheckpoint != nil {
		b.WriteString("### Last Checkpoint\n")
		fmt.Fprintf(b, "A previous execution checkpoint exists: **%s** (saved %s)\n", sc.LastCheckpoint.Label, sc.LastCheckpoint.CreatedAt)
		if sc.LastCheckpoint.FilesChanged != nil {
			b.WriteString("Files changed at checkpoint: ")
			fmt.Fprintf(b, "%v\n", sc.LastCheckpoint.FilesChanged)
		}
		b.WriteString("\nYou can resume from this checkpoint state if appropriate.\n\n")
	}
}
