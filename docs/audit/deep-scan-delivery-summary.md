# Deep Scan Delivery Summary

## 1. Overall Assessment

Alphenix is a **well-architected early-stage product** with a solid foundation for AI-native task management. The core loop — issue creation → agent assignment → task enqueue → runtime selection → daemon execution → run recording → result delivery — is **fully implemented and functional end-to-end**.

The codebase demonstrates strong engineering judgment in several critical areas: the task state machine with explicit transition validation, the multi-factor runtime selection algorithm, the modular prompt assembly system with cache optimization, and the run observability stack (steps + coalescers + compaction + events). These are not minimum-viable implementations — they're production-grade designs.

However, several features exist in various states of partial completion, and some architectural decisions have accumulated technical debt that should be addressed before the platform scales.

## 2. Project Stage

**Current stage: Late Prototype / Early Alpha**

The product is past the "does it work at all?" stage but not yet at the "can we ship to real users?" stage. Specifically:

- **Core workflows are complete**: Issue CRUD, agent management, task lifecycle, runtime pool, run recording, real-time updates
- **Advanced features are partially built**: Goal decomposition (backend done, routes were missing), fork orchestration (backend done, routes were missing), team delegation (tables and routes exist, workflow incomplete), MCP integration (CRUD done, execution coupling loose)
- **Infrastructure is solid**: CI pipeline covers lint + typecheck + unit + e2e, Docker multi-stage build works, worktree isolation supports parallel development
- **UX gaps exist**: No search, 200-issue limit, no WebSocket connection indicator, missing error boundaries

## 3. Documents Created

### Architecture (docs/architecture/)
| File | Content |
|------|---------|
| `00-system-overview.md` | Project identity, core object model, system layers, primary execution path, current capabilities, architecture risks |
| `01-module-map.md` | Full repository directory map, backend and frontend module breakdown, coupling analysis, separation candidates |
| `02-domain-model.md` | Entity descriptions, state machines, relationship diagram, status design assessment |

### Flows (docs/flows/)
| File | Content |
|------|---------|
| `task-run-lifecycle.md` | Task creation (3 paths), claiming, execution, completion (with/without review), failure, cancellation, retry. Run lifecycle with full ExecuteRun loop detail |
| `runtime-and-scheduling.md` | Runtime registration, heartbeat, sweeping, SelectRuntime algorithm with scoring formula, queue management, drain/pause modes |
| `prompt-and-context-assembly.md` | PromptRegistry architecture, static/dynamic sections, per-task building, role-based permissions, preview endpoints, cache optimization |
| `repo-workspace-lifecycle.md` | Workspace CRUD and hydration, repo binding, agent execution environment, boundaries and security risks |
| `mcp-hook-extension-flow.md` | MCP server integration (config vs execution), lifecycle hooks, extension points, maturity assessment |
| `run-report-debugging.md` | Run data model, API endpoints, frontend components, real-time updates, debugging workflow, current gaps |

### Audit (docs/audit/)
| File | Content |
|------|---------|
| `current-project-assessment.md` | What's solved vs unsolved, top 5 strengths, top 5 risks, priority investment areas |
| `high-priority-issues.md` | 14 issues across P0/P1/P2/P3 with location, trigger, risk, and fix guidance |
| `deep-scan-delivery-summary.md` | This document |

## 4. Issues Fixed

### P0-1: Migration 042 Version Conflict — FIXED
- **Problem**: Two migrations shared version 042 (`issue_kind` and `mcp_servers`), causing `goose` `buildMigrations` to fail with "duplicate up migration" error on fresh databases
- **Fix**: Merged both into `042_issue_kind_and_mcp_servers.{up,down}.sql`. Updated both canonical (`server/migrations/`) and embedded (`server/pkg/migrations/migrations/`) directories. Deleted the four conflicting files.
- **Verification**: Compilation passes (`go vet`)

### P1-1: Decompose & Fork Handlers Not Routed — FIXED
- **Problem**: `handler/decompose.go` (3 endpoints) and `handler/fork.go` (3 endpoints) were fully implemented but had no route registration — completely unreachable features
- **Fix**: Added route registrations:
  - `routes_issue.go`: `POST /decompose`, `GET /decompose/{runId}`, `POST /decompose/confirm`
  - `routes_daemon.go`: `POST/tasks/{taskId}/forks/{forkId}/start|complete|fail`
- **Verification**: Compilation passes (`go vet`)

