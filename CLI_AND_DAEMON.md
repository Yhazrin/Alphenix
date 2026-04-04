# CLI and Agent Daemon Guide

The `multicode` CLI connects your local machine to Multicode. It handles authentication, workspace management, issue tracking, and runs the agent daemon that executes AI tasks locally.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap multicode-ai/tap
brew install multicode-cli
```

### Build from Source

```bash
git clone https://github.com/multicode-ai/multicode.git
cd multicode
make build
cp server/bin/multicode /usr/local/bin/multicode
```

### Update

```bash
multicode update
```

This auto-detects your installation method (Homebrew or manual) and upgrades accordingly.

## Quick Start

```bash
# 1. Authenticate (opens browser for login)
multicode login

# 2. Start the agent daemon
multicode daemon start

# 3. Done — agents in your watched workspaces can now execute tasks on your machine
```

`multicode login` automatically discovers all workspaces you belong to and adds them to the daemon watch list.

## Authentication

### Browser Login

```bash
multicode login
```

Opens your browser for OAuth authentication, creates a 90-day personal access token, and auto-configures your workspaces.

### Token Login

```bash
multicode login --token
```

Authenticate by pasting a personal access token directly. Useful for headless environments.

### Check Status

```bash
multicode auth status
```

Shows your current server, user, and token validity.

### Logout

```bash
multicode auth logout
```

Removes the stored authentication token.

## Agent Daemon

The daemon is the local agent runtime. It detects available AI CLIs on your machine, registers them with the Multicode server, and executes tasks when agents are assigned work.

### Start

```bash
multicode daemon start
```

By default, the daemon runs in the background and logs to `~/.multicode/daemon.log`.

To run in the foreground (useful for debugging):

```bash
multicode daemon start --foreground
```

### Stop

```bash
multicode daemon stop
```

### Status

```bash
multicode daemon status
multicode daemon status --output json
```

Shows PID, uptime, detected agents, and watched workspaces.

### Logs

```bash
multicode daemon logs              # Last 50 lines
multicode daemon logs -f           # Follow (tail -f)
multicode daemon logs -n 100       # Last 100 lines
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
| Poll interval | `--poll-interval` | `MULTICODE_DAEMON_POLL_INTERVAL` | `3s` |
| Heartbeat interval | `--heartbeat-interval` | `MULTICODE_DAEMON_HEARTBEAT_INTERVAL` | `15s` |
| Agent timeout | `--agent-timeout` | `MULTICODE_AGENT_TIMEOUT` | `2h` |
| Max concurrent tasks | `--max-concurrent-tasks` | `MULTICODE_DAEMON_MAX_CONCURRENT_TASKS` | `20` |
| Daemon ID | `--daemon-id` | `MULTICODE_DAEMON_ID` | hostname |
| Device name | `--device-name` | `MULTICODE_DAEMON_DEVICE_NAME` | hostname |
| Runtime name | `--runtime-name` | `MULTICODE_AGENT_RUNTIME_NAME` | `Local Agent` |
| Workspaces root | — | `MULTICODE_WORKSPACES_ROOT` | `~/multicode_workspaces` |

Agent-specific overrides:

| Variable | Description |
|----------|-------------|
| `MULTICODE_CLAUDE_PATH` | Custom path to the `claude` binary |
| `MULTICODE_CLAUDE_MODEL` | Override the Claude model used |
| `MULTICODE_CODEX_PATH` | Custom path to the `codex` binary |
| `MULTICODE_CODEX_MODEL` | Override the Codex model used |

### Self-Hosted Server

When connecting to a self-hosted Multicode instance, point the CLI to your server before logging in:

```bash
export MULTICODE_APP_URL=https://app.example.com
export MULTICODE_SERVER_URL=wss://api.example.com/ws

multicode login
multicode daemon start
```

Or set them persistently:

```bash
multicode config set app_url https://app.example.com
multicode config set server_url wss://api.example.com/ws
```

### Profiles

Profiles let you run multiple daemons on the same machine — for example, one for production and one for a staging server.

```bash
# Start a daemon for the staging server
multicode --profile staging login
multicode --profile staging daemon start

# Default profile runs separately
multicode daemon start
```

Each profile gets its own config directory (`~/.multicode/profiles/<name>/`), daemon state, health port, and workspace root.

## Workspaces

### List Workspaces

```bash
multicode workspace list
```

Watched workspaces are marked with `*`. The daemon only processes tasks for watched workspaces.

### Watch / Unwatch

```bash
multicode workspace watch <workspace-id>
multicode workspace unwatch <workspace-id>
```

### Get Details

```bash
multicode workspace get <workspace-id>
multicode workspace get <workspace-id> --output json
```

### List Members

```bash
multicode workspace members <workspace-id>
```

## Issues

### List Issues

```bash
multicode issue list
multicode issue list --status in_progress
multicode issue list --priority urgent --assignee "Agent Name"
multicode issue list --limit 20 --output json
```

Available filters: `--status`, `--priority`, `--assignee`, `--limit`.

### Get Issue

```bash
multicode issue get <id>
multicode issue get <id> --output json
```

### Create Issue

```bash
multicode issue create --title "Fix login bug" --description "..." --priority high --assignee "Lambda"
```

Flags: `--title` (required), `--description`, `--status`, `--priority`, `--assignee`, `--parent`, `--due-date`.

### Update Issue

```bash
multicode issue update <id> --title "New title" --priority urgent
```

### Assign Issue

```bash
multicode issue assign <id> --to "Lambda"
multicode issue assign <id> --unassign
```

### Change Status

```bash
multicode issue status <id> in_progress
```

Valid statuses: `backlog`, `todo`, `in_progress`, `in_review`, `done`, `blocked`, `cancelled`.

### Comments

```bash
# List comments
multicode issue comment list <issue-id>

# Add a comment
multicode issue comment add <issue-id> --content "Looks good, merging now"

# Reply to a specific comment
multicode issue comment add <issue-id> --parent <comment-id> --content "Thanks!"

# Delete a comment
multicode issue comment delete <comment-id>
```

### Execution History

```bash
# List all execution runs for an issue
multicode issue runs <issue-id>
multicode issue runs <issue-id> --output json

# View messages for a specific execution run
multicode issue run-messages <task-id>
multicode issue run-messages <task-id> --output json

# Incremental fetch (only messages after a given sequence number)
multicode issue run-messages <task-id> --since 42 --output json
```

The `runs` command shows all past and current executions for an issue, including running tasks. The `run-messages` command shows the detailed message log (tool calls, thinking, text, errors) for a single run. Use `--since` for efficient polling of in-progress runs.

## Configuration

### View Config

```bash
multicode config show
```

Shows config file path, server URL, app URL, and default workspace.

### Set Values

```bash
multicode config set server_url wss://api.example.com/ws
multicode config set app_url https://app.example.com
multicode config set workspace_id <workspace-id>
```

## Other Commands

```bash
multicode version              # Show CLI version and commit hash
multicode update               # Update to latest version
multicode agent list           # List agents in the current workspace
```

## Output Formats

Most commands support `--output` with two formats:

- `table` — human-readable table (default for list commands)
- `json` — structured JSON (useful for scripting and automation)

```bash
multicode issue list --output json
multicode daemon status --output json
```
