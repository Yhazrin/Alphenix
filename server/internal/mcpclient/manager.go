package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/multica-ai/multicode/server/internal/tool"
)

// ServerConfig holds the configuration needed to connect to an MCP server.
// This mirrors the mcp_servers DB table columns used for client connections.
type ServerConfig struct {
	ID        string
	Name      string
	Transport string // "stdio" or "http"
	URL       string // for http transport
	Command   string // for stdio transport
	Args      []string
	Env       map[string]string
}

// ToolResult represents the result of an MCP tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

// ContentBlock is a single content item in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Client defines the interface for communicating with a single MCP server.
// Implementations handle the actual transport (stdio subprocess, HTTP/SSE).
type Client interface {
	// Connect establishes the connection and performs the MCP initialization handshake.
	Connect(ctx context.Context) error
	// Disconnect closes the connection gracefully.
	Disconnect(ctx context.Context) error
	// ListTools discovers available tools on the server.
	ListTools(ctx context.Context) ([]ToolDescriptor, error)
	// CallTool invokes a tool by name with the given arguments.
	CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error)
	// IsConnected returns true if the client has an active connection.
	IsConnected() bool
}

// ToolDescriptor describes a tool available on an MCP server.
type ToolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ClientFactory creates a Client for the given server configuration.
// This allows swapping transport implementations (stdio, HTTP/SSE) without
// changing the Manager logic.
type ClientFactory func(config ServerConfig) (Client, error)

// Manager manages the lifecycle of MCP client connections and syncs
// discovered tools into the shared tool Registry.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]managedClient // server ID → client state
	registry *tool.Registry
	factory  ClientFactory
}

type managedClient struct {
	client Client
	config ServerConfig
	tools  []ToolDescriptor
}

// NewManager creates a Manager with the given tool registry and client factory.
func NewManager(registry *tool.Registry, factory ClientFactory) *Manager {
	return &Manager{
		clients:  make(map[string]managedClient),
		registry: registry,
		factory:  factory,
	}
}

// Connect connects to a single MCP server, performs the initialization handshake,
// discovers available tools, and registers them in the tool registry.
func (m *Manager) Connect(ctx context.Context, config ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If already connected, disconnect first.
	if mc, ok := m.clients[config.ID]; ok {
		if mc.client.IsConnected() {
			if err := mc.client.Disconnect(ctx); err != nil {
				slog.Warn("failed to disconnect existing MCP client", "server", config.Name, "error", err)
			}
		}
		m.unregisterTools(config.ID, config.Name)
		delete(m.clients, config.ID)
	}

	client, err := m.factory(config)
	if err != nil {
		return fmt.Errorf("create MCP client for %q: %w", config.Name, err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect to MCP server %q: %w", config.Name, err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		_ = client.Disconnect(ctx)
		return fmt.Errorf("list tools from MCP server %q: %w", config.Name, err)
	}

	m.clients[config.ID] = managedClient{
		client: client,
		config: config,
		tools:  tools,
	}

	m.registerTools(config, tools)
	slog.Info("MCP server connected", "server", config.Name, "tools", len(tools))
	return nil
}

// Disconnect disconnects from a single MCP server and unregisters its tools.
func (m *Manager) Disconnect(ctx context.Context, serverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mc, ok := m.clients[serverID]
	if !ok {
		return fmt.Errorf("MCP server %q not connected", serverID)
	}

	if mc.client.IsConnected() {
		if err := mc.client.Disconnect(ctx); err != nil {
			slog.Warn("failed to disconnect MCP client", "server", mc.config.Name, "error", err)
		}
	}

	m.unregisterTools(serverID, mc.config.Name)
	delete(m.clients, serverID)
	slog.Info("MCP server disconnected", "server", mc.config.Name)
	return nil
}

// ConnectAll connects to all provided MCP server configs.
// Servers that fail to connect are logged but do not block others.
// Returns the number of successfully connected servers.
func (m *Manager) ConnectAll(ctx context.Context, configs []ServerConfig) int {
	connected := 0
	for _, cfg := range configs {
		if err := m.Connect(ctx, cfg); err != nil {
			slog.Error("failed to connect MCP server", "server", cfg.Name, "error", err)
			continue
		}
		connected++
	}
	return connected
}

// DisconnectAll disconnects from all connected MCP servers.
// Each disconnect uses its own derived context so that a single failure
// or timeout does not prevent other servers from being shut down gracefully.
func (m *Manager) DisconnectAll(ctx context.Context) {
	m.mu.Lock()
	ids := make([]string, 0, len(m.clients))
	for id := range m.clients {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		// Derive per-server context: respects parent deadline but isolates
		// cancellation so one slow disconnect doesn't cancel the rest.
		dCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := m.Disconnect(dCtx, id); err != nil {
			slog.Warn("error disconnecting MCP server", "id", id, "error", err)
		}
		cancel()
	}
}

// IsConnected returns true if the server with the given ID is currently connected.
func (m *Manager) IsConnected(serverID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mc, ok := m.clients[serverID]
	return ok && mc.client.IsConnected()
}

// ConnectedServers returns the IDs of all currently connected servers.
func (m *Manager) ConnectedServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.clients))
	for id, mc := range m.clients {
		if mc.client.IsConnected() {
			ids = append(ids, id)
		}
	}
	return ids
}

// CallTool invokes a tool on the specified MCP server.
func (m *Manager) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (*ToolResult, error) {
	m.mu.RLock()
	mc, ok := m.clients[serverID]
	m.mu.RUnlock()

	if !ok || !mc.client.IsConnected() {
		return nil, fmt.Errorf("MCP server %q not connected", serverID)
	}

	// Apply a reasonable default timeout for tool calls.
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return mc.client.CallTool(callCtx, toolName, args)
}

// registerTools registers discovered MCP tools into the shared registry.
// Tools are namespaced as "mcp.{serverName}.{toolName}" via RegisterDynamic.
// Must be called with m.mu held.
func (m *Manager) registerTools(cfg ServerConfig, tools []ToolDescriptor) {
	for _, td := range tools {
		def := tool.ToolDef{
			Name:        td.Name,
			Description: td.Description,
			Schema:      td.InputSchema,
			Permission:  tool.PermissionNetwork,
			Source:      tool.SourceMCP,
			SourceConfig: tool.MCPSourceConfig(cfg.Name, td.Name, cfg.ID),
		}
		if err := m.registry.RegisterDynamic(def); err != nil {
			slog.Warn("failed to register MCP tool", "server", cfg.Name, "tool", td.Name, "error", err)
		}
	}
}

// unregisterTools removes all tools belonging to the given MCP server from the registry.
// Must be called with m.mu held.
func (m *Manager) unregisterTools(serverID, serverName string) {
	prefix := fmt.Sprintf("mcp.%s.", serverName)
	names := m.registry.Names()
	removed := 0
	for _, name := range names {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			if m.registry.Unregister(name) {
				removed++
			}
		}
	}
	if removed > 0 {
		slog.Info("unregistered MCP tools", "server", serverName, "count", removed)
	}
}
