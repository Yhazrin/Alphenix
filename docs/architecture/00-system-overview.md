# System Overview

## Project Identity

Alphenix is an **AI-native task management platform** — a Linear-like issue tracker where AI agents are first-class participants. Agents can be assigned issues, create issues, comment, change status, and execute code tasks autonomously via local daemon runtimes.

**Target audience**: 2–10 person engineering teams that want to delegate real coding work to AI agents within a structured, observable workflow.

**Stage**: Early product (pre-production). Core issue/agent/task/runtime loop is functional. Advanced features (teams, MCP, decompose, fork orchestration) are partially implemented.

## Core Object Model

```
Workspace ──┬── Member (human user with role)
             ├── Agent (AI worker with instructions, runtime, skills)
             ├── Issue (work item: goal or task)
             ├── Team (agent group with lead)
             └── WorkspaceRepo (git repository binding)

Agent ────── AgentRuntime (execution environment: daemon instance)
         └── Skill (reusable instruction set with files)

Issue ───┬── Comment (threaded, by member or agent)
         ├── AgentTaskQueue (queued work unit for agent execution)
         ├── InboxItem (notification for subscribers)
         ├── IssueDependency (blocks/blocked_by/related)
         └── Attachment (uploaded files)

AgentTaskQueue ──┬── Run (execution record with phases and steps)
                 ├── TaskMessage (progress messages from daemon)
                 ├── TaskCheckpoint (resumption state)
                 ├── TaskReview (automated/manual review verdict)
                 └── TaskDependency (inter-task ordering)

Run ──┬── RunStep (thinking/text/tool_use/tool_result/error)
      ├── RunTodo (agent's task breakdown)
      ├── RunArtifact (produced files/reports)
      ├── RunEvent (persisted event stream for replay)
      ├── RunHandoff (delegation to another run/team/agent)
      └── RunContinuation (compaction/resumption packet)
```

## System Layers

| Layer | Technology | Location |
|-------|-----------|----------|
| **Frontend** | Next.js 16 (App Router), React 19, Zustand, Tailwind 4, shadcn/ui | `apps/web/` |
| **API** | Go, Chi v5 router, JWT auth (HS256) | `server/cmd/server/`, `server/internal/handler/` |
| **Services** | TaskService, RunOrchestrator, ReviewService, CollaborationService | `server/internal/service/` |
| **Agent SDK** | Backend interface (Claude Code, Codex, OpenCode), session/fork management | `server/pkg/agent/` |
| **Daemon** | Local runtime process, CLI polling, repo cache, execution environment | `server/internal/daemon/` |
| **Real-time** | WebSocket hub (per-workspace rooms), event bus (sync pub/sub) | `server/internal/realtime/`, `server/internal/events/` |
| **Database** | PostgreSQL 17 + pgvector, sqlc code generation | `server/migrations/`, `server/pkg/db/` |
| **CLI** | Cobra-based `alphenix` binary for agents and humans | `server/cmd/alphenix/` |

## Primary Execution Path

```
Human creates Issue → assigns Agent
  → TaskService.EnqueueTaskForIssue()
    → SelectRuntime() (policy-based or agent default)
    → Creates AgentTaskQueue row (status: queued)
  → Daemon polls /api/daemon/runtimes/{id}/tasks/claim
    → ClaimTask() (FOR UPDATE SKIP LOCKED, respects max_concurrent_tasks)
    → Task → dispatched → running
  → Daemon assembles prompt (system_prompt + skills + issue context)
    → Backend.Execute() (Claude Code / Codex CLI)
    → RunOrchestrator records steps, todos, artifacts
    → Coalescer + StepCoalescer reduce DB writes
    → Compactor summarizes long conversations
  → Task completes → ReviewService (if max_reviews > 0) → completed/failed
    → Agent comment posted on issue
    → ReconcileAgentStatus()
    → WS broadcast to all workspace clients
```

## Current Capabilities (verified in code)

1. **Issue management**: Full CRUD with board/list views, priority, status, assignee (member/agent/team), parent-child, dependencies, reactions, subscriptions
2. **Agent lifecycle**: Create, configure (instructions, runtime, skills, tools, triggers), archive/restore, status reconciliation
3. **Task execution**: Enqueue → claim → start → complete/fail/cancel with full state machine enforcement
4. **Runtime selection**: Policy-based selection with tags, preferred/fallback lists, load scoring, queue depth limits
5. **Run recording**: Phase-based lifecycle, step-by-step recording, todos, artifacts, events, compaction, continuations, handoffs
6. **Prompt assembly**: Modular registry with static/dynamic sections, cache-friendly boundary, skill injection, role-based permissions
7. **Real-time sync**: WebSocket per-workspace broadcast, frontend WS event system, optimistic updates with rollback
8. **Review system**: Automated review after task completion, verdict-based retry/pass/fail
9. **Collaboration**: Agent messaging, task chaining, checkpoints, memory (pgvector), task dependencies
10. **Authentication**: Email + verification code, JWT, PAT (mul_ prefix), daemon tokens (mdt_ prefix)

## Architecture Risks

1. **Migration 042 conflict**: Two different migrations share version 042 — will break `goose` in fresh environments
2. **Dead code paths**: `decompose.go` and `fork.go` handlers exist but have no route registration — unreachable features
3. **DaemonAuth unused**: `middleware.DaemonAuth` (mdt_ prefix) is defined but daemon routes use `middleware.Auth` (JWT/PAT) — the dedicated daemon auth path is dead code
4. **Schema/code drift**: `agent_memory.sql` was deleted from queries/ but `agent_memory.sql.go` still exists in generated code
5. **NoOpPublisher**: Outbox persists all events but external publishing is a no-op — outbox table grows unboundedly
6. **Frontend search modal**: Type system allows "search" modal but ModalRegistry has no implementation — keyboard shortcut `/` and sidebar trigger a no-op
7. **Issue list cap**: Frontend fetches max 200 issues with no pagination — breaks for active workspaces
8. **No run phase validation**: RunOrchestrator.AdvancePhase() accepts any phase string without validating allowed transitions
