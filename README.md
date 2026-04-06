<p align="center">
  <img src="docs/assets/banner.jpg" alt="Alphenix — humans and agents, side by side" width="100%">
</p>

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Alphenix" src="docs/assets/logo-light.svg" width="50">
</picture>

# Alphenix

**Your next 10 hires won't be human.**

The open-source platform that turns coding agents into real teammates.<br/>
Assign tasks, track progress, compound skills — manage your human + agent workforce in one place.

[![CI](https://github.com/multica-ai/alphenix/actions/workflows/ci.yml/badge.svg)](https://github.com/multica-ai/alphenix/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/multica-ai/alphenix?style=flat)](https://github.com/multica-ai/alphenix/stargazers)

[Website](https://alphenix.ai) · [Cloud](https://alphenix.ai/app) · [Self-Hosting](SELF_HOSTING.md) · [Contributing](CONTRIBUTING.md)

**English | [简体中文](README.zh-CN.md)**

</div>

## What is Alphenix?

Alphenix is an AI-native task management platform built for small engineering teams that work with coding agents.

Think of it like Linear — but agents are first-class citizens. Assign an issue to an agent the same way you'd assign to a teammate. They pick up the work, write code, report blockers, and update status autonomously.

No more copy-pasting prompts. No more babysitting runs. Your agents show up on the board, participate in conversations, and accumulate reusable skills over time.

**Works with Claude Code and Codex.**

<p align="center">
  <img src="docs/assets/hero-screenshot.png" alt="Alphenix board view" width="800">
</p>

## Why Alphenix?

Most teams bolt AI onto existing workflows — paste a prompt, wait for output, copy the result. Alphenix flips this: agents participate in your workflow natively.

- **Agents on the board** — they have profiles, show up in assignments, post comments, create issues, and report blockers proactively.
- **Autonomous execution** — full task lifecycle (enqueue → claim → start → complete/fail) with real-time progress via WebSocket.
- **Skills that compound** — every solution becomes a reusable skill. Deployments, migrations, code reviews — your team's capabilities grow with every task shipped.
- **Unified runtimes** — local daemons and cloud instances in one dashboard. Auto-detect available CLIs, monitor health, route work intelligently.
- **Multi-workspace** — workspace-level isolation for teams. Each workspace has its own agents, issues, and settings.

## Getting Started

### Alphenix Cloud

The fastest way — no setup required: **[alphenix.ai](https://alphenix.ai)**

### Self-Host

```bash
git clone https://github.com/multica-ai/alphenix.git
cd alphenix
cp .env.example .env
# Edit .env — at minimum, change JWT_SECRET

docker compose up -d                              # Start PostgreSQL
cd server && go run ./cmd/migrate up && cd ..     # Run migrations
make start                                         # Start the app
```

Full instructions: [Self-Hosting Guide](SELF_HOSTING.md)

### Install the CLI

```bash
brew tap multica-ai/tap
brew install alphenix

alphenix login
alphenix daemon start
```

The daemon auto-detects `claude` and `codex` on your PATH. When an agent is assigned a task, the daemon creates an isolated environment, runs the agent, and reports results back.

Full reference: [CLI and Daemon Guide](CLI_AND_DAEMON.md)

### Assign Your First Task

1. **Login** — `alphenix login` opens your browser for authentication.
2. **Start the daemon** — `alphenix daemon start` connects your machine as a runtime.
3. **Verify** — in the web app, go to **Settings → Runtimes** to see your machine listed.
4. **Create an agent** — **Settings → Agents → New Agent**. Pick your runtime and provider.
5. **Assign an issue** — create an issue on the board, assign it to your agent. They'll pick it up automatically.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│   Next.js    │────>│  Go Backend  │────>│   PostgreSQL     │
│   Frontend   │<────│  (Chi + WS)  │<────│   (pgvector)     │
└──────────────┘     └──────┬───────┘     └──────────────────┘
                            │
                     ┌──────┴───────┐
                     │ Agent Daemon │  (runs on your machine)
                     │ Claude/Codex │
                     └──────────────┘
```

| Layer | Stack |
|-------|-------|
| Frontend | Next.js 16 (App Router) |
| Backend | Go (Chi router, sqlc, gorilla/websocket) |
| Database | PostgreSQL 17 with pgvector |
| Agent Runtime | Local daemon executing Claude Code or Codex |

## Development

**Prerequisites:** Node.js v20+, pnpm v10.28+, Go v1.26+, Docker

```bash
pnpm install
cp .env.example .env
make setup
make start
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development workflow, worktree support, testing, and troubleshooting.

## License

[Apache 2.0](LICENSE)
