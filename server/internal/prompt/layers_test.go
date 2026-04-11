package prompt

import (
	"context"
	"strings"
	"testing"
)

// --- joinNonEmpty ---

func TestJoinNonEmpty_Basic(t *testing.T) {
	got := joinNonEmpty([]string{"a", "b", "c"}, ", ")
	if got != "a, b, c" {
		t.Errorf("got %q, want %q", got, "a, b, c")
	}
}

func TestJoinNonEmpty_SkipsEmpty(t *testing.T) {
	got := joinNonEmpty([]string{"a", "", "c"}, ", ")
	if got != "a, c" {
		t.Errorf("got %q, want %q", got, "a, c")
	}
}

func TestJoinNonEmpty_AllEmpty(t *testing.T) {
	got := joinNonEmpty([]string{"", "", ""}, ", ")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestJoinNonEmpty_Nil(t *testing.T) {
	got := joinNonEmpty(nil, ", ")
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestJoinNonEmpty_Single(t *testing.T) {
	got := joinNonEmpty([]string{"only"}, "\n")
	if got != "only" {
		t.Errorf("got %q, want %q", got, "only")
	}
}

// --- assembleBaseSystem ---

func TestAssembleBaseSystem_Defaults(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleBaseSystem(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Alphenix") {
		t.Error("should contain default app name")
	}
	if !strings.Contains(got, "dev") {
		t.Error("should contain default version")
	}
}

func TestAssembleBaseSystem_Custom(t *testing.T) {
	lctx := &LayerContext{AppName: "MyApp", AppVersion: "2.0"}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleBaseSystem(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "MyApp") {
		t.Error("should contain custom app name")
	}
	if !strings.Contains(got, "2.0") {
		t.Error("should contain custom version")
	}
}

// --- assembleAlphenixRole ---

func TestAssembleAlphenixRole_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleAlphenixRole(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty for no role, got %q", got)
	}
}

func TestAssembleAlphenixRole_WithRole(t *testing.T) {
	lctx := &LayerContext{AgentRole: "Code Reviewer"}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleAlphenixRole(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Code Reviewer") {
		t.Error("should contain role")
	}
}

// --- assembleWorkspacePolicy ---

func TestAssembleWorkspacePolicy_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleWorkspacePolicy(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestAssembleWorkspacePolicy_WithRules(t *testing.T) {
	lctx := &LayerContext{WorkspaceRules: "Always write tests."}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleWorkspacePolicy(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Workspace Policy") {
		t.Error("should contain header")
	}
	if !strings.Contains(got, "Always write tests.") {
		t.Error("should contain rules")
	}
}

// --- assembleAgentProfile ---

func TestAssembleAgentProfile_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleAgentProfile(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestAssembleAgentProfile_WithProfile(t *testing.T) {
	lctx := &LayerContext{AgentProfile: "Focus on Go."}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleAgentProfile(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Agent Instructions") {
		t.Error("should contain header")
	}
	if !strings.Contains(got, "Focus on Go.") {
		t.Error("should contain profile")
	}
}

// --- assembleTaskObjective ---

func TestAssembleTaskObjective_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleTaskObjective(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestAssembleTaskObjective_TitleOnly(t *testing.T) {
	lctx := &LayerContext{TaskTitle: "Fix auth"}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleTaskObjective(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Fix auth") {
		t.Error("should contain title")
	}
	if strings.Contains(got, "Status:") {
		t.Error("should not contain status when empty")
	}
}

func TestAssembleTaskObjective_Full(t *testing.T) {
	lctx := &LayerContext{
		TaskTitle:       "Fix auth",
		TaskDescription: "JWT fails on expired tokens",
		IssueStatus:     "in_progress",
	}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleTaskObjective(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Fix auth") {
		t.Error("should contain title")
	}
	if !strings.Contains(got, "JWT fails") {
		t.Error("should contain description")
	}
	if !strings.Contains(got, "in_progress") {
		t.Error("should contain status")
	}
}

// --- assembleSkills ---

func TestAssembleSkills_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleSkills(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestAssembleSkills_WithSkills(t *testing.T) {
	lctx := &LayerContext{
		SkillDescriptions: []SkillInfo{
			{Name: "test-writer", Description: "Generates tests"},
			{Name: "reviewer", Description: "Reviews PRs"},
		},
	}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleSkills(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "test-writer") {
		t.Error("should contain first skill")
	}
	if !strings.Contains(got, "reviewer") {
		t.Error("should contain second skill")
	}
}

// --- assembleTodo ---

func TestAssembleTodo_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleTodo(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestAssembleTodo_StatusMarkers(t *testing.T) {
	lctx := &LayerContext{
		TodoItems: []TodoInfo{
			{Title: "Done task", Status: "completed"},
			{Title: "Active task", Status: "in_progress"},
			{Title: "Pending task", Status: "todo"},
		},
	}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleTodo(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "[x] Done task") {
		t.Error("completed should use [x]")
	}
	if !strings.Contains(got, "[>] Active task") {
		t.Error("in_progress should use [>]")
	}
	if !strings.Contains(got, "[ ] Pending task") {
		t.Error("todo should use [ ]")
	}
}

func TestAssembleTodo_WithDescription(t *testing.T) {
	lctx := &LayerContext{
		TodoItems: []TodoInfo{
			{Title: "Fix bug", Description: "In auth module", Status: "todo"},
		},
	}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleTodo(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "In auth module") {
		t.Error("should contain description")
	}
}

// --- assembleCheckpoint ---

func TestAssembleCheckpoint_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleCheckpoint(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestAssembleCheckpoint_WithCheckpoint(t *testing.T) {
	lctx := &LayerContext{LastCheckpoint: "Validated token parsing."}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleCheckpoint(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Validated token parsing.") {
		t.Error("should contain checkpoint text")
	}
}

// --- assembleToolPolicy ---

func TestAssembleToolPolicy_Empty(t *testing.T) {
	ctx := context.WithValue(context.Background(), layerCtxKey, &LayerContext{})
	got, err := assembleToolPolicy(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestAssembleToolPolicy_AllowedOnly(t *testing.T) {
	lctx := &LayerContext{AllowedTools: []string{"read", "write"}}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleToolPolicy(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Allowed Tools") {
		t.Error("should contain Allowed Tools header")
	}
	if strings.Contains(got, "Restricted") {
		t.Error("should not contain Restricted when none set")
	}
}

func TestAssembleToolPolicy_RestrictedOnly(t *testing.T) {
	lctx := &LayerContext{RestrictedTools: []string{"shell_exec"}}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleToolPolicy(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Restricted Tools") {
		t.Error("should contain Restricted Tools header")
	}
	if !strings.Contains(got, "elevated permission") {
		t.Error("should mention elevated permission")
	}
}

func TestAssembleToolPolicy_Both(t *testing.T) {
	lctx := &LayerContext{
		AllowedTools:   []string{"read"},
		RestrictedTools: []string{"exec"},
	}
	ctx := context.WithValue(context.Background(), layerCtxKey, lctx)
	got, err := assembleToolPolicy(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Allowed Tools") {
		t.Error("should contain Allowed Tools")
	}
	if !strings.Contains(got, "Restricted Tools") {
		t.Error("should contain Restricted Tools")
	}
}

// --- DefaultLayers ---

func TestDefaultLayers_Count(t *testing.T) {
	layers := DefaultLayers()
	if len(layers) != 11 {
		t.Errorf("expected 11 layers, got %d", len(layers))
	}
}

func TestDefaultLayers_PriorityOrder(t *testing.T) {
	layers := DefaultLayers()
	for i := 1; i < len(layers); i++ {
		if layers[i].Priority < layers[i-1].Priority {
			t.Errorf("layer %d (%s) has lower priority than layer %d (%s)",
				i, layers[i].Name, i-1, layers[i-1].Name)
		}
	}
}
