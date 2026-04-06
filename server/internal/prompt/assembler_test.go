package prompt

import (
	"context"
	"strings"
	"testing"
)

func TestAssembler_BasicBuild(t *testing.T) {
	a := NewAssembler()
	a.Add("first", 10, "Layer A")
	a.Add("second", 20, "Layer B")
	a.Add("third", 30, "Layer C")

	got := a.Build()
	want := "Layer A\n\nLayer B\n\nLayer C"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestAssembler_SkipsEmptyLayers(t *testing.T) {
	a := NewAssembler()
	a.Add("first", 10, "Layer A")
	a.Add("empty", 20, "")
	a.Add("whitespace", 30, "   ")
	a.Add("third", 40, "Layer C")

	got := a.Build()
	want := "Layer A\n\nLayer C"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestAssembler_PriorityOrder(t *testing.T) {
	a := NewAssembler()
	a.Add("high", 30, "Low priority")
	a.Add("low", 10, "High priority")
	a.Add("mid", 20, "Medium priority")

	got := a.Build()
	want := "High priority\n\nMedium priority\n\nLow priority"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestAssemblePrompt_FullContext(t *testing.T) {
	lctx := &LayerContext{
		AppName:         "Alphenix",
		AppVersion:      "1.0",
		AgentRole:       "Senior Developer",
		WorkspaceRules:  "Always write tests.",
		AgentProfile:    "Focus on Go code.",
		TaskTitle:       "Fix auth bug",
		TaskDescription: "JWT validation fails on expired tokens.",
		IssueStatus:     "in_progress",
		SkillDescriptions: []SkillInfo{
			{Name: "test-writer", Description: "Generates test cases"},
		},
		TodoItems: []TodoInfo{
			{Title: "Reproduce the bug", Status: "completed"},
			{Title: "Fix the validator", Status: "in_progress"},
		},
		LastCheckpoint: "Validated token parsing is correct.",
		AllowedTools:   []string{"read_file", "write_file"},
		RestrictedTools: []string{"shell_exec"},
	}

	got := AssemblePrompt(context.Background(), lctx)

	// Verify all sections are present in order
	checks := []string{
		"only reliable recovery", // compaction safety (Priority 5, highest)
		"Alphenix",
		"Startup Sequence",
		"Senior Developer",
		"Workspace Policy",
		"Agent Instructions",
		"Current Task: Fix auth bug",
		"Available Skills",
		"test-writer",
		"Current Todo List",
		"[x] Reproduce the bug",
		"[>] Fix the validator",
		"Last Checkpoint",
		"Allowed Tools",
		"Restricted Tools",
	}

	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("Build() missing %q", check)
		}
	}

	// Verify ordering: base system before role, role before workspace, etc.
	for i := 0; i < len(checks)-1; i++ {
		pos1 := strings.Index(got, checks[i])
		pos2 := strings.Index(got, checks[i+1])
		if pos1 > pos2 {
			t.Errorf("Order violation: %q (pos %d) should come before %q (pos %d)",
				checks[i], pos1, checks[i+1], pos2)
		}
	}
}

func TestAssemblePrompt_MinimalContext(t *testing.T) {
	lctx := &LayerContext{}
	got := AssemblePrompt(context.Background(), lctx)

	// Should still have base system, just nothing else
	if !strings.Contains(got, "Alphenix") {
		t.Error("minimal context should still include app name")
	}
	// Compaction safety and startup sequence are always present
	if !strings.Contains(got, "only reliable recovery") {
		t.Error("compaction safety should always be present")
	}
	if !strings.Contains(got, "Startup Sequence") {
		t.Error("startup sequence should always be present")
	}
	// Should NOT have empty sections
	if strings.Contains(got, "Workspace Policy") {
		t.Error("empty workspace rules should not produce section")
	}
	if strings.Contains(got, "Current Task") {
		t.Error("empty task should not produce section")
	}
}

func TestAssembleCompactionSafety(t *testing.T) {
	content, err := assembleCompactionSafety(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "compaction") {
		t.Error("should mention compaction")
	}
	if !strings.Contains(content, "checkpoint") {
		t.Error("should mention checkpoint as recovery source")
	}
}

func TestAssembleStartupSequence(t *testing.T) {
	content, err := assembleStartupSequence(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	steps := []string{"Acknowledge", "Check Memory", "Process", "Complete All Work", "Report"}
	for _, step := range steps {
		if !strings.Contains(content, step) {
			t.Errorf("startup sequence missing step: %q", step)
		}
	}
}
