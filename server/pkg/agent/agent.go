// Package agent provides a unified interface for executing prompts via
// coding agents (Claude Code, Codex, OpenCode). It mirrors the happy-cli AgentBackend
// pattern, translated to idiomatic Go.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Backend is the unified interface for executing prompts via coding agents.
type Backend interface {
	// Execute runs a prompt and returns a Session for streaming results.
	// The caller should read from Session.Messages (optional) and wait on
	// Session.Result for the final outcome.
	Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error)

	// Fork creates a lightweight sub-agent that inherits the parent's context.
	// Inspired by Claude Code's fork mode: the sub-agent shares the parent's
	// working directory and session cache but runs a focused, directive-style prompt.
	//
	// The parent should NOT read ForkSession.OutputFile mid-flight ("Don't peek").
	// Results arrive via ForkSession.Result once the fork completes.
	Fork(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error)
}

// ExecOptions configures a single execution.
type ExecOptions struct {
	Cwd              string
	Model            string
	SystemPrompt     string
	MaxTurns         int
	Timeout          time.Duration
	ResumeSessionID  string // if non-empty, resume a previous agent session
	MaxThinkingTokens int  // max tokens for extended thinking (0 = default)

	// ToolPermissions controls which tools the agent may use.
	// Nil means all tools allowed (default behavior).
	ToolPermissions *ToolPermissions

	// ToolHooks, if set, receive pre/post notifications for each tool call.
	// Pre-hooks can deny or modify a tool call; post-hooks can observe results.
	ToolHooks ToolHooks

	// LifecycleHooks, if set, receive notifications for agent lifecycle events.
	LifecycleHooks LifecycleHooks
}

// ToolPermissions defines role-based tool restrictions for an agent session.
// Inspired by Claude Code's coordinator/agent mode tool allow/deny lists.
type ToolPermissions struct {
	// AllowedTools lists tool names the agent may use. If empty, all tools allowed.
	// Examples: "Bash", "Read", "Edit", "Write", "Grep", "Glob"
	AllowedTools []string

	// DeniedTools lists tool names the agent must NOT use. Takes precedence over AllowedTools.
	DeniedTools []string

	// ReadOnly when true prevents any write operations (Edit, Write, Bash with side effects).
	ReadOnly bool
}

// IsToolAllowed checks if a tool is permitted under these permissions.
func (tp *ToolPermissions) IsToolAllowed(toolName string) bool {
	if tp == nil {
		return true
	}
	// Denied takes precedence.
	for _, d := range tp.DeniedTools {
		if d == toolName {
			return false
		}
	}
	// ReadOnly blocks write tools.
	if tp.ReadOnly {
		switch toolName {
		case "Edit", "Write", "NotebookEdit":
			return false
		}
	}
	// If AllowedTools is empty, everything not denied is allowed.
	if len(tp.AllowedTools) == 0 {
		return true
	}
	for _, a := range tp.AllowedTools {
		if a == toolName {
			return true
		}
	}
	return false
}

// AgentRole defines the behavioral role for an agent session.
// Maps to Claude Code's coordinator vs agent mode distinction.
type AgentRole string

const (
	// AgentRoleExecutor is the default — executes tasks directly.
	AgentRoleExecutor AgentRole = "executor"
	// AgentRoleCoordinator orchestrates sub-agents without doing direct work.
	AgentRoleCoordinator AgentRole = "coordinator"
	// AgentRoleReviewer is read-only — reviews code without modifications.
	AgentRoleReviewer AgentRole = "reviewer"
)

// LifecycleHooks provides callbacks for agent lifecycle events.
// Inspired by Claude Code's session_start and stop hooks.
type LifecycleHooks struct {
	// SessionStart is called when the agent session initializes.
	SessionStart func(ctx context.Context, sessionID string)

	// Stop is called when the agent session ends, before the Result is sent.
	Stop func(ctx context.Context, result Result)
}

