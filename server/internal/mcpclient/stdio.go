package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
)

// StdioClient implements Client by communicating with an MCP server
// over stdin/stdout of a subprocess using JSON-RPC 2.0.
type StdioClient struct {
	config ServerConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	id     int64
	closed bool
}

// NewStdioClient creates a new stdio-based MCP client.
func NewStdioClient(config ServerConfig) (Client, error) {
	if config.Command == "" {
		return nil, fmt.Errorf("stdio transport requires a command")
	}
	return &StdioClient{config: config}, nil
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *StdioClient) nextID() int64 {
	c.id++
	return c.id
}

// call sends a JSON-RPC request and reads the response.
// Must be called with c.mu held.
func (c *StdioClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	var paramsRaw json.RawMessage
	if params != nil {
		var err error
		paramsRaw, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
	}

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  method,
		Params:  paramsRaw,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		return nil, fmt.Errorf("read response: EOF")
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

func (c *StdioClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && !c.closed {
		return fmt.Errorf("already connected")
	}

	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)
	if len(c.config.Env) > 0 {
		env := make([]string, 0, len(c.config.Env))
		for k, v := range c.config.Env {
			env = append(env, k+"="+v)
		}
		c.cmd.Env = env
	}

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	c.stdout = bufio.NewScanner(stdout)
	// Allow large tool responses (up to 10MB).
	c.stdout.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}
	c.closed = false

	// MCP initialization handshake.
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "alphenix",
			"version": "1.0.0",
		},
	}
	if _, err := c.call(ctx, "initialize", initParams); err != nil {
		_ = c.cmd.Process.Kill()
		c.closed = true
		return fmt.Errorf("MCP initialize: %w", err)
	}

	// Send initialized notification (no response expected).
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, _ := json.Marshal(notif)
	c.stdin.Write(append(data, '\n'))

	slog.Debug("MCP stdio client connected", "command", c.config.Command, "server", c.config.Name)
	return nil
}

func (c *StdioClient) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.cmd == nil {
		return nil
	}
	c.closed = true

	// Try graceful shutdown via JSON-RPC notification.
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
	}
	data, _ := json.Marshal(notif)
	c.stdin.Write(append(data, '\n'))
	c.stdin.Close()

	// Wait for process to exit, kill if it doesn't.
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()

	select {
	case <-done:
	case <-ctx.Done():
		_ = c.cmd.Process.Kill()
		<-done
	}

	slog.Debug("MCP stdio client disconnected", "server", c.config.Name)
	return nil
}

func (c *StdioClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed && c.cmd != nil
}

type listToolsResult struct {
	Tools []ToolDescriptor `json:"tools"`
}

func (c *StdioClient) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var parsed listToolsResult
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("parse tools list: %w", err)
	}
	return parsed.Tools, nil
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func (c *StdioClient) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	params := callToolParams{Name: name, Arguments: args}
	result, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var toolResult ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, fmt.Errorf("parse tool result: %w", err)
	}
	return &toolResult, nil
}
