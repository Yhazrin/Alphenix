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

func TestAssembleSystemPromptOverride(t *testing.T) {
	prompt := AssembleSystemPrompt(SystemPromptConfig{
		AgentName:     "test-agent",
		OverridePrompt: "CUSTOM OVERRIDE",
	})
	if prompt != "CUSTOM OVERRIDE" {
		t.Errorf("override should replace entire prompt, got: %s", prompt)
	}
}

func TestAssembleSystemPromptAppend(t *testing.T) {
	prompt := AssembleSystemPrompt(SystemPromptConfig{
		AgentName:    "test-agent",
		AppendPrompt: "EXTRA SECTION",
	})
	if !strings.Contains(prompt, "EXTRA SECTION") {
		t.Error("append prompt should be added at the end")
	}
	// Append should come after static sections.
	idxExtra := strings.Index(prompt, "EXTRA SECTION")
	idxIdentity := strings.Index(prompt, "Identity")
	if idxIdentity > idxExtra {
		t.Error("append should come after identity section")
	}
}

func TestAssembleSystemPromptCustom(t *testing.T) {
	prompt := AssembleSystemPrompt(SystemPromptConfig{
		AgentName:   "test-agent",
		CustomPrompt: "My custom system rules.",
	})
	if !strings.Contains(prompt, "My custom system rules.") {
		t.Error("custom prompt should be included")
	}
	// Static sections should still be present.
	if !strings.Contains(prompt, "Execution Protocol") {
		t.Error("static sections should remain with custom prompt")
	}
}

func TestPromptRegistry(t *testing.T) {
	t.Run("register and resolve", func(t *testing.T) {
		r := NewPromptRegistry()
		r.Register(PromptSection{
			Name:  "test-static",
			Phase: PhaseStatic,
			Order: 10,
			Compute: func() string {
				return "STATIC CONTENT\n"
			},
		})
		r.Register(PromptSection{
			Name:  "test-dynamic",
			Phase: PhaseDynamic,
			Order: 10,
			Compute: func() string {
				return "DYNAMIC CONTENT\n"
			},
		})

		result := r.Resolve()
		if !strings.Contains(result, "STATIC CONTENT") {
			t.Error("should contain static section")
		}
		if !strings.Contains(result, "DYNAMIC CONTENT") {
			t.Error("should contain dynamic section")
		}
		// Static should come before dynamic boundary marker.
		idxStatic := strings.Index(result, "STATIC CONTENT")
		idxBoundary := strings.Index(result, "Task-Specific Context")
		if idxStatic > idxBoundary {
			t.Error("static sections should come before dynamic boundary")
		}
	})

	t.Run("memoization caches results", func(t *testing.T) {
		r := NewPromptRegistry()
		count := 0
		r.Register(PromptSection{
			Name:  "counter",
			Phase: PhaseStatic,
			Order: 10,
			Compute: func() string {
				count++
				return "computed"
			},
		})

		r.Resolve()
		r.Resolve()
		if count != 1 {
			t.Errorf("section should be computed once, got %d calls", count)
		}
	})

	t.Run("invalidate forces recomputation", func(t *testing.T) {
		r := NewPromptRegistry()
		count := 0
		r.Register(PromptSection{
			Name:  "counter",
			Phase: PhaseStatic,
			Order: 10,
			Compute: func() string {
				count++
				return "computed"
			},
		})

		r.Resolve()
		r.Invalidate()
		r.Resolve()
		if count != 2 {
			t.Errorf("after invalidate, section should be recomputed, got %d calls", count)
		}
	})

	t.Run("invalidate section clears single entry", func(t *testing.T) {
		r := NewPromptRegistry()
		countA := 0
		countB := 0
		r.Register(PromptSection{
			Name:  "section-a",
			Phase: PhaseStatic,
			Order: 10,
			Compute: func() string {
				countA++
				return "A"
			},
		})
		r.Register(PromptSection{
			Name:  "section-b",
			Phase: PhaseStatic,
			Order: 20,
			Compute: func() string {
				countB++
				return "B"
			},
		})

		r.Resolve()
		r.InvalidateSection("section-a")
		r.Resolve()
		if countA != 2 {
			t.Errorf("section-a should be recomputed, got %d calls", countA)
		}
		if countB != 1 {
			t.Errorf("section-b should stay cached, got %d calls", countB)
		}
	})

	t.Run("duplicate name replaces section", func(t *testing.T) {
		r := NewPromptRegistry()
		r.Register(PromptSection{
			Name:  "dup",
			Phase: PhaseStatic,
			Order: 10,
			Compute: func() string {
				return "FIRST"
			},
		})
		r.Register(PromptSection{
			Name:  "dup",
			Phase: PhaseStatic,
			Order: 10,
			Compute: func() string {
				return "SECOND"
			},
		})

		result := r.Resolve()
		if strings.Contains(result, "FIRST") {
			t.Error("replaced section should not appear")
		}
		if !strings.Contains(result, "SECOND") {
			t.Error("new section content should appear")
		}
	})

	t.Run("nil compute returns empty", func(t *testing.T) {
		r := NewPromptRegistry()
		r.Register(PromptSection{
			Name:    "nil-section",
			Phase:   PhaseStatic,
			Order:   10,
			Compute: nil,
		})

		result := r.Resolve()
		if strings.Contains(result, "nil-section") {
			t.Error("nil compute should produce no output")
		}
	})

	t.Run("empty compute returns empty", func(t *testing.T) {
		r := NewPromptRegistry()
		r.Register(PromptSection{
			Name:  "empty",
			Phase: PhaseStatic,
			Order: 10,
			Compute: func() string {
				return ""
			},
		})

		result := r.Resolve()
		if result != "" {
			t.Errorf("all-empty registry should resolve to empty string, got %q", result)
		}
	})
}

func TestCoordinatorForkGuidance(t *testing.T) {
	t.Run("coordinator gets fork guidance", func(t *testing.T) {
		prompt := AssembleSystemPrompt(SystemPromptConfig{
			AgentName:  "coordinator-agent",
			AgentRole:  "coordinator",
			AppendPrompt: "", // ensure no override
		})
		if !strings.Contains(prompt, "Sub-Agent Delegation") {
			t.Error("coordinator prompt should contain sub-agent delegation section")
		}
		if !strings.Contains(prompt, "Don't peek") {
			t.Error("coordinator prompt should mention Don't peek rule")
		}
		if !strings.Contains(prompt, "directive-style") {
			t.Error("coordinator prompt should mention directive-style prompts")
		}
	})

	t.Run("executor does not get fork guidance", func(t *testing.T) {
		prompt := AssembleSystemPrompt(SystemPromptConfig{
			AgentName: "executor-agent",
			AgentRole: "executor",
		})
		if strings.Contains(prompt, "Sub-Agent Delegation") {
			t.Error("executor prompt should not contain sub-agent delegation section")
		}
	})

	t.Run("empty role does not get fork guidance", func(t *testing.T) {
		prompt := AssembleSystemPrompt(SystemPromptConfig{
			AgentName: "default-agent",
		})
		if strings.Contains(prompt, "Sub-Agent Delegation") {
			t.Error("default role prompt should not contain sub-agent delegation section")
		}
	})

	t.Run("reviewer does not get fork guidance", func(t *testing.T) {
		prompt := AssembleSystemPrompt(SystemPromptConfig{
			AgentName: "reviewer-agent",
			AgentRole: "reviewer",
		})
		if strings.Contains(prompt, "Sub-Agent Delegation") {
			t.Error("reviewer prompt should not contain sub-agent delegation section")
		}
	})
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
