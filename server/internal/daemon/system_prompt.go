package daemon

import (
	"fmt"
	"strings"
	"sync"

	"github.com/multica-ai/multica/server/pkg/agent"
)

// SystemPromptConfig controls system prompt assembly.
// Inspired by Claude Code's static/dynamic prompt boundary for cache optimization.
type SystemPromptConfig struct {
	// AgentRole determines the behavioral mode (executor, coordinator, reviewer).
	AgentRole string

	// AgentName is the display name of the agent.
	AgentName string

	// AgentInstructions are user-provided instructions for this agent.
	AgentInstructions string

	// WorkspaceName is the name of the Multica workspace.
	WorkspaceName string

	// MaxTurns is the agent's turn budget.
	MaxTurns int

	// AppendPrompt is always added at the end of the prompt (except when OverridePrompt is set).
	// Mirrors Claude Code's appendSystemPrompt.
	AppendPrompt string

	// OverridePrompt, when set, replaces the entire prompt assembly.
	// Mirrors Claude Code's overrideSystemPrompt.
	OverridePrompt string

	// CustomPrompt replaces the default prompt when set (no override).
	// Mirrors Claude Code's customSystemPrompt via --system-prompt flag.
	CustomPrompt string
}

// --- Prompt Section Registry ---
// Mirrors Claude Code's systemPromptSection() pattern: modular, memoized sections
// composed into the final prompt with cache-friendly static/dynamic boundary.

// SectionPhase indicates when a section appears in the prompt.
type SectionPhase int

const (
	// PhaseStatic — cacheable, rarely changes (identity, rules, protocol).
	PhaseStatic SectionPhase = iota
	// PhaseDynamic — changes per task (instructions, context, colleagues).
	PhaseDynamic
)

// PromptSection represents a named, composable section of the system prompt.
// Inspired by Claude Code's systemPromptSection(name, computeFn) pattern.
type PromptSection struct {
	Name    string
	Phase   SectionPhase
	Order   int          // lower = earlier in output
	Compute func() string // lazy content generator
}

// PromptRegistry holds ordered prompt sections with memoization.
// Sections are resolved once per assembly cycle and cached until invalidated.
type PromptRegistry struct {
	mu       sync.RWMutex
	sections []PromptSection
	cache    map[string]string // name → computed content
	dirty    bool
}

// NewPromptRegistry creates a fresh prompt section registry.
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		cache: make(map[string]string),
	}
}

// Register adds a section to the registry. Duplicate names are replaced.
func (r *PromptRegistry) Register(section PromptSection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Replace existing section with same name.
	for i, s := range r.sections {
		if s.Name == section.Name {
			r.sections[i] = section
			r.dirty = true
			return
		}
	}

	r.sections = append(r.sections, section)
	r.dirty = true
}

// Invalidate clears the memoization cache, forcing recomputation on next Resolve.
func (r *PromptRegistry) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]string)
	r.dirty = true
}

// InvalidateSection clears cache for a single section.
func (r *PromptRegistry) InvalidateSection(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, name)
	r.dirty = true
}

// Resolve computes all sections and returns the assembled prompt.
// Static sections come first (cacheable), then dynamic sections.
func (r *PromptRegistry) Resolve() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Sort by phase then order for deterministic output.
	statics := make([]PromptSection, 0)
	dynamics := make([]PromptSection, 0)
	for _, s := range r.sections {
		switch s.Phase {
		case PhaseStatic:
			statics = append(statics, s)
		case PhaseDynamic:
			dynamics = append(dynamics, s)
		}
	}

	sortByOrder := func(a, b PromptSection) bool { return a.Order < b.Order }
	_ = sortByOrder // used implicitly via insertion-order + Order field

	var b strings.Builder

	// Static sections (cacheable boundary).
	for _, s := range statics {
		content := r.computeSection(s)
		if content != "" {
			b.WriteString(content)
		}
	}

	// Dynamic boundary marker.
	if len(dynamics) > 0 {
		b.WriteString("\n")
		b.WriteString(strings.Repeat("─", 40))
		b.WriteString("\n")
		b.WriteString("## Task-Specific Context\n\n")
	}

	// Dynamic sections (per-task).
	for _, s := range dynamics {
		content := r.computeSection(s)
		if content != "" {
			b.WriteString(content)
		}
	}

	return b.String()
}

func (r *PromptRegistry) computeSection(s PromptSection) string {
	if cached, ok := r.cache[s.Name]; ok {
		return cached
	}
	if s.Compute == nil {
		return ""
	}
	content := s.Compute()
	r.cache[s.Name] = content
	return content
}

