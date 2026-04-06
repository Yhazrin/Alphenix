# CLI and Agent Daemon Guide

The `alphenix` CLI connects your local machine to Alphenix. It handles authentication, workspace management, issue tracking, and runs the agent daemon that executes AI tasks locally.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap multica-ai/tap
brew install alphenix-cli
```

### Build from Source

```bash
git clone https://github.com/multica-ai/alphenix.git
cd alphenix
make build
cp server/bin/alphenix /usr/local/bin/alphenix
```

### Update

```bash
alphenix update
```

This auto-detects your installation method (Homebrew or manual) and upgrades accordingly.

## Quick Start

```bash
# 1. Authenticate (opens browser for login)
alphenix login

# 2. Start the agent daemon
alphenix daemon start

# 3. Done — agents in your watched workspaces can now execute tasks on your machine
```

`alphenix login` automatically discovers all workspaces you belong to and adds them to the daemon watch list.

## Authentication

### Browser Login

```bash
alphenix login
```

Opens your browser for OAuth authentication, creates a 90-day personal access token, and auto-configures your workspaces.

### Token Login

```bash
alphenix login --token
```

Authenticate by pasting a personal access token directly. Useful for headless environments.

### Check Status

```bash
alphenix auth status
```

Shows your current server, user, and token validity.

### Logout

```bash
alphenix auth logout
```

Removes the stored authentication token.

## Agent Daemon

The daemon is the local agent runtime. It detects available AI CLIs on your machine, registers them with the Alphenix server, and executes tasks when agents are assigned work.

### Start

```bash
alphenix daemon start
```

By default, the daemon runs in the background and logs to `~/.alphenix/daemon.log`.

To run in the foreground (useful for debugging):

```bash
alphenix daemon start --foreground
```

### Stop

```bash
alphenix daemon stop
```

### Status

```bash
alphenix daemon status
alphenix daemon status --output json
```

Shows PID, uptime, detected agents, and watched workspaces.

### Logs

```bash
alphenix daemon logs              # Last 50 lines
alphenix daemon logs -f           # Follow (tail -f)
alphenix daemon logs -n 100       # Last 100 lines
```

### Supported Agents

The daemon auto-detects these AI CLIs on your PATH:

| CLI | Command | Description |
|-----|---------|-------------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `claude` | Anthropic's coding agent |
| [Codex](https://github.com/openai/codex) | `codex` | OpenAI's coding agent |

You need at least one installed. The daemon registers each detected CLI as an available runtime.

### How It Works

1. On start, the daemon detects installed agent CLIs and registers a runtime for each agent in each watched workspace
2. It polls the server at a configurable interval (default: 3s) for claimed tasks
3. When a task arrives, it creates an isolated workspace directory, spawns the agent CLI, and streams results back
4. Heartbeats are sent periodically (default: 15s) so the server knows the daemon is alive
5. On shutdown, all runtimes are deregistered

### Configuration

Daemon behavior is configured via flags or environment variables:

| Setting | Flag | Env Variable | Default |
|---------|------|--------------|---------|
| Poll interval | `--poll-interval` | `ALPHENIX_DAEMON_POLL_INTERVAL` | `3s` |
| Heartbeat interval | `--heartbeat-interval` | `ALPHENIX_DAEMON_HEARTBEAT_INTERVAL` | `15s` |
| Agent timeout | `--agent-timeout` | `ALPHENIX_AGENT_TIMEOUT` | `2h` |
| Max concurrent tasks | `--max-concurrent-tasks` | `ALPHENIX_DAEMON_MAX_CONCURRENT_TASKS` | `20` |
| Daemon ID | `--daemon-id` | `ALPHENIX_DAEMON_ID` | hostname |
| Device name | `--device-name` | `ALPHENIX_DAEMON_DEVICE_NAME` | hostname |
| Runtime name | `--runtime-name` | `ALPHENIX_AGENT_RUNTIME_NAME` | `Local Agent` |
| Workspaces root | — | `ALPHENIX_WORKSPACES_ROOT` | `~/alphenix_workspaces` |

Agent-specific overrides:

| Variable | Description |
|----------|-------------|
| `ALPHENIX_CLAUDE_PATH` | Custom path to the `claude` binary |
| `ALPHENIX_CLAUDE_MODEL` | Override the Claude model used |
| `ALPHENIX_CODEX_PATH` | Custom path to the `codex` binary |
| `ALPHENIX_CODEX_MODEL` | Override the Codex model used |

### Self-Hosted Server

When connecting to a self-hosted Alphenix instance, point the CLI to your server before logging in:

```bash
export ALPHENIX_APP_URL=https://app.example.com
export ALPHENIX_SERVER_URL=wss://api.example.com/ws

