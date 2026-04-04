package daemon

import (
	"strings"
	"testing"
)

func TestAssembleSystemPrompt(t *testing.T) {
	prompt := AssembleSystemPrompt(SystemPromptConfig{
		AgentName:         "test-agent",
		AgentInstructions: "Focus on backend code.",
	})

	if !strings.Contains(prompt, "test-agent") {
		t.Error("prompt should contain agent name")
	}
	if !strings.Contains(prompt, "Identity") {
		t.Error("prompt should contain Identity section")
	}
	if !strings.Contains(prompt, "Core Rules") {
		t.Error("prompt should contain Core Rules section")
	}
	if !strings.Contains(prompt, "Execution Protocol") {
		t.Error("prompt should contain Execution Protocol section")
	}
	if !strings.Contains(prompt, "Focus on backend code.") {
		t.Error("prompt should contain agent instructions")
	}
	if !strings.Contains(prompt, "Research") {
		t.Error("prompt should mention Research phase")
	}
	if !strings.Contains(prompt, "PROGRESS:") {
		t.Error("prompt should include PROGRESS pattern")
	}
	if !strings.Contains(prompt, "DONE:") {
		t.Error("prompt should include DONE pattern")
	}
}

func TestAssembleSystemPromptDefaultName(t *testing.T) {
	prompt := AssembleSystemPrompt(SystemPromptConfig{})
	if !strings.Contains(prompt, "agent") {
		t.Error("default agent name should be 'agent'")
	}
}

func TestDefaultToolPermissions(t *testing.T) {
	t.Run("executor gets nil", func(t *testing.T) {
		perms := DefaultToolPermissions("executor")
		if perms != nil {
			t.Error("executor should have nil permissions (all allowed)")
		}
	})

	t.Run("coordinator cannot edit", func(t *testing.T) {
		perms := DefaultToolPermissions("coordinator")
		if perms.IsToolAllowed("Edit") {
			t.Error("coordinator should not be allowed to Edit")
		}
		if perms.IsToolAllowed("Write") {
			t.Error("coordinator should not be allowed to Write")
		}
		if !perms.IsToolAllowed("Read") {
			t.Error("coordinator should be allowed to Read")
		}
		if !perms.IsToolAllowed("Bash") {
			t.Error("coordinator should be allowed to Bash")
		}
	})

	t.Run("reviewer is read-only", func(t *testing.T) {
		perms := DefaultToolPermissions("reviewer")
		if perms.IsToolAllowed("Edit") {
			t.Error("reviewer should not be allowed to Edit")
		}
		if perms.IsToolAllowed("Bash") {
			t.Error("reviewer should not be allowed to Bash")
		}
		if !perms.IsToolAllowed("Read") {
			t.Error("reviewer should be allowed to Read")
		}
		if !perms.IsToolAllowed("Grep") {
			t.Error("reviewer should be allowed to Grep")
		}
	})

	t.Run("unknown role gets nil", func(t *testing.T) {
		perms := DefaultToolPermissions("unknown")
		if perms != nil {
			t.Error("unknown role should have nil permissions (all allowed)")
		}
	})
}