### P1-2: Search Modal Broken UX — FIXED
- **Problem**: Keyboard shortcut `/` and sidebar search button triggered `useModalStore.open("search")` but `ModalRegistry` had no case for "search" — invisible modal that captured state
- **Fix**: Removed `"search"` from `ModalType` union. Changed sidebar search button to `toast.info("Search is coming soon")`. Commented out `/` keyboard shortcut with TODO.
- **Verification**: TypeScript typecheck passes, all 76 tests pass

### P1-4: No Run Phase Transition Validation — FIXED
- **Problem**: `RunOrchestrator.AdvancePhase()` accepted any phase string without validation, allowing invalid transitions (e.g. completed → pending)
- **Fix**: Added `allowedRunTransitions` map and `CanRunTransition()` function (mirroring the task state machine pattern). Added validation check in `AdvancePhase()` before executing the transition.
- **Verification**: Compilation passes (`go vet`)

### Pre-existing TypeScript Errors — FIXED (5 areas)
- **Problem**: 13 TypeScript compilation errors across `decompose-dialog.tsx`, `dependency-badge.tsx`, `sub-issues-section.tsx`, `assignee-picker.tsx`
- **Fix**:
  - Added `DecomposePreview`, `IssueDependency`, `SubtaskPreview`, and related types to `shared/types/issue.ts`
  - Exported new types from `shared/types/index.ts`
  - Added `decomposeIssue`, `getDecomposeResult`, `confirmDecompose`, `listIssueDependencies`, `listSubIssues` methods to `ApiClient`
  - Added `issue_kind` field to `Issue` type
  - Fixed `PickerItem` prop mismatch in `assignee-picker.tsx`
- **Verification**: TypeScript typecheck passes (0 errors), all 76 unit tests pass

## 5. Issues Not Fixed (require larger changes or decisions)

| ID | Issue | Reason |
|----|-------|--------|
| P0-2 | sqlc generated code drift (agent_memory) | Requires running `make sqlc` which needs a DB connection and produces large diffs in generated files |
| P1-3 | DaemonAuth middleware unused | Requires auth model decision: wire DaemonAuth or remove it |
| P2-1 | Issue list 200-item cap | Requires both API pagination and frontend infinite scroll |
| P2-2 | Outbox NoOpPublisher | Architectural decision: webhook? message queue? remove outbox? |
| P2-3 | Embedded migration copy | Build system change to use embed directly |
| P2-4 | Dual API pattern | Large refactor across many files |
| P3-* | Various (E2E coverage, WS indicator, memory cleanup, error boundaries) | Lower priority, significant scope |

## 6. Recommended Next Steps (Priority Order)

### 1. Run `make sqlc` and commit
Clean up the generated code drift. This is mechanical and eliminates a class of confusion.

### 2. Implement basic search
Search is the most-requested missing feature for any task management tool. A minimal implementation:
- Server: Add a `GET /api/issues?q=...` query parameter with `ILIKE` or `tsvector` matching
- Frontend: Implement the search modal with `cmdk` (already common in Next.js apps) or shadcn Command component
- Connect to `useModalStore.open("search")` and the `/` shortcut

### 3. Resolve DaemonAuth direction
Either wire `middleware.DaemonAuth` into daemon routes and migrate existing daemons to `mdt_` tokens, or remove the middleware and document that daemons authenticate via PAT.

### 4. Add issue pagination
Change `listIssues` to support cursor-based pagination. Add infinite scroll or "load more" in the frontend. This unblocks any workspace with more than 200 issues.

### 5. Activate outbox publishing
The outbox pattern is already built. Adding a webhook publisher would unlock external integrations and event-driven automation — a key differentiator for an AI-native platform.

## 7. Module Handoff Readiness

| Module | Ready for handoff? | Notes |
|--------|-------------------|-------|
| `server/internal/service/task.go` | **Yes** | Well-structured, state machine enforced, good logging |
| `server/internal/service/run_orchestrator.go` | **Yes** | Clean lifecycle, good event broadcasting |
| `server/internal/daemon/` | **Moderate** | Complex but well-organized; needs repo cache cleanup story |
| `server/internal/handler/` | **Yes** | One-file-per-domain pattern is easy to navigate |
| `apps/web/features/issues/` | **Moderate** | Large (68 files) but well-organized; needs pagination |
| `apps/web/features/realtime/` | **Yes** | Small, clean, well-documented |
| `server/internal/events/` | **Needs work** | Outbox NoOp creates false impression of completeness |
| `apps/web/features/modals/` | **Yes** | Clean after search fix |
| `server/pkg/agent/` | **Yes** | Could be extracted as standalone module |
| `e2e/` | **Yes** | Good fixtures/helpers pattern, 8 comprehensive specs |