// ToolHooks provides pre/post interception for tool calls.
// Inspired by Claude Code's hook system (pre-tool-use, post-tool-use).
type ToolHooks struct {
	// PreToolUse is called before a tool executes. Return Deny to block it.
	PreToolUse func(ctx context.Context, tool string, input map[string]any) ToolHookResult

	// PostToolUse is called after a tool executes with its result.
	PostToolUse func(ctx context.Context, tool string, input map[string]any, output string)
}

// ToolHookResult is the outcome of a PreToolUse hook.
type ToolHookResult struct {
	// Deny, if true, blocks the tool call. The agent receives an error result.
	Deny bool

	// DenyReason is the error message shown to the agent when Deny is true.
	DenyReason string

	// UpdatedInput, if non-nil, replaces the tool's input parameters.
	UpdatedInput map[string]any
}

// Session represents a running agent execution.
type Session struct {
	// Messages streams events as the agent works. The channel is closed
	// when the agent finishes (before Result is sent).
	Messages <-chan Message
	// Result receives exactly one value — the final outcome — then closes.
	Result <-chan Result
}

// MessageType identifies the kind of Message.
type MessageType string

const (
	MessageText       MessageType = "text"
	MessageThinking   MessageType = "thinking"
	MessageToolUse    MessageType = "tool-use"
	MessageToolResult MessageType = "tool-result"
	MessageStatus     MessageType = "status"
	MessageError      MessageType = "error"
	MessageLog        MessageType = "log"
)

// Message is a unified event emitted by an agent during execution.
type Message struct {
	Type    MessageType
	Content string         // text content (Text, Error, Log)
	Tool    string         // tool name (ToolUse, ToolResult)
	CallID  string         // tool call ID (ToolUse, ToolResult)
	Input   map[string]any // tool input (ToolUse)
	Output  string         // tool output (ToolResult)
	Status  string         // agent status string (Status)
	Level   string         // log level (Log)
}

// Result is the final outcome after an agent session completes.
type Result struct {
	Status     string // "completed", "failed", "aborted", "timeout"
	Output     string // accumulated text output
	Error      string // error message if failed
	DurationMs int64
	SessionID  string
}

// ForkOptions configures a fork (lightweight sub-agent with inherited context).
// Inspired by Claude Code's fork mode where omitting subagent_type creates
// a context-sharing child that uses a directive-style prompt.
type ForkOptions struct {
	Cwd             string           // working directory (should match parent's)
	Model           string           // model override
	MaxTurns        int              // turn budget for the fork
	Timeout         time.Duration    // max execution time
	ToolPermissions *ToolPermissions // inherited from parent if nil
	ToolHooks       ToolHooks        // pre/post tool hooks for observability
	ParentSessionID string           // parent's session ID for context sharing

	// OutputFile is the path where the fork writes its final result.
	// The parent must NOT read this file mid-flight ("Don't peek" rule).
	OutputFile string
}

// ForkSession represents a running fork (lightweight sub-agent).
type ForkSession struct {
	// Result receives the fork's outcome when it completes.
	Result <-chan ForkResult

	// OutputFile is the path where the fork will write its result.
	// Do NOT read this while the fork is running ("Don't peek").
	OutputFile string
}

// ForkResult is the outcome of a fork execution.
type ForkResult struct {
	Status     string // "completed", "failed", "aborted", "timeout"
	Output     string // fork's text output
	Error      string // error message if failed
	DurationMs int64
}

// Config configures a Backend instance.
type Config struct {
	ExecutablePath string            // path to CLI binary (claude, codex, or opencode)
	Env            map[string]string // extra environment variables
	Logger         *slog.Logger
}

// New creates a Backend for the given agent type.
// Supported types: "claude", "codex", "opencode".
func New(agentType string, cfg Config) (Backend, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	switch agentType {
	case "claude":
		return &claudeBackend{cfg: cfg}, nil
	case "codex":
		return &codexBackend{cfg: cfg}, nil
	case "opencode":
		return &opencodeBackend{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unknown agent type: %q (supported: claude, codex, opencode)", agentType)
	}
}

// DetectVersion runs the agent CLI with --version and returns the output.
func DetectVersion(ctx context.Context, executablePath string) (string, error) {
	return detectCLIVersion(ctx, executablePath)
}