alphenix login
alphenix daemon start
```

Or set them persistently:

```bash
alphenix config set app_url https://app.example.com
alphenix config set server_url wss://api.example.com/ws
```

### Profiles

Profiles let you run multiple daemons on the same machine — for example, one for production and one for a staging server.

```bash
# Start a daemon for the staging server
alphenix --profile staging login
alphenix --profile staging daemon start

# Default profile runs separately
alphenix daemon start
```

Each profile gets its own config directory (`~/.alphenix/profiles/<name>/`), daemon state, health port, and workspace root.

## Workspaces

### List Workspaces

```bash
alphenix workspace list
```

Watched workspaces are marked with `*`. The daemon only processes tasks for watched workspaces.

### Watch / Unwatch

```bash
alphenix workspace watch <workspace-id>
alphenix workspace unwatch <workspace-id>
```

### Get Details

```bash
alphenix workspace get <workspace-id>
alphenix workspace get <workspace-id> --output json
```

### List Members

```bash
alphenix workspace members <workspace-id>
```

## Issues

### List Issues

```bash
alphenix issue list
alphenix issue list --status in_progress
alphenix issue list --priority urgent --assignee "Agent Name"
alphenix issue list --limit 20 --output json
```

Available filters: `--status`, `--priority`, `--assignee`, `--limit`.

### Get Issue

```bash
alphenix issue get <id>
alphenix issue get <id> --output json
```

### Create Issue

```bash
alphenix issue create --title "Fix login bug" --description "..." --priority high --assignee "Lambda"
```

Flags: `--title` (required), `--description`, `--status`, `--priority`, `--assignee`, `--parent`, `--due-date`.

### Update Issue

```bash
alphenix issue update <id> --title "New title" --priority urgent
```

### Assign Issue

```bash
alphenix issue assign <id> --to "Lambda"
alphenix issue assign <id> --unassign
```

### Change Status

```bash
alphenix issue status <id> in_progress
```

Valid statuses: `backlog`, `todo`, `in_progress`, `in_review`, `done`, `blocked`, `cancelled`.

### Comments

```bash
# List comments
alphenix issue comment list <issue-id>

# Add a comment
alphenix issue comment add <issue-id> --content "Looks good, merging now"

# Reply to a specific comment
alphenix issue comment add <issue-id> --parent <comment-id> --content "Thanks!"

# Delete a comment
alphenix issue comment delete <comment-id>
```

### Execution History

```bash
# List all execution runs for an issue
alphenix issue runs <issue-id>
alphenix issue runs <issue-id> --output json

# View messages for a specific execution run
alphenix issue run-messages <task-id>
alphenix issue run-messages <task-id> --output json

# Incremental fetch (only messages after a given sequence number)
alphenix issue run-messages <task-id> --since 42 --output json
```

The `runs` command shows all past and current executions for an issue, including running tasks. The `run-messages` command shows the detailed message log (tool calls, thinking, text, errors) for a single run. Use `--since` for efficient polling of in-progress runs.

## Configuration

### View Config

```bash
alphenix config show
```

Shows config file path, server URL, app URL, and default workspace.

### Set Values

```bash
alphenix config set server_url wss://api.example.com/ws
alphenix config set app_url https://app.example.com
alphenix config set workspace_id <workspace-id>
```

## Other Commands

```bash
alphenix version              # Show CLI version and commit hash
alphenix update               # Update to latest version
alphenix agent list           # List agents in the current workspace
```

## Output Formats

Most commands support `--output` with two formats:

- `table` — human-readable table (default for list commands)
- `json` — structured JSON (useful for scripting and automation)

```bash
alphenix issue list --output json
alphenix daemon status --output json
```
