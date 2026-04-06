# Module Map

## Repository Top-Level

```
alphenix/
├── apps/web/           Next.js 16 frontend (standalone, no shared pkg deps)
├── server/             Go backend (API, CLI, daemon, agent SDK)
├── e2e/                Playwright end-to-end tests
├── scripts/            Shell helpers (postgres, port checks, entrypoint)
├── docs/               Documentation and assets
├── .github/workflows/  CI/CD (lint, test, e2e, release)
├── .devcontainer/      VS Code dev container config
├── Makefile            Orchestration: setup, start, stop, check, build, sqlc
├── docker-compose.yml  Postgres + app + dev profiles
├── Dockerfile          Multi-stage: frontend → go builder → alpine runtime
└── pnpm-workspace.yaml Monorepo config (apps/* only)
```

## Backend (`server/`)

```
server/
├── cmd/
│   ├── server/         HTTP API entry point
│   │   ├── main.go           App bootstrap (pool, bus, hub, outbox, sweepers)
│   │   ├── router.go         Route assembly with middleware
│   │   ├── routes_auth.go    Public auth endpoints
│   │   ├── routes_daemon.go  Daemon task lifecycle API
│   │   ├── routes_issue.go   Issue/comment/attachment routes
│   │   ├── routes_agent.go   Agent management routes
│   │   ├── routes_team.go    Team management routes
│   │   ├── routes_mcp.go     MCP server management routes
│   │   ├── routes_other.go   Skills, runtimes, inbox, runs, tokens, workspace repos
│   │   ├── listeners.go      Bus→Hub event bridge
│   │   ├── subscriber_listeners.go  Issue subscription events
│   │   ├── activity_listeners.go    Activity log recording
│   │   ├── notification_listeners.go  Inbox notification creation
│   │   ├── mcp_lifecycle.go  MCP server startup connections
│   │   ├── memory_sweeper.go Agent memory cleanup goroutine
│   │   └── runtime_sweeper.go Runtime heartbeat timeout goroutine
│   ├── alphenix/      CLI binary (login, daemon, agent, issue, workspace, etc.)
│   └── migrate/        Database migration runner (goose)
├── internal/
│   ├── handler/        HTTP handlers (one file per domain)
│   │   ├── handler.go        Shared Handler struct, helpers, middleware
│   │   ├── auth.go           Login, verify, me, JWT
│   │   ├── issue.go          Issue CRUD, batch operations
│   │   ├── comment.go        Comment CRUD, mention triggers
│   │   ├── daemon.go         Task lifecycle (claim, start, complete, fail)
│   │   ├── run.go            Run management and execution
│   │   ├── skill.go          Skill CRUD, agent-skill bindings
│   │   ├── agent.go          Agent CRUD, prompt preview
│   │   ├── workspace.go      Workspace and member management
│   │   ├── collaboration.go  Messaging, dependencies, checkpoints, memory
│   │   ├── review.go         Review submission
│   │   ├── runtime_*.go      Runtime pool, policy, ping, update, usage
│   │   ├── decompose.go      Goal decomposition (NOT ROUTED)
│   │   ├── fork.go           Fork lifecycle (NOT ROUTED)
│   │   └── ...               (team, inbox, activity, reactions, file, PAT, etc.)
│   ├── service/        Business logic layer
│   │   ├── task.go           Task enqueue, claim, complete, fail, chain, broadcast
│   │   ├── task_state.go     Task state machine (allowed transitions)
│   │   ├── run_orchestrator.go  Run lifecycle, step recording, execution loop
│   │   ├── runtime_selector.go  Policy-based runtime selection
│   │   ├── collaboration.go    Agent messaging, memory (pgvector)
│   │   ├── review.go          Automated review service
│   │   ├── compactor.go       Conversation compaction (token budget)
│   │   ├── coalescer.go       Event coalescing (reduce DB writes)
│   │   ├── summarizer.go      LLM-based summarization
│   │   └── email.go           Transactional email (Resend)
│   ├── daemon/         Local agent runtime
│   │   ├── daemon.go         Core daemon loop (poll, execute, report)
│   │   ├── client.go         HTTP client for daemon API
│   │   ├── system_prompt.go  Prompt registry and assembly
│   │   ├── prompt.go         Per-task prompt building
│   │   ├── hooks.go          Lifecycle and tool hooks
│   │   ├── execenv/          Execution environment (git, worktrees)
│   │   ├── repocache/        Repository caching
│   │   └── usage/            Usage tracking (Claude/Codex tokens)
│   ├── realtime/       WebSocket hub
│   ├── events/         Internal event bus + outbox
│   ├── auth/           JWT generation, CloudFront signing
│   ├── middleware/     Auth, workspace, logging, CloudFront cookies
│   ├── mention/        @mention expansion in comments
│   ├── logger/         Structured slog config
│   └── util/           UUID/text/timestamp conversion helpers
├── pkg/
│   ├── agent/          Agent backend interface and implementations
│   │   ├── agent.go          Backend, Session, ExecOptions, ForkSession
│   │   ├── claude.go         Claude Code CLI backend
│   │   ├── codex.go          Codex CLI backend
│   │   ├── opencode.go       OpenCode CLI backend
│   │   ├── orchestrator.go   TaskSpec, WorkerContext, WorkerState machine
│   │   └── fork_manager.go   Parallel fork orchestration
│   ├── db/
│   │   ├── queries/    SQL source files (35 files, one per domain)
│   │   └── generated/  sqlc output (models.go + query methods)
│   ├── protocol/       Event type constants (protocol.EventXxx)
│   ├── redact/         Secret redaction for agent output
│   └── migrations/     Embedded migration files (copy of server/migrations/)
└── migrations/         Canonical migration directory (49 versions)
```

