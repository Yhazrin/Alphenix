package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// claudeBackend implements Backend by spawning the Claude Code CLI
// with --output-format stream-json.
type claudeBackend struct {
	cfg  Config
	opts ExecOptions // populated per-Execute call
}

func (b *claudeBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	b.opts = opts // store for handleControlRequest
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "claude"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("claude executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := []string{
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "bypassPermissions",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if opts.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.SystemPrompt)
	}
	if opts.ResumeSessionID != "" {
		args = append(args, "--resume", opts.ResumeSessionID)
	}
	args = append(args, "-p", prompt)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("claude stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("claude stdin pipe: %w", err)
	}
	cmd.Stderr = newLogWriter(b.cfg.Logger, "[claude:stderr] ")

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start claude: %w", err)
	}

	b.cfg.Logger.Info("claude started", "pid", cmd.Process.Pid, "cwd", opts.Cwd, "model", opts.Model)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)
		defer stdin.Close()

		startTime := time.Now()
		var output strings.Builder
		var sessionID string
		finalStatus := "completed"
		var finalError string

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var msg claudeSDKMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}

			switch msg.Type {
			case "assistant":
				b.handleAssistant(msg, msgCh, &output)
			case "user":
				b.handleUser(msg, msgCh)
			case "system":
				if msg.SessionID != "" {
					sessionID = msg.SessionID
				}
				trySend(msgCh, Message{Type: MessageStatus, Status: "running"})
			case "result":
				sessionID = msg.SessionID
				if msg.ResultText != "" {
					output.Reset()
					output.WriteString(msg.ResultText)
				}
				if msg.IsError {
					finalStatus = "failed"
					finalError = msg.ResultText
				}
			case "log":
				if msg.Log != nil {
					trySend(msgCh, Message{
						Type:    MessageLog,
						Level:   msg.Log.Level,
						Content: msg.Log.Message,
					})
				}
			case "control_request":
				b.handleControlRequest(msg, stdin)
			}
		}

		// Wait for process exit
		exitErr := cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("claude timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if exitErr != nil && finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("claude exited with error: %v", exitErr)
		}

		b.cfg.Logger.Info("claude finished", "pid", cmd.Process.Pid, "status", finalStatus, "duration", duration.Round(time.Millisecond).String())

		resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

func (b *claudeBackend) Fork(ctx context.Context, prompt string, opts ForkOptions) (*ForkSession, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "claude"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("claude executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	// Fork uses --resume to inherit parent context and a directive-style prompt.
	args := []string{
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "bypassPermissions",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if opts.ParentSessionID != "" {
		args = append(args, "--resume", opts.ParentSessionID)
	}
	args = append(args, "-p", prompt)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)
	cmd.Stderr = newLogWriter(b.cfg.Logger, "[claude:fork:stderr] ")

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start claude fork: %w", err)
	}

	b.cfg.Logger.Info("claude fork started", "pid", cmd.Process.Pid, "parent_session", opts.ParentSessionID)

	resCh := make(chan ForkResult, 1)
	outputFile := opts.OutputFile

	go func() {
		defer cancel()

		startTime := time.Now()
		var output strings.Builder
		finalStatus := "completed"
		var finalError string

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			resCh <- ForkResult{Status: "failed", Error: fmt.Sprintf("fork stdout pipe: %v", err)}
			return
		}

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg claudeSDKMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "assistant":
				var content claudeMessageContent
				if err := json.Unmarshal(msg.Message, &content); err != nil {
					continue
				}
				for _, block := range content.Content {
					if block.Type == "text" && block.Text != "" {
						output.WriteString(block.Text)
					}
				}
			case "result":
				if msg.ResultText != "" {
					output.Reset()
					output.WriteString(msg.ResultText)
				}
				if msg.IsError {
					finalStatus = "failed"
					finalError = msg.ResultText
				}
			}
		}

		exitErr := cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("claude fork timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "fork cancelled"
		} else if exitErr != nil && finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("claude fork exited with error: %v", exitErr)
		}

		outputStr := output.String()

		// Write result to output file if specified ("Don't peek" until done).
		if outputFile != "" && outputStr != "" {
			_ = os.WriteFile(outputFile, []byte(outputStr), 0644)
		}

		b.cfg.Logger.Info("claude fork finished", "pid", cmd.Process.Pid, "status", finalStatus, "duration", duration.Round(time.Millisecond).String())

		resCh <- ForkResult{
			Status:     finalStatus,
			Output:     outputStr,
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
		}
	}()

	return &ForkSession{Result: resCh, OutputFile: outputFile}, nil
}

func (b *claudeBackend) handleAssistant(msg claudeSDKMessage, ch chan<- Message, output *strings.Builder) {
	var content claudeMessageContent
	if err := json.Unmarshal(msg.Message, &content); err != nil {
		return
	}

	for _, block := range content.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				output.WriteString(block.Text)
				trySend(ch, Message{Type: MessageText, Content: block.Text})
			}
		case "thinking":
			if block.Text != "" {
				trySend(ch, Message{Type: MessageThinking, Content: block.Text})
			}
		case "tool_use":
			var input map[string]any
			if block.Input != nil {
				_ = json.Unmarshal(block.Input, &input)
			}
			trySend(ch, Message{
				Type:   MessageToolUse,
				Tool:   block.Name,
				CallID: block.ID,
				Input:  input,
			})
		}
	}
}

