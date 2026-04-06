# Claude Code Orchestration Patterns — Applied to Alphenix

This document extracts Claude Code's core orchestration engineering patterns and maps them to
Alphenix's existing architecture. Each pattern includes the original design rationale, the
alphenix implementation status, and concrete enhancement recommendations.

## 1. System Prompt Assembly

### Claude Code Pattern

Claude Code uses a **modular section system** with aggressive prompt caching:

```
systemPromptSection(name, computeFn) → cached SystemPromptSection
resolveSystemPromptSections(sections[]) → ordered, deduplicated prompt
buildEffectiveSystemPrompt({override, coordinator, agent, custom, default, append})
  → priority-based final prompt
```

Key design decisions:
- **Static/dynamic boundary**: The STATIC section (identity, rules, tool guidance) rarely changes
  between invocations and benefits from Anthropic's prompt caching. The DYNAMIC section (issue
  context, colleague list, instructions) changes per task.
- **Priority chain**: override > coordinator > agent(proactive appends, otherwise replaces) >
  custom > default. `appendSystemPrompt` always added at end (except when override is set).
- **Memoization**: Each section is computed once and cached. Cache busts only when the underlying
  data changes (MCP connect, plugin reload, permission mode change).

### Alphenix Status

**Partially implemented**. `daemon/system_prompt.go` has:
- Static/dynamic boundary (`AssembleSystemPrompt` / `writeStaticSection`)
- Role-based tool permissions (`DefaultToolPermissions`)

**Missing**:
- No section registry or memoization
- No priority chain (single prompt, no override/coordinator/agent distinction)
- No `appendSystemPrompt` support
- No cache-aware section resolution

### Enhancement

See `server/internal/daemon/system_prompt.go` — added `PromptSection`, `PromptRegistry`,
and `AssembleWithRegistry()` for modular, cacheable prompt assembly.

---

## 2. Agent Roles & Permission Modes

### Claude Code Pattern

Six permission modes with inheritance:
- `default` — interactive approval for each tool
- `acceptEdits` — auto-approve file edits
- `auto` — auto-approve everything except destructive ops
- `bypassPermissions` — skip all permission checks
- `dontAsk` — similar to auto but for non-interactive
- `plan` — read-only, can propose but not execute