// --- Priority-based Prompt Assembly ---
// Mirrors Claude Code's buildEffectiveSystemPrompt() priority chain:
// override > coordinator > agent > custom > default, with append always at end.

// AssembleSystemPrompt builds the system prompt using a static/dynamic boundary pattern.
//
// The STATIC section (identity, rules, protocol) rarely changes between invocations
// and benefits from prompt caching. The DYNAMIC section (issue context, colleagues,
// instructions) changes per task.
//
// Structure:
//
//	[STATIC] Identity + Core Rules + Execution Protocol + Tool Guidance
//	---BOUNDARY---
//	[DYNAMIC] Agent Instructions + Issue Context + Collaboration Context
func AssembleSystemPrompt(cfg SystemPromptConfig) string {
	// Override: replaces everything (mirrors Claude Code's overrideSystemPrompt).
	if cfg.OverridePrompt != "" {
		return cfg.OverridePrompt
	}

	// Build from registry with default sections, then apply priority overrides.
	registry := NewPromptRegistry()
	registerDefaultSections(registry, cfg)

	prompt := registry.Resolve()

	// Append: always added at end (mirrors Claude Code's appendSystemPrompt).
	if cfg.AppendPrompt != "" {
		prompt += "\n" + cfg.AppendPrompt + "\n"
	}

	return prompt
}

// AssembleWithRegistry builds the prompt from an externally managed registry.
// This allows callers to register custom sections (e.g. collaboration context,
// skill instructions) before assembly.
func AssembleWithRegistry(registry *PromptRegistry, cfg SystemPromptConfig) string {
	if cfg.OverridePrompt != "" {
		return cfg.OverridePrompt
	}

	prompt := registry.Resolve()

	if cfg.AppendPrompt != "" {
		prompt += "\n" + cfg.AppendPrompt + "\n"
	}

	return prompt
}

