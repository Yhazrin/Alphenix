package daemon

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/multica-ai/alphenix/server/pkg/agent"
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

	// WorkspaceName is the name of the Alphenix workspace.
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

// ContentHash returns a stable hash of the section's computed content.
// Used for cache key generation and change detection.
func (s PromptSection) ContentHash() string {
	if s.Compute == nil {
		return ""
	}
	content := s.Compute()
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

// PromptRegistry holds ordered prompt sections with memoization.
// Sections are resolved once per assembly cycle and cached until invalidated.
type PromptRegistry struct {
	mu          sync.RWMutex
	sections    []PromptSection
	cache       map[string]string // dynamic sections: per-cycle cache
	staticCache map[string]string // static sections: persistent cache, cleared only on explicit invalidation
	dirty       bool
}

// NewPromptRegistry creates a fresh prompt section registry.
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		cache:       make(map[string]string),
		staticCache: make(map[string]string),
	}
}

// sharedRegistry is the process-wide prompt registry used by the server.
// It enables external callers (e.g. toolRegistry.OnChange) to invalidate
// cached static sections when tool definitions change.
var (
	sharedRegistry   *PromptRegistry
	sharedRegistryMu sync.Mutex
)

// SharedRegistry returns the process-wide PromptRegistry, creating it on first call.
func SharedRegistry() *PromptRegistry {
	sharedRegistryMu.Lock()
	defer sharedRegistryMu.Unlock()
	if sharedRegistry == nil {
		sharedRegistry = NewPromptRegistry()
	}
	return sharedRegistry
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

// Invalidate clears both static and dynamic caches, forcing full recomputation on next Resolve.
func (r *PromptRegistry) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]string)
	r.staticCache = make(map[string]string)
	r.dirty = true
}

// InvalidateStatic clears only the static cache, leaving dynamic cache intact.
// Use this when a static section's content has changed (e.g. tool registry update)
// but dynamic sections don't need recomputation.
func (r *PromptRegistry) InvalidateStatic() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.staticCache = make(map[string]string)
	r.dirty = true
}

// InvalidateSection clears cache for a single section in both caches.
func (r *PromptRegistry) InvalidateSection(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, name)
	delete(r.staticCache, name)
	r.dirty = true
}

// Resolve computes all sections and returns the assembled prompt.
// Static sections come first (persistent cache), then dynamic sections (per-cycle cache).
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

	sort.Slice(statics, func(i, j int) bool { return statics[i].Order < statics[j].Order })
	sort.Slice(dynamics, func(i, j int) bool { return dynamics[i].Order < dynamics[j].Order })

	var b strings.Builder

	// Static sections (persistent cache — survives across Resolve calls).
	for _, s := range statics {
		content := r.computeStaticSection(s)
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

	// Dynamic sections (per-cycle cache — cleared each assembly).
	for _, s := range dynamics {
		content := r.computeDynamicSection(s)
		if content != "" {
			b.WriteString(content)
		}
	}

	return b.String()
}

// ExportedSection holds resolved section metadata for preview/UI purposes.
type ExportedSection struct {
	Name    string `json:"name"`
	Phase   string `json:"phase"` // "static" or "dynamic"
	Order   int    `json:"order"`
	Content string `json:"content"`
}

// ExportSections resolves all registered sections and returns them individually.
// Used by handler preview endpoints to expose section-level breakdown.
func (r *PromptRegistry) ExportSections() []ExportedSection {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]ExportedSection, 0, len(r.sections))
	for _, s := range r.sections {
		var content string
		if s.Phase == PhaseStatic {
			content = r.computeStaticSection(s)
		} else {
			content = r.computeDynamicSection(s)
		}
		phase := "static"
		if s.Phase == PhaseDynamic {
			phase = "dynamic"
		}
		result = append(result, ExportedSection{
			Name:    s.Name,
			Phase:   phase,
			Order:   s.Order,
			Content: content,
		})
	}
	return result
}

func (r *PromptRegistry) computeStaticSection(s PromptSection) string {
	if cached, ok := r.staticCache[s.Name]; ok {
		return cached
	}
	if s.Compute == nil {
		return ""
	}
	content := s.Compute()
	r.staticCache[s.Name] = content
	return content
}

