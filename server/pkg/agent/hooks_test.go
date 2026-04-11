package agent

import (
	"context"
	"testing"
)

// --- HookMatcher.Matches ---

func TestHookMatcher_NilMatcher(t *testing.T) {
	var m *HookMatcher
	if !m.Matches("anything") {
		t.Error("nil matcher should match all tools")
	}
}

func TestHookMatcher_EmptyTools(t *testing.T) {
	m := &HookMatcher{}
	if !m.Matches("anything") {
		t.Error("empty tools list should match all tools")
	}
}

func TestHookMatcher_MatchFound(t *testing.T) {
	m := &HookMatcher{Tools: []string{"bash", "edit"}}
	if !m.Matches("bash") {
		t.Error("expected match for 'bash'")
	}
	if m.Matches("write") {
		t.Error("expected no match for 'write'")
	}
}

func TestHookMatcher_Negate(t *testing.T) {
	m := &HookMatcher{Tools: []string{"bash"}, Negate: true}
	if m.Matches("bash") {
		t.Error("negated matcher should NOT match 'bash'")
	}
	if !m.Matches("edit") {
		t.Error("negated matcher should match 'edit'")
	}
}

func TestHookMatcher_NegateEmptyTools(t *testing.T) {
	m := &HookMatcher{Negate: true}
	// Empty tools → matches all → negate → matches all (since found=false, !found=true)
	if !m.Matches("anything") {
		t.Error("negated matcher with empty tools should match all")
	}
}

// --- HookChain.RunPreToolUse ---

func TestRunPreToolUse_NilChain(t *testing.T) {
	var hc *HookChain
	result := hc.RunPreToolUse(context.Background(), "bash", nil)
	if result.Deny {
		t.Error("nil chain should return allow")
	}
}

func TestRunPreToolUse_EmptyChain(t *testing.T) {
	hc := &HookChain{}
	result := hc.RunPreToolUse(context.Background(), "bash", nil)
	if result.Deny {
		t.Error("empty chain should return allow")
	}
}

func TestRunPreToolUse_AllowByDefault(t *testing.T) {
	hc := &HookChain{
		PreToolUse: []HookEntry{
			{Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
				return ToolHookResult{} // allow
			}},
		},
	}
	result := hc.RunPreToolUse(context.Background(), "bash", nil)
	if result.Deny {
		t.Error("expected allow")
	}
}

func TestRunPreToolUse_FirstDenyWins(t *testing.T) {
	callCount := 0
	hc := &HookChain{
		PreToolUse: []HookEntry{
			{Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
				callCount++
				return ToolHookResult{Deny: true, DenyReason: "blocked"}
			}},
			{Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
				callCount++
				return ToolHookResult{}
			}},
		},
	}
	result := hc.RunPreToolUse(context.Background(), "bash", nil)
	if !result.Deny {
		t.Error("expected deny from first hook")
	}
	if result.DenyReason != "blocked" {
		t.Errorf("expected reason %q, got %q", "blocked", result.DenyReason)
	}
	if callCount != 1 {
		t.Errorf("expected only 1 hook called (deny stops chain), got %d", callCount)
	}
}

func TestRunPreToolUse_UpdatedInputPassedThrough(t *testing.T) {
	var capturedInput map[string]any
	hc := &HookChain{
		PreToolUse: []HookEntry{
			{Run: func(_ context.Context, _ string, input map[string]any, _ string) ToolHookResult {
				return ToolHookResult{UpdatedInput: map[string]any{"key": "updated"}}
			}},
			{Run: func(_ context.Context, _ string, input map[string]any, _ string) ToolHookResult {
				capturedInput = input
				return ToolHookResult{}
			}},
		},
	}
	hc.RunPreToolUse(context.Background(), "bash", map[string]any{"key": "original"})
	if capturedInput == nil || capturedInput["key"] != "updated" {
		t.Errorf("expected updated input passed to next hook, got %v", capturedInput)
	}
}