func (b *claudeBackend) handleUser(msg claudeSDKMessage, ch chan<- Message) {
	var content claudeMessageContent
	if err := json.Unmarshal(msg.Message, &content); err != nil {
		return
	}

	for _, block := range content.Content {
		if block.Type == "tool_result" {
			resultStr := ""
			if block.Content != nil {
				resultStr = string(block.Content)
			}
			trySend(ch, Message{
				Type:   MessageToolResult,
				CallID: block.ToolUseID,
				Output: resultStr,
			})
		}
	}
}

func (b *claudeBackend) handleControlRequest(msg claudeSDKMessage, stdin interface{ Write([]byte) (int, error) }) {
	var req claudeControlRequestPayload
	if err := json.Unmarshal(msg.Request, &req); err != nil {
		return
	}

	var inputMap map[string]any
	if req.Input != nil {
		_ = json.Unmarshal(req.Input, &inputMap)
	}
	if inputMap == nil {
		inputMap = map[string]any{}
	}

	// Step 1: Check ToolPermissions — deny if tool is not allowed.
	if b.opts.ToolPermissions != nil && !b.opts.ToolPermissions.IsToolAllowed(req.ToolName) {
		b.cfg.Logger.Info("claude: tool denied by permissions", "tool", req.ToolName)
		denyResp := map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": msg.RequestID,
				"response": map[string]any{
					"behavior": "deny",
					"message":  fmt.Sprintf("tool %q is not allowed for this agent role", req.ToolName),
				},
			},
		}
		data, _ := json.Marshal(denyResp)
		data = append(data, '\n')
		_, _ = stdin.Write(data)
		return
	}

	// Step 2: Run PreToolUse hook if configured.
	updatedInput := inputMap
	if b.opts.ToolHooks.PreToolUse != nil {
		result := b.opts.ToolHooks.PreToolUse(context.Background(), req.ToolName, inputMap)
		if result.Deny {
			b.cfg.Logger.Info("claude: tool denied by hook", "tool", req.ToolName, "reason", result.DenyReason)
			denyResp := map[string]any{
				"type": "control_response",
				"response": map[string]any{
					"subtype":    "success",
					"request_id": msg.RequestID,
					"response": map[string]any{
						"behavior": "deny",
						"message":  result.DenyReason,
					},
				},
			}
			data, _ := json.Marshal(denyResp)
			data = append(data, '\n')
			_, _ = stdin.Write(data)
			return
		}
		if result.UpdatedInput != nil {
			updatedInput = result.UpdatedInput
		}
	}

	// Step 3: Allow the tool call.
	response := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": msg.RequestID,
			"response": map[string]any{
				"behavior":     "allow",
				"updatedInput": updatedInput,
			},
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		b.cfg.Logger.Warn("claude: failed to marshal control response", "error", err)
		return
	}
	data = append(data, '\n')
	if _, err := stdin.Write(data); err != nil {
		b.cfg.Logger.Warn("claude: failed to write control response", "error", err)
	}
}

// ── Claude SDK JSON types ──

type claudeSDKMessage struct {
	Type      string          `json:"type"`
	Message   json.RawMessage `json:"message,omitempty"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`

	// result fields
	ResultText string  `json:"result,omitempty"`
	IsError    bool    `json:"is_error,omitempty"`
	DurationMs float64 `json:"duration_ms,omitempty"`
	NumTurns   int     `json:"num_turns,omitempty"`

	// log fields
	Log *claudeLogEntry `json:"log,omitempty"`

	// control request fields
	RequestID string          `json:"request_id,omitempty"`
	Request   json.RawMessage `json:"request,omitempty"`
}

type claudeLogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type claudeMessageContent struct {
	Role    string             `json:"role"`
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

type claudeControlRequestPayload struct {
	Subtype  string          `json:"subtype"`
	ToolName string          `json:"tool_name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// ── Shared helpers ──

func trySend(ch chan<- Message, msg Message) {
	select {
	case ch <- msg:
	default:
		// Channel full — drop message. Final output is accumulated separately
		// in Result.Output, so only streaming consumers are affected.
	}
}

func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

func detectCLIVersion(ctx context.Context, execPath string) (string, error) {
	cmd := exec.CommandContext(ctx, execPath, "--version")
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("detect version for %s: %w", execPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// logWriter adapts a *slog.Logger to an io.Writer for capturing stderr.
type logWriter struct {
	logger *slog.Logger
	prefix string
}

func newLogWriter(logger *slog.Logger, prefix string) *logWriter {
	return &logWriter{logger: logger, prefix: prefix}
}

func (w *logWriter) Write(p []byte) (int, error) {
	text := strings.TrimSpace(string(p))
	if text != "" {
		w.logger.Debug(w.prefix + text)
	}
	return len(p), nil
}