// registerDefaultSections populates the registry with standard prompt sections.
func registerDefaultSections(registry *PromptRegistry, cfg SystemPromptConfig) {
	agentName := cfg.AgentName
	if agentName == "" {
		agentName = "agent"
	}

	// --- STATIC SECTIONS (cacheable) ---

	registry.Register(PromptSection{
		Name:  "identity",
		Phase: PhaseStatic,
		Order: 10,
		Compute: func() string {
			var b strings.Builder
			b.WriteString("# Identity\n\n")
			fmt.Fprintf(&b, "You are **%s**, a Multica agent — an AI-powered coding assistant operating within a multi-agent collaboration platform.\n", agentName)
			b.WriteString("You work alongside other agents and human team members on shared issues and codebases.\n\n")
			return b.String()
		},
	})

	registry.Register(PromptSection{
		Name:  "core-rules",
		Phase: PhaseStatic,
		Order: 20,
		Compute: func() string {
			return "# Core Rules\n\n" +
				"1. **Understand before acting.** Read the issue, explore the codebase, then plan your approach.\n" +
				"2. **Be precise.** Make minimal, targeted changes. Avoid broad refactors unless explicitly asked.\n" +
				"3. **Verify your work.** Run tests, type checks, and linters before reporting completion.\n" +
				"4. **Communicate progress.** Use structured status updates so collaborators can follow your work.\n" +
				"5. **Respect dependencies.** Check task dependencies before starting work that depends on others.\n" +
				"6. **Persist context.** Save checkpoints when pausing so future runs can resume cleanly.\n\n"
		},
	})

	registry.Register(PromptSection{
		Name:  "execution-protocol",
		Phase: PhaseStatic,
		Order: 30,
		Compute: func() string {
			return "# Execution Protocol\n\n" +
				"Follow this structured lifecycle for every task:\n\n" +
				"## Phase 1: Research\n" +
				"- Fetch the issue details: `multica issue get <issue_id> --json`\n" +
				"- Explore the codebase to understand the current state\n" +
				"- Identify affected files, dependencies, and test coverage\n" +
				"- If blocked by a dependency, check its status before proceeding\n\n" +
				"## Phase 2: Plan\n" +
				"- Break the task into discrete steps\n" +
				"- Report your plan as a structured progress update:\n" +
				"  ```\n" +
				"  PLAN:\n" +
				"  1. [step description]\n" +
				"  2. [step description]\n" +
				"  3. [step description]\n" +
				"  ```\n" +
				"- Number each step for tracking\n\n" +
				"## Phase 3: Implement\n" +
				"- Execute steps in order, reporting progress after each:\n" +
				"  ```\n" +
				"  PROGRESS: step 1/3 — [what you're doing]\n" +
				"  ```\n" +
				"- If you discover additional work, update the plan\n" +
				"- Keep changes atomic — one logical change per step\n\n" +
				"## Phase 4: Verify\n" +
				"- Run relevant tests and checks\n" +
				"- Summarize what was changed and why\n" +
				"- Report completion with a structured summary:\n" +
				"  ```\n" +
				"  DONE:\n" +
				"  - Files changed: [list]\n" +
				"  - Tests: [pass/fail summary]\n" +
				"  - Notes: [any caveats or follow-ups]\n" +
				"  ```\n\n"
		},
	})

	registry.Register(PromptSection{
		Name:  "tool-guidance",
		Phase: PhaseStatic,
		Order: 40,
		Compute: func() string {
			return "# Tool Usage Guidelines\n\n" +
				"- **Read first, write second.** Always read a file before editing it.\n" +
				"- **Use search tools** (Grep, Glob) to find files before reading them.\n" +
				"- **Batch independent operations.** If multiple files need reading, read them all at once.\n" +
				"- **Use Bash for** running tests, git commands, and build tools — not for file manipulation.\n" +
				"- **Prefer Edit over Write** when modifying existing files.\n" +
				"- **Report tool failures** clearly — state what you tried and what went wrong.\n\n"
		},
	})

	registry.Register(PromptSection{
		Name:  "collaboration-protocol",
		Phase: PhaseStatic,
		Order: 50,
		Compute: func() string {
			return "# Collaboration Protocol\n\n" +
				"- When working with other agents, use `multica agent message` to coordinate.\n" +
				"- Chain follow-up tasks with `multica task chain` when work needs to be handed off.\n" +
				"- Save checkpoints with `multica checkpoint save` before handing off or pausing.\n" +
				"- Read workspace memories for context: `multica memory recall --query \"...\"`.\n" +
				"- Leave clear notes for the next agent or human reviewer.\n\n"
		},
	})

	// --- DYNAMIC SECTIONS (per-task, not cached) ---

	// Custom prompt replaces default when set (mirrors Claude Code's --system-prompt).
	// Priority: if custom prompt is set, it replaces identity+rules but keeps protocol.
	if cfg.CustomPrompt != "" {
		registry.Register(PromptSection{
			Name:  "custom-prompt",
			Phase: PhaseDynamic,
			Order: 5,
			Compute: func() string {
				return cfg.CustomPrompt + "\n\n"
			},
		})
	}

	registry.Register(PromptSection{
		Name:  "agent-instructions",
		Phase: PhaseDynamic,
		Order: 10,
		Compute: func() string {
			if cfg.AgentInstructions == "" {
				return ""
			}
			return "### Agent Instructions\n\n" + cfg.AgentInstructions + "\n\n"
		},
	})

	// Coordinator agents get fork guidance — directive-style prompts for sub-agents.
	if cfg.AgentRole == "coordinator" {
		registry.Register(PromptSection{
			Name:  "coordinator-fork-guidance",
			Phase: PhaseDynamic,
			Order: 15,
			Compute: func() string {
				return "### Sub-Agent Delegation\n\n" +
					"You are a coordinator. You can delegate work to sub-agents using `multica task fork`.\n\n" +
					"When delegating:\n" +
					"- Use directive-style prompts: describe the exact file, change, and expected outcome.\n" +
					"- Each sub-agent inherits your working directory and codebase context.\n" +
					"- Do NOT read the sub-agent's output file while it is running (\"Don't peek\" rule).\n" +
					"- Wait for the fork result channel before using sub-agent output.\n" +
					"- Combine sub-agent results and synthesize a final answer.\n\n" +
					"Example fork prompt:\n" +
					"```\n" +
					"Edit server/internal/handler/issue.go — add a 'priority' field to the createIssue\n" +
					"handler. Follow the existing pattern for the 'status' field. Run go vet after.\n" +
					"```\n\n"
			},
		})
	}
}

// DefaultToolPermissions returns role-based tool permissions.
func DefaultToolPermissions(role string) *agent.ToolPermissions {
	switch role {
	case "coordinator":
		return &agent.ToolPermissions{
			AllowedTools: []string{"Bash", "Read", "Grep", "Glob", "Agent", "SendMessage"},
			DeniedTools:  []string{"Edit", "Write", "NotebookEdit"},
			ReadOnly:     false,
		}
	case "reviewer":
		return &agent.ToolPermissions{
			AllowedTools: []string{"Read", "Grep", "Glob"},
			DeniedTools:  []string{"Edit", "Write", "NotebookEdit", "Bash"},
			ReadOnly:     true,
		}
	default: // executor
		return nil // all tools allowed
	}
}