func TestRunPreToolUse_MatcherFilters(t *testing.T) {
	bashCalled := false
	hc := &HookChain{
		PreToolUse: []HookEntry{
			{
				Matcher: &HookMatcher{Tools: []string{"bash"}},
				Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
					bashCalled = true
					return ToolHookResult{Deny: true, DenyReason: "bash blocked"}
				},
			},
		},
	}
	// Call with "edit" — should not match
	result := hc.RunPreToolUse(context.Background(), "edit", nil)
	if bashCalled {
		t.Error("hook should not have been called for 'edit'")
	}
	if result.Deny {
		t.Error("expected allow (no matching hook)")
	}

	// Call with "bash" — should match
	result = hc.RunPreToolUse(context.Background(), "bash", nil)
	if !bashCalled {
		t.Error("hook should have been called for 'bash'")
	}
	if !result.Deny {
		t.Error("expected deny")
	}
}

// --- HookChain.RunPostToolUse ---

func TestRunPostToolUse_NilChain(t *testing.T) {
	var hc *HookChain
	// Should not panic
	hc.RunPostToolUse(context.Background(), "bash", nil, "output")
}

func TestRunPostToolUse_EmptyChain(t *testing.T) {
	hc := &HookChain{}
	hc.RunPostToolUse(context.Background(), "bash", nil, "output")
}

func TestRunPostToolUse_AllMatchingRun(t *testing.T) {
	count := 0
	hc := &HookChain{
		PostToolUse: []HookEntry{
			{Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
				count++
				return ToolHookResult{}
			}},
			{Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
				count++
				return ToolHookResult{}
			}},
		},
	}
	hc.RunPostToolUse(context.Background(), "bash", nil, "output")
	if count != 2 {
		t.Errorf("expected 2 hooks called, got %d", count)
	}
}

func TestRunPostToolUse_MatcherFilters(t *testing.T) {
	count := 0
	hc := &HookChain{
		PostToolUse: []HookEntry{
			{
				Matcher: &HookMatcher{Tools: []string{"bash"}},
				Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
					count++
					return ToolHookResult{}
				},
			},
		},
	}
	hc.RunPostToolUse(context.Background(), "edit", nil, "output")
	if count != 0 {
		t.Errorf("expected 0 calls for non-matching tool, got %d", count)
	}
	hc.RunPostToolUse(context.Background(), "bash", nil, "output")
	if count != 1 {
		t.Errorf("expected 1 call for matching tool, got %d", count)
	}
}

// --- ResolvePermission ---

func TestResolvePermission_NilPermsNilChain(t *testing.T) {
	result := ResolvePermission(context.Background(), "bash", nil, nil, nil)
	if result.Deny {
		t.Error("expected allow with nil perms and nil chain")
	}
}

func TestResolvePermission_DeniedByPerms(t *testing.T) {
	perms := &ToolPermissions{DeniedTools: []string{"bash"}}
	result := ResolvePermission(context.Background(), "bash", nil, perms, nil)
	if !result.Deny {
		t.Error("expected deny for denied tool")
	}
}

func TestResolvePermission_AllowedByPerms(t *testing.T) {
	perms := &ToolPermissions{AllowedTools: []string{"bash", "edit"}}
	result := ResolvePermission(context.Background(), "bash", nil, perms, nil)
	if result.Deny {
		t.Error("expected allow for allowed tool")
	}
}

func TestResolvePermission_DeniedByChain(t *testing.T) {
	chain := &HookChain{
		PreToolUse: []HookEntry{
			{Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
				return ToolHookResult{Deny: true, DenyReason: "chain blocked"}
			}},
		},
	}
	result := ResolvePermission(context.Background(), "bash", nil, nil, chain)
	if !result.Deny {
		t.Error("expected deny from chain")
	}
}

func TestResolvePermission_PermDeniedOverridesChainAllow(t *testing.T) {
	perms := &ToolPermissions{DeniedTools: []string{"bash"}}
	chain := &HookChain{
		PreToolUse: []HookEntry{
			{Run: func(_ context.Context, _ string, _ map[string]any, _ string) ToolHookResult {
				return ToolHookResult{} // chain allows
			}},
		},
	}
	result := ResolvePermission(context.Background(), "bash", nil, perms, chain)
	if !result.Deny {
		t.Error("perms deny should take precedence over chain allow")
	}
}