Agent permission scoping: child agent's `permissionMode` is honored UNLESS the parent is
`bypassPermissions`/`acceptEdits`/`auto` (parent's permissiveness is inherited).

### Alphenix Status

**Good foundation**. `agent.ToolPermissions` has:
- `AllowedTools`, `DeniedTools`, `ReadOnly`
- `IsToolAllowed()` with deny-first precedence
- Role-based defaults: executor (nil=all), coordinator (read+agent), reviewer (read-only)

**Missing**:
- No permission inheritance between parent/child agents
- No interactive approval flow (all-or-nothing)
- No `plan` role that proposes without executing

### Enhancement

The `ToolPermissions` struct already covers the core semantics. The main gap is permission
inheritance when spawning subagents. See the fork mode enhancement below.

---

## 3. Agent Fork Mode (Subagent with Context Inheritance)

### Claude Code Pattern

Omitting `subagent_type` creates a **fork** — a lightweight agent that:
- Inherits the parent's full conversation context
- Shares the parent's prompt cache (massive token savings)
- Uses a directive-style prompt (what to do, not what the situation is)
- Returns via `output_file` — parent must NOT read it mid-flight ("Don't peek")
- Parent must NOT fabricate fork results before notification arrives ("Don't race")

```
Agent({name: "ship-audit", prompt: "Audit what's left..."})
  → fork: inherits context, shares cache
Agent({subagent_type: "code-reviewer", prompt: "Review migration..."})
  → fresh agent: zero context, needs full briefing
```

### Alphenix Status

**Not implemented**. All agent executions start fresh with no context inheritance.

### Enhancement

See `server/pkg/agent/agent.go` — added `ForkOptions` and `ForkSession` to the `Backend`
interface, plus guidance rules for fork prompts.

---

## 4. Three-Tier Agent Model

### Claude Code Pattern

```
Main Session (user-facing REPL)
  └─ Teammates (in-process or tmux-based, persistent collaborators)
       └─ Subagents (autonomous workers, spawned per task)
```

Rules:
- **Teammates** share context via filesystem (`.claude/teams/`), cannot spawn other teammates
- **Subagents** are fully autonomous, communicate results via output files
- **Flat team hierarchy**: teammates can only spawn subagents, not other teammates
- **Mailbox-based async messaging**: file-based message passing with lock files

### Alphenix Status

**Two of three tiers implemented**:
- Main session: the Go backend acts as the orchestrator
- Subagents: daemon spawns CLI processes (claude, codex, opencode)

**Missing the teammate tier** — persistent, context-sharing collaborators that sit between
the orchestrator and subagents.

### Enhancement

The `CollaborationService` already has inter-agent messaging, DAG dependencies, and shared
context. The missing piece is a formal "teammate" abstraction. See the hook system enhancement
for lifecycle event wiring.

---

## 5. Hook System (Event-Driven Extensibility)

### Claude Code Pattern

20+ event types with three hook kinds:
- **Command hooks**: Shell commands executed on events
- **Prompt hooks**: Additional prompt text injected at lifecycle points
- **Agent hooks**: Full agent invocations triggered by events

Key events:
```
PreToolUse, PostToolUse, Notification, Stop, SubagentStart, SubagentStop,
TeammateStart, TeammateIdle, PreCompact, SessionStart, SessionEnd
```

### Alphenix Status

**Not implemented as a formal system**. The event bus (`internal/events/`) publishes domain
events but there's no hook registration/execution mechanism.

### Enhancement

See `server/internal/service/hooks.go` — added `HookService` with event-driven lifecycle
management that integrates with the existing event bus.

---

## 6. Message Priority & Flushing

### Claude Code Pattern

Three-tier priority system:
- `immediate-preempt`: errors, critical state changes — flush immediately, interrupt batch
- `normal`: tool_use, tool_result — flush on tick
- `fold`: text, thinking — accumulate and fold into batches

### Alphenix Status

**Implemented in `daemon/notification.go`**:
- `ClassifyMessage()` maps message types to priority classes
- Priority-based flushing in `daemon.go` message drain loop
- Periodic ticker (500ms) for batched messages
- Immediate flush for urgent messages (errors)

This is already well-implemented. No changes needed.

---

## 7. Collaborative Context Assembly

### Claude Code Pattern

When agents work in teams, the system prompt is enriched with:
- Colleague list (other active agents, their capabilities)
- Pending messages from other agents
- Task dependencies (DAG awareness)
- Workspace memory (past observations via semantic search)
- Last execution checkpoint (resumable state)

### Alphenix Status

**Well implemented** in `CollaborationService.BuildSharedContext()`:
- Colleagues loaded from agent list
- Pending messages via `GetPendingMessages()`
- DAG dependencies via `GetDependencyInfo()`
- Hybrid memory recall (BM25 + vector + RRF)
- Checkpoint resumption via `GetLatestCheckpoint()`

The prompt injection layer in `daemon/prompt.go`'s `appendCollaborationContext()` handles
all these signals. This is a strong implementation.

### Minor Enhancement

See the system prompt enhancement for adding collaboration context as a registered section
for better modularity.

---

## 8. Structured Execution Protocol

### Claude Code Pattern

Claude Code's system prompt enforces a strict execution lifecycle:
1. Understand before acting (read, explore, plan)
2. Make minimal, targeted changes
3. Verify work (tests, type checks, linters)
4. Communicate progress (structured status updates)

### Alphenix Status

**Implemented in `daemon/system_prompt.go`**:
- Four-phase protocol: Research → Plan → Implement → Verify
- Structured progress reporting (PLAN/PROGRESS/DONE formats)
- Progress detection via regex in `daemon/notification.go`

This mirrors Claude Code's approach well. The main improvement is making the protocol
configurable per agent role (coordinator vs executor vs reviewer).

---

## Summary: What We're Adding

| Pattern | Alphenix Status | Action |
|---|---|---|
| Modular prompt sections | Partial | Enhance with registry + caching |
| Permission modes | Good | Add inheritance + plan mode |
| Agent fork mode | None | Implement fork options + context sharing |
| Three-tier model | Partial | Add teammate abstraction concept |
| Hook system | None | Add HookService with event types |
| Message priority | Done | No changes needed |
| Collaborative context | Done | Minor modularization |
| Execution protocol | Done | Make role-configurable |