## Frontend (`apps/web/`)

```
apps/web/
├── app/                    Next.js App Router
│   ├── layout.tsx              Root layout (Theme, Auth, WS, Modals)
│   ├── (landing)/              Landing page (SSR)
│   │   ├── page.tsx            Home page
│   │   ├── about/              About page
│   │   └── changelog/          Changelog page
│   ├── (auth)/login/           Login page
│   └── (dashboard)/            Authenticated dashboard
│       ├── layout.tsx          Auth guard + sidebar shell
│       ├── _components/        Sidebar, keyboard shortcuts
│       ├── issues/             Issue list and detail pages
│       ├── board/              Board view (same component as issues)
│       ├── my-issues/          Filtered to current user
│       ├── inbox/              Notifications
│       ├── agents/             Agent list and detail
│       ├── teams/              Team management
│       ├── runtimes/           Runtime pool management
│       ├── skills/             Skill management
│       ├── mcp/                MCP server configuration
│       ├── settings/           Workspace settings
│       ├── tasks/[id]/         Task detail
│       └── runs/[runId]/       Run detail with steps/timeline
├── features/               Domain modules
│   ├── auth/               Store, initializer, cookie
│   ├── workspace/          Store, hooks, orchestration
│   ├── issues/             Store (6 stores), components (35+), hooks (8), config, utils
│   ├── inbox/              Store with dedup
│   ├── realtime/           WS provider, sync, hooks
│   ├── modals/             Store, registry, create-issue, create-workspace
│   ├── editor/             Tiptap markdown editor with extensions
│   ├── skills/             Skill management components
│   ├── runs/               Run detail components (if present)
│   ├── runtimes/           Runtime management components
│   ├── mcp/                MCP server UI
│   ├── landing/            Landing page components
│   ├── my-issues/          My issues page
│   └── navigation/         Navigation utilities
├── shared/                 Cross-feature code
│   ├── api/                ApiClient (REST), WSClient, domain API modules
│   ├── types/              TypeScript domain types
│   ├── hooks/              Shared hooks (file upload)
│   ├── utils.ts            General utilities
│   └── logger.ts           Console logger with namespaces
├── components/ui/          shadcn/ui primitives
└── test/                   Vitest setup and shared mocks
```

## Module Coupling

### Strong Coupling (by design)
- `handler/` ↔ `service/` — handlers call services directly
- `service/task.go` ↔ `service/runtime_selector.go` — task enqueue calls SelectRuntime
- `daemon/` ↔ `pkg/agent/` — daemon uses agent backends
- `features/issues/` ↔ `shared/api/` — all data flows through ApiClient
- `features/realtime/` ↔ all stores — WS events sync to zustand stores

### Moderate Coupling (cross-cutting)
- `events/bus.go` ↔ all handlers — publish events for WS broadcast
- `middleware/auth.go` ↔ `handler/handler.go` — auth context extraction
- `features/workspace/store.ts` → `features/issues/store.ts` — hydration triggers issue fetch

### Candidates for Future Separation
- `pkg/agent/` could become a standalone Go module (no server deps except types)
- `daemon/` could be extracted as a separate binary with its own go.mod
- `features/landing/` has no dependencies on dashboard features — could be a separate deployment
- `events/outbox.go` + external publishing could become a separate worker process