// --- mergeHookResult ---

func TestMergeHookResult_DenyExplicit(t *testing.T) {
	hr := HookResult{Deny: true, DenyReason: "nope"}
	got := mergeHookResult(hr)
	if !got.Deny {
		t.Error("expected deny")
	}
	if got.DenyReason != "nope" {
		t.Errorf("expected reason %q, got %q", "nope", got.DenyReason)
	}
}

func TestMergeHookResult_DecisionDeny(t *testing.T) {
	hr := HookResult{Decision: PermissionDeny}
	got := mergeHookResult(hr)
	if !got.Deny {
		t.Error("expected deny from Decision")
	}
	if got.DenyReason != "denied by hook" {
		t.Errorf("expected default reason, got %q", got.DenyReason)
	}
}

func TestMergeHookResult_DecisionDenyPreservesReason(t *testing.T) {
	hr := HookResult{Decision: PermissionDeny, DenyReason: "custom"}
	got := mergeHookResult(hr)
	if got.DenyReason != "custom" {
		t.Errorf("expected custom reason preserved, got %q", got.DenyReason)
	}
}

func TestMergeHookResult_DecisionApprove(t *testing.T) {
	hr := HookResult{Deny: true, Decision: PermissionApprove}
	got := mergeHookResult(hr)
	if got.Deny {
		t.Error("approve should override deny")
	}
	if got.DenyReason != "" {
		t.Errorf("expected empty reason on approve, got %q", got.DenyReason)
	}
}

func TestMergeHookResult_DecisionAsk(t *testing.T) {
	hr := HookResult{Decision: PermissionAsk}
	got := mergeHookResult(hr)
	if !got.Deny {
		t.Error("ask should fall back to deny in non-interactive mode")
	}
	if got.DenyReason != "requires user approval (non-interactive mode)" {
		t.Errorf("expected ask reason, got %q", got.DenyReason)
	}
}

func TestMergeHookResult_UpdatedInput(t *testing.T) {
	input := map[string]any{"key": "val"}
	hr := HookResult{UpdatedInput: input}
	got := mergeHookResult(hr)
	if got.UpdatedInput["key"] != "val" {
		t.Error("expected UpdatedInput to be preserved")
	}
}

// --- ToolPermissions.IsToolAllowed ---

func TestIsToolAllowed_NilPerms(t *testing.T) {
	var tp *ToolPermissions
	if !tp.IsToolAllowed("anything") {
		t.Error("nil perms should allow all tools")
	}
}

func TestIsToolAllowed_DeniedTakesPrecedence(t *testing.T) {
	tp := &ToolPermissions{
		AllowedTools: []string{"bash"},
		DeniedTools:  []string{"bash"},
	}
	if tp.IsToolAllowed("bash") {
		t.Error("denied should take precedence over allowed")
	}
}

func TestIsToolAllowed_ReadOnly(t *testing.T) {
	tp := &ToolPermissions{ReadOnly: true}
	for _, tool := range []string{"Edit", "Write", "NotebookEdit"} {
		if tp.IsToolAllowed(tool) {
			t.Errorf("read-only should block %s", tool)
		}
	}
	if !tp.IsToolAllowed("Read") {
		t.Error("read-only should allow Read")
	}
}

func TestIsToolAllowed_EmptyAllowedMeansAll(t *testing.T) {
	tp := &ToolPermissions{}
	if !tp.IsToolAllowed("anything") {
		t.Error("empty allowed tools should allow all")
	}
}

func TestIsToolAllowed_OnlyAllowedTools(t *testing.T) {
	tp := &ToolPermissions{AllowedTools: []string{"bash", "read"}}
	if !tp.IsToolAllowed("bash") {
		t.Error("expected bash to be allowed")
	}
	if tp.IsToolAllowed("write") {
		t.Error("expected write to be denied")
	}
}
