package agent

import (
	"context"
	"fmt"
)

// HookEntry is a single hook in a HookChain. It has an optional Matcher
// that filters which tools the hook applies to, and a Run function that
// executes the hook logic.
type HookEntry struct {
	// Matcher, if non-nil, filters which tools this hook applies to.
	// If nil, the hook applies to all tools.
	Matcher *HookMatcher

	// Run executes the hook. For PreToolUse hooks, it returns a ToolHookResult.
	// For PostToolUse hooks, the return value is ignored.
	Run func(ctx context.Context, tool string, input map[string]any, output string) ToolHookResult
}

// HookMatcher filters hooks by tool name.
type HookMatcher struct {
	// Tools lists tool names this hook applies to. If empty, matches all tools.
	Tools []string

	// Negate when true inverts the match — the hook applies to all tools
	// EXCEPT those listed in Tools.
	Negate bool
}

// Matches returns true if this matcher applies to the given tool name.
func (m *HookMatcher) Matches(tool string) bool {
	if m == nil || len(m.Tools) == 0 {
		return true
	}
	found := false
	for _, t := range m.Tools {
		if t == tool {
			found = true
			break
		}
	}
	if m.Negate {
		return !found
	}
	return found
}

// HookChain replaces the single-callback ToolHooks with an ordered list of
// hook entries. Each entry can be filtered by tool name via HookMatcher.
// For PreToolUse hooks, the first deny result wins (deny-first-wins semantics).
type HookChain struct {
	PreToolUse  []HookEntry
	PostToolUse []HookEntry
}

// RunPreToolUse executes the PreToolUse chain. Returns the first deny result,
// or allow if no hook denies. If the chain is empty, returns allow.
func (hc *HookChain) RunPreToolUse(ctx context.Context, tool string, input map[string]any) ToolHookResult {
	if hc == nil {
		return ToolHookResult{}
	}
	for _, entry := range hc.PreToolUse {
		if entry.Matcher != nil && !entry.Matcher.Matches(tool) {
			continue
		}
		result := entry.Run(ctx, tool, input, "")
		if result.Deny {
			return result
		}
		if result.UpdatedInput != nil {
			input = result.UpdatedInput
		}
	}
	return ToolHookResult{}
}

// RunPostToolUse executes the PostToolUse chain. All matching entries run;
// return values are ignored.
func (hc *HookChain) RunPostToolUse(ctx context.Context, tool string, input map[string]any, output string) {
	if hc == nil {
		return
	}
	for _, entry := range hc.PostToolUse {
		if entry.Matcher != nil && !entry.Matcher.Matches(tool) {
			continue
		}
		entry.Run(ctx, tool, input, output)
	}
}

// ToHookChain converts the legacy single-callback ToolHooks into a HookChain.
// This provides backward compatibility — existing code using ToolHooks continues
// to work unchanged.
func (th ToolHooks) ToHookChain() *HookChain {
	hc := &HookChain{}
	if th.PreToolUse != nil {
		hc.PreToolUse = append(hc.PreToolUse, HookEntry{
			Run: func(ctx context.Context, tool string, input map[string]any, _ string) ToolHookResult {
				return th.PreToolUse(ctx, tool, input)
			},
		})
	}
	if th.PostToolUse != nil {
		hc.PostToolUse = append(hc.PostToolUse, HookEntry{
			Run: func(ctx context.Context, tool string, input map[string]any, output string) ToolHookResult {
				th.PostToolUse(ctx, tool, input, output)
				return ToolHookResult{}
			},
		})
	}
	return hc
}

// PermissionDecision represents an explicit permission decision from a hook.
type PermissionDecision string

const (
	// PermissionApprove allows the tool call to proceed.
	PermissionApprove PermissionDecision = "approve"
	// PermissionDeny blocks the tool call.
	PermissionDeny PermissionDecision = "deny"
	// PermissionAsk prompts the user for a decision (falls back to deny in non-interactive mode).
	PermissionAsk PermissionDecision = "ask"
)

// HookResult is the enriched return value from a hook, supporting explicit
// permission decisions and additional context injection.
type HookResult struct {
	// Deny blocks the tool call (backward-compatible with ToolHookResult).
	Deny bool

	// DenyReason is the error message shown to the agent when Deny is true.
	DenyReason string

	// UpdatedInput, if non-nil, replaces the tool's input parameters.
	UpdatedInput map[string]any

	// Decision is an explicit permission decision from the hook.
	// Takes precedence over Deny if set.
	Decision PermissionDecision

	// AdditionalContext is extra text injected into the tool result for the agent.
	AdditionalContext string
}

// ResolvePermission combines hook decisions with ToolPermissions to produce
// a final allow/deny decision. Deny rules always win over hook approval,
// matching Claude Code's permission resolution semantics.
//
// Resolution order:
//  1. ToolPermissions check (deny takes precedence)
//  2. Hook chain PreToolUse (first deny wins)
//  3. Default: allow
func ResolvePermission(ctx context.Context, tool string, input map[string]any, perms *ToolPermissions, chain *HookChain) ToolHookResult {
	// Step 1: ToolPermissions check.
	if perms != nil && !perms.IsToolAllowed(tool) {
		return ToolHookResult{
			Deny:       true,
			DenyReason: fmt.Sprintf("tool %q is not allowed for this agent role", tool),
		}
	}

	// Step 2: Hook chain PreToolUse.
	if chain != nil {
		result := chain.RunPreToolUse(ctx, tool, input)
		if result.Deny {
			return result
		}
		// If hook updated input, pass it through.
		return result
	}

	// Step 3: Default allow.
	return ToolHookResult{}
}

// mergeHookResult converts an enriched HookResult back to a ToolHookResult
// for backward compatibility. If Decision is set, it takes precedence over Deny.
func mergeHookResult(hr HookResult) ToolHookResult {
	deny := hr.Deny
	reason := hr.DenyReason

	switch hr.Decision {
	case PermissionDeny:
		deny = true
		if reason == "" {
			reason = "denied by hook"
		}
	case PermissionApprove:
		deny = false
		reason = ""
	case PermissionAsk:
		// In non-interactive mode, ask falls back to deny.
		deny = true
		if reason == "" {
			reason = "requires user approval (non-interactive mode)"
		}
	}

	return ToolHookResult{
		Deny:          deny,
		DenyReason:    reason,
		UpdatedInput:  hr.UpdatedInput,
	}
}
