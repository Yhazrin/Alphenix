package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SubtaskPreview JSON round-trip
// ---------------------------------------------------------------------------

func TestSubtaskPreview_JSONRoundTrip(t *testing.T) {
	assigneeType := "agent"
	st := SubtaskPreview{
		Title:        "Implement login",
		Description:  "Add OAuth2 login flow",
		Deliverable:  "Working login endpoint",
		DependsOn:    []int{0, 1},
		AssigneeType: &assigneeType,
	}

	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SubtaskPreview
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Title != "Implement login" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Description != "Add OAuth2 login flow" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Deliverable != "Working login endpoint" {
		t.Errorf("Deliverable = %q", got.Deliverable)
	}
	if len(got.DependsOn) != 2 || got.DependsOn[0] != 0 || got.DependsOn[1] != 1 {
		t.Errorf("DependsOn = %v", got.DependsOn)
	}
	if got.AssigneeType == nil || *got.AssigneeType != "agent" {
		t.Errorf("AssigneeType = %v", got.AssigneeType)
	}
}

func TestSubtaskPreview_NilOptionalFields(t *testing.T) {
	st := SubtaskPreview{
		Title:       "Simple task",
		Description: "Do X",
		Deliverable: "X done",
		DependsOn:   []int{},
	}

	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SubtaskPreview
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.AssigneeType != nil {
		t.Errorf("AssigneeType should be nil, got %q", *got.AssigneeType)
	}
	if got.AssigneeID != nil {
		t.Errorf("AssigneeID should be nil, got %q", *got.AssigneeID)
	}
	if len(got.DependsOn) != 0 {
		t.Errorf("DependsOn should be empty, got %v", got.DependsOn)
	}
}

func TestSubtaskPreview_Omitempty(t *testing.T) {
	st := SubtaskPreview{
		Title:       "T",
		Description: "D",
		Deliverable: "Del",
		DependsOn:   []int{},
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	// assignee_type and assignee_id should be omitted when nil
	if strings.Contains(s, "assignee_type") {
		t.Error("assignee_type should be omitted when nil")
	}
	if strings.Contains(s, "assignee_id") {
		t.Error("assignee_id should be omitted when nil")
	}
}

// ---------------------------------------------------------------------------
// DecomposePreview JSON
// ---------------------------------------------------------------------------

func TestDecomposePreview_JSONRoundTrip(t *testing.T) {
	preview := DecomposePreview{
		Subtasks: []SubtaskPreview{
			{Title: "A", Description: "desc A", Deliverable: "del A", DependsOn: []int{}},
			{Title: "B", Description: "desc B", Deliverable: "del B", DependsOn: []int{0}},
		},
		PlanSummary: "Two-step plan",
		Risks:       []string{"risk 1", "risk 2"},
	}

	b, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got DecomposePreview
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Subtasks) != 2 {
		t.Fatalf("Subtasks length = %d, want 2", len(got.Subtasks))
	}
	if got.Subtasks[1].DependsOn[0] != 0 {
		t.Errorf("Subtasks[1].DependsOn[0] = %d, want 0", got.Subtasks[1].DependsOn[0])
	}
	if got.PlanSummary != "Two-step plan" {
		t.Errorf("PlanSummary = %q", got.PlanSummary)
	}
	if len(got.Risks) != 2 {
		t.Errorf("Risks length = %d, want 2", len(got.Risks))
	}
}

func TestDecomposePreview_EmptySubtasks(t *testing.T) {
	preview := DecomposePreview{
		Subtasks:    []SubtaskPreview{},
		PlanSummary: "empty",
		Risks:       []string{},
	}
	b, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got DecomposePreview
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Subtasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(got.Subtasks))
	}
}

// ---------------------------------------------------------------------------
// ConfirmDecomposeRequest JSON
// ---------------------------------------------------------------------------

func TestConfirmDecomposeRequest_ValidJSON(t *testing.T) {
	body := `{"subtasks":[{"title":"T1","description":"D1","deliverable":"Del1","depends_on":[]},{"title":"T2","description":"D2","deliverable":"Del2","depends_on":[0]}]}`
	var req ConfirmDecomposeRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.Subtasks) != 2 {
		t.Fatalf("Subtasks length = %d, want 2", len(req.Subtasks))
	}
	if req.Subtasks[0].Title != "T1" {
		t.Errorf("Subtasks[0].Title = %q", req.Subtasks[0].Title)
	}
	if len(req.Subtasks[1].DependsOn) != 1 || req.Subtasks[1].DependsOn[0] != 0 {
		t.Errorf("Subtasks[1].DependsOn = %v", req.Subtasks[1].DependsOn)
	}
}