func (r *PromptRegistry) computeDynamicSection(s PromptSection) string {
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

// RegisterDefaultSectionsForPreview populates the registry with standard prompt sections.
// This is the exported version of registerDefaultSections for use by handler preview endpoints.
func RegisterDefaultSectionsForPreview(registry *PromptRegistry, cfg SystemPromptConfig) {
	registerDefaultSections(registry, cfg)
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
			fmt.Fprintf(&b, "You are **%s**.\n\n", agentName)
			b.WriteString("Think of yourself as a colleague who is always available, accumulates knowledge over time, and develops expertise through interactions.\n\n")
			if cfg.WorkspaceName != "" {
				fmt.Fprintf(&b, "You are working in the **%s** workspace.\n", cfg.WorkspaceName)
			}
			b.WriteString("You work alongside other agents and human team members on shared issues and codebases.\n")
			b.WriteString("Use the memory system to persist what you learn — your memory should be able to bootstrap your full context if your conversation is lost.\n\n")
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
		Name:  "critical-rules",
		Phase: PhaseStatic,
		Order: 25,
		Compute: func() string {
			return "# Critical Rules\n\n" +
				"1. **Check memory FIRST** before starting any work — recall past decisions and context.\n" +
				"2. **CLAIM tasks** before starting work on them — if fulfilling a message requires action beyond just replying (running tools, writing code, making changes), claim it first.\n" +
				"3. **Update status in real-time** so the team knows your progress at every step.\n" +
				"4. **Report BLOCKERs immediately** — don't silently stall; escalate if you're stuck.\n" +
				"5. **Complete all assigned tasks** — don't stop after one action; keep going until done or blocked.\n" +
				"6. **Store important findings in memory** immediately — if you learn something useful, save it NOW.\n\n"
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
				"- Fetch the issue details: `alphenix issue get <issue_id> --json`\n" +
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
				"  ```\n" +
				"- Call `mcp__chat__update_task_status` to mark the task as done (status: \"done\").\n\n"
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
				"- When working with other agents, use `alphenix agent message` to coordinate.\n" +
				"- Chain follow-up tasks with `alphenix task chain` when work needs to be handed off.\n" +
				"- Save checkpoints with `alphenix checkpoint save` before handing off or pausing.\n" +
				"- Read workspace memories for context: `alphenix memory recall --query \"...\"`.\n" +
				"- Leave clear notes for the next agent or human reviewer.\n\n"
		},
	})

	registry.Register(PromptSection{
		Name:  "conversation-etiquette",
		Phase: PhaseStatic,
		Order: 52,
		Compute: func() string {
			return "# Conversation Etiquette\n\n" +
				"- Respect ongoing conversations — don't interject unless you are mentioned or have a direct contribution.\n" +
				"- Only the agent doing the work should report on it.\n" +
				"- Before stopping, check for concrete blockers you own and report them.\n" +
				"- Skip idle narration — don't announce what you're about to do, just do it.\n\n"
		},
	})

	registry.Register(PromptSection{
		Name:  "startup-sequence",
		Phase: PhaseStatic,
		Order: 55,
		Compute: func() string {
			return "# Startup Sequence\n\n" +
				"Follow this sequence every time you start a new task or resume work:\n\n" +
				"1. **Acknowledge** — Send a brief status update that you're starting work.\n" +
				"2. **Review Context** — Read the shared context above (colleagues, pending messages, dependencies) before proceeding.\n" +
				"3. **Check Memory** — Search your memory for relevant past decisions and context.\n" +
				"4. **Process** — Work through your assigned tasks in priority order.\n" +
				"5. **Complete All Work** — Don't stop after one action. Keep going until all tasks are done or you're blocked.\n\n"
		},
	})

	registry.Register(PromptSection{
		Name:  "formatting-rules",
		Phase: PhaseStatic,
		Order: 58,
		Compute: func() string {
			return "# Formatting Rules\n\n" +
				"- Use plain text only. No HTML tags or markup.\n" +
				"- Use plain-text @mentions without backticks (e.g. @agent-name, not `@agent-name`).\n" +
				"- When a URL appears next to non-ASCII punctuation, wrap it in angle brackets or use a markdown link.\n" +
				"- Keep messages concise and scannable. Use short paragraphs and bullet lists.\n\n"
		},
	})

	registry.Register(PromptSection{
		Name:  "compaction-safety",
		Phase: PhaseStatic,
		Order: 60,
		Compute: func() string {
			return "# Compaction Safety\n\n" +
				"IMPORTANT: Your conversation may be compacted (truncated) at any time.\n" +
				"When this happens, only shared context and memory system survive.\n\n" +
				"Therefore:\n" +
				"- Never rely on conversation history alone\n" +
				"- Write down important decisions immediately using memory tools\n" +
				"- If you learn something useful, store it NOW\n" +
				"- Your memory should be able to bootstrap your full context\n\n"
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
					"You are a coordinator. You can delegate work to sub-agents using `alphenix task fork`.\n\n" +
					"When delegating:\n" +
					"- Use directive-style prompts: describe the exact file, change, and expected outcome.\n" +
					"- Each sub-agent inherits your working directory and codebase context.\n" +
					"- Do NOT read the sub-agent's output file while it is running (\"Don't peek\" rule).\n" +
					"- Wait for the fork result channel before using sub-agent output.\n" +
					"- Combine sub-agent results and synthesize a final answer.\n\n" +
					"**Multi-Fork (Parallel Delegation):**\n" +
					"- You can launch multiple sub-agents in parallel for independent tasks.\n" +
					"- Each sub-agent runs in an isolated worktree — they do not share files during execution.\n" +
					"- After all forks complete, their results are aggregated into a summary for you.\n" +
					"- Use multi-fork when tasks are independent and can run concurrently.\n" +
					"- Example: fork 1 analyzes tests, fork 2 reviews code, fork 3 checks docs — all in parallel.\n\n" +
					"Example single fork prompt:\n" +
					"```\n" +
					"Edit server/internal/handler/issue.go — add a 'priority' field to the createIssue\n" +
					"handler. Follow the existing pattern for the 'status' field. Run go vet after.\n" +
					"```\n\n" +
					"Example multi-fork pattern:\n" +
					"```\n" +
					"Fork 1: Run all unit tests and report failures.\n" +
					"Fork 2: Review server/internal/handler/ for error handling gaps.\n" +
					"Fork 3: Check that all API endpoints have matching OpenAPI docs.\n" +
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