func TestConfirmDecomposeRequest_EmptySubtasks(t *testing.T) {
	body := `{"subtasks":[]}`
	var req ConfirmDecomposeRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.Subtasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(req.Subtasks))
	}
}

func TestConfirmDecomposeRequest_InvalidJSON(t *testing.T) {
	body := `{invalid`
	var req ConfirmDecomposeRequest
	if err := json.Unmarshal([]byte(body), &req); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// DecomposeResponse JSON
// ---------------------------------------------------------------------------

func TestDecomposeResponse_Running(t *testing.T) {
	resp := DecomposeResponse{
		RunID:  "run-123",
		Status: "running",
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"running"`) {
		t.Error("should contain running status")
	}
	// preview and error should be omitted when nil/empty
	if strings.Contains(s, `"preview"`) {
		t.Error("preview should be omitted when nil")
	}
}

func TestDecomposeResponse_Completed(t *testing.T) {
	preview := &DecomposePreview{
		Subtasks:    []SubtaskPreview{{Title: "T", Description: "D", Deliverable: "Del", DependsOn: []int{}}},
		PlanSummary: "plan",
		Risks:       []string{},
	}
	resp := DecomposeResponse{
		RunID:   "run-456",
		Status:  "completed",
		Preview: preview,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got DecomposeResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("Status = %q", got.Status)
	}
	if got.Preview == nil {
		t.Fatal("Preview should not be nil")
	}
	if len(got.Preview.Subtasks) != 1 {
		t.Errorf("Preview.Subtasks length = %d", len(got.Preview.Subtasks))
	}
}

func TestDecomposeResponse_Failed(t *testing.T) {
	resp := DecomposeResponse{
		RunID:  "run-789",
		Status: "failed",
		Error:  "decomposition failed",
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got DecomposeResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("Status = %q", got.Status)
	}
	if got.Error != "decomposition failed" {
		t.Errorf("Error = %q", got.Error)
	}
}

// ---------------------------------------------------------------------------
// Dependency boundary validation logic (mirrors ConfirmDecompose lines 342-344)
// ---------------------------------------------------------------------------

// validateDepIndex checks whether a dependency index is valid (not out of bounds, not self-referencing).
func validateDepIndex(depIdx, currentIdx, total int) bool {
	return depIdx >= 0 && depIdx < total && depIdx != currentIdx
}

func TestValidateDepIndex_Valid(t *testing.T) {
	if !validateDepIndex(0, 1, 3) {
		t.Error("index 0 should be valid for current=1, total=3")
	}
}

func TestValidateDepIndex_Negative(t *testing.T) {
	if validateDepIndex(-1, 1, 3) {
		t.Error("negative index should be invalid")
	}
}

func TestValidateDepIndex_OutOfBounds(t *testing.T) {
	if validateDepIndex(3, 0, 3) {
		t.Error("index 3 should be out of bounds for total=3")
	}
	if validateDepIndex(100, 0, 3) {
		t.Error("index 100 should be out of bounds")
	}
}

func TestValidateDepIndex_SelfReference(t *testing.T) {
	if validateDepIndex(2, 2, 5) {
		t.Error("self-reference should be invalid")
	}
}

func TestValidateDepIndex_EmptyTotal(t *testing.T) {
	if validateDepIndex(0, 0, 0) {
		t.Error("any index with total=0 should be invalid")
	}
}

func TestValidateDepIndex_FirstToLast(t *testing.T) {
	// Last task depends on first — valid
	if !validateDepIndex(0, 2, 3) {
		t.Error("last depending on first should be valid")
	}
}

func TestValidateDepIndex_LastToFirst(t *testing.T) {
	// First task depends on last — valid
	if !validateDepIndex(2, 0, 3) {
		t.Error("first depending on last should be valid")
	}
}

// ---------------------------------------------------------------------------
// Dependency wiring with boundary checks (mirrors ConfirmDep loop)
// ---------------------------------------------------------------------------

type mockSubtask struct {
	Title      string
	DependsOn  []int
}

func wireDependencies(subtasks []mockSubtask) [][2]int {
	var edges [][2]int
	for i, st := range subtasks {
		for _, depIdx := range st.DependsOn {
			if !validateDepIndex(depIdx, i, len(subtasks)) {
				continue
			}
			edges = append(edges, [2]int{i, depIdx})
		}
	}
	return edges
}

func TestWireDependencies_ValidChain(t *testing.T) {
	subtasks := []mockSubtask{
		{Title: "A", DependsOn: []int{}},
		{Title: "B", DependsOn: []int{0}},
		{Title: "C", DependsOn: []int{0, 1}},
	}
	edges := wireDependencies(subtasks)
	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}
	// B -> A
	if edges[0][0] != 1 || edges[0][1] != 0 {
		t.Errorf("edge[0] = %v, want [1,0]", edges[0])
	}
	// C -> A
	if edges[1][0] != 2 || edges[1][1] != 0 {
		t.Errorf("edge[1] = %v, want [2,0]", edges[1])
	}
	// C -> B
	if edges[2][0] != 2 || edges[2][1] != 1 {
		t.Errorf("edge[2] = %v, want [2,1]", edges[2])
	}
}

func TestWireDependencies_SelfRefSkipped(t *testing.T) {
	subtasks := []mockSubtask{
		{Title: "A", DependsOn: []int{0}}, // self-ref
	}
	edges := wireDependencies(subtasks)
	if len(edges) != 0 {
		t.Errorf("self-ref should produce 0 edges, got %d", len(edges))
	}
}

func TestWireDependencies_OutOfBoundsSkipped(t *testing.T) {
	subtasks := []mockSubtask{
		{Title: "A", DependsOn: []int{5}},
		{Title: "B", DependsOn: []int{-1}},
	}
	edges := wireDependencies(subtasks)
	if len(edges) != 0 {
		t.Errorf("out-of-bounds should produce 0 edges, got %d", len(edges))
	}
}

func TestWireDependencies_MixedValidInvalid(t *testing.T) {
	subtasks := []mockSubtask{
		{Title: "A", DependsOn: []int{}},
		{Title: "B", DependsOn: []int{0, 1, -1, 99}}, // 0 is valid, 1 is self, -1 and 99 invalid
	}
	edges := wireDependencies(subtasks)
	if len(edges) != 1 {
		t.Fatalf("expected 1 valid edge, got %d", len(edges))
	}
	if edges[0][0] != 1 || edges[0][1] != 0 {
		t.Errorf("edge = %v, want [1,0]", edges[0])
	}
}

func TestWireDependencies_EmptySubtasks(t *testing.T) {
	edges := wireDependencies(nil)
	if len(edges) != 0 {
		t.Errorf("nil subtasks should produce 0 edges, got %d", len(edges))
	}
}

func TestWireDependencies_ParallelNoDeps(t *testing.T) {
	subtasks := []mockSubtask{
		{Title: "A", DependsOn: []int{}},
		{Title: "B", DependsOn: []int{}},
		{Title: "C", DependsOn: []int{}},
	}
	edges := wireDependencies(subtasks)
	if len(edges) != 0 {
		t.Errorf("no deps should produce 0 edges, got %d", len(edges))
	}
}

// ---------------------------------------------------------------------------
// decomposeSystemPrompt constant
// ---------------------------------------------------------------------------

func TestDecomposeSystemPrompt_ContainsKeyInstructions(t *testing.T) {
	prompt := decomposeSystemPrompt
	checks := []string{
		"Architect Agent",
		"subtasks",
		"depends_on",
		"assignee_type",
		"plan_summary",
		"risks",
		"JSON",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("system prompt missing %q", c)
		}
	}
}

func TestDecomposeSystemPrompt_NoMarkdownFences(t *testing.T) {
	prompt := decomposeSystemPrompt
	if strings.Contains(prompt, "```") {
		t.Error("system prompt should not contain markdown fences")
	}
}

func TestDecomposeSystemPrompt_HasSchema(t *testing.T) {
	prompt := decomposeSystemPrompt
	// Should contain the JSON schema structure
	if !strings.Contains(prompt, `"subtasks"`) {
		t.Error("system prompt should contain subtasks schema field")
	}
	if !strings.Contains(prompt, `"deliverable"`) {
		t.Error("system prompt should contain deliverable schema field")
	}
}
