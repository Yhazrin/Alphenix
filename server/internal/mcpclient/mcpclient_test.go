package mcpclient

import (
	"context"
	"fmt"
	"testing"

	"github.com/multica-ai/alphenix/server/internal/tool"
)

// --- stubClient implements Client for testing the Manager ---

type stubClient struct {
	connected bool
	tools     []ToolDescriptor
	callErr   error
	callResult *ToolResult
}

func (s *stubClient) Connect(_ context.Context) error {
	s.connected = true
	return nil
}

func (s *stubClient) Disconnect(_ context.Context) error {
	s.connected = false
	return nil
}

func (s *stubClient) ListTools(_ context.Context) ([]ToolDescriptor, error) {
	if s.tools == nil {
		return nil, fmt.Errorf("no tools")
	}
	return s.tools, nil
}

func (s *stubClient) CallTool(_ context.Context, _ string, _ map[string]any) (*ToolResult, error) {
	return s.callResult, s.callErr
}

func (s *stubClient) IsConnected() bool {
	return s.connected
}

// --- factory tests ---

func TestDefaultFactory_Stdio(t *testing.T) {
	factory := DefaultFactory()
	// Empty command will fail at Connect, not at factory creation.
	_, err := factory(ServerConfig{Transport: "stdio", Command: "echo"})
	if err != nil {
		t.Fatalf("unexpected error creating stdio client: %v", err)
	}
}

func TestDefaultFactory_DefaultTransport(t *testing.T) {
	factory := DefaultFactory()
	// Transport "" should default to stdio.
	_, err := factory(ServerConfig{Command: "echo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultFactory_UnsupportedTransport(t *testing.T) {
	factory := DefaultFactory()
	_, err := factory(ServerConfig{Transport: "websocket"})
	if err == nil {
		t.Fatal("expected error for unsupported transport")
	}
}

func TestDefaultFactory_MissingCommand(t *testing.T) {
	factory := DefaultFactory()
	_, err := factory(ServerConfig{Transport: "stdio"})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

// --- Manager tests ---

func newTestManager() (*Manager, *tool.Registry) {
	registry := tool.NewRegistry()
	var stub *stubClient
	factory := func(config ServerConfig) (Client, error) {
		stub = &stubClient{
			tools: []ToolDescriptor{
				{Name: "test-tool", Description: "a test tool"},
			},
		}
		return stub, nil
	}
	m := NewManager(registry, factory)
	return m, registry
}

func TestManager_Connect(t *testing.T) {
	m, registry := newTestManager()
	err := m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "test-srv", Command: "echo"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !m.IsConnected("srv-1") {
		t.Error("expected srv-1 to be connected")
	}
	// Verify tool was registered.
	names := registry.Names()
	found := false
	for _, n := range names {
		if n == "mcp.test-srv.test-tool" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tool mcp.test-srv.test-tool registered, got %v", names)
	}
}

func TestManager_Connect_ReplacesExisting(t *testing.T) {
	m, _ := newTestManager()
	err := m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "test-srv", Command: "echo"})
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	// Reconnect should disconnect old and replace.
	err = m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "test-srv-v2", Command: "echo"})
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}
	if !m.IsConnected("srv-1") {
		t.Error("expected srv-1 still connected after reconnect")
	}
}

func TestManager_Connect_FactoryError(t *testing.T) {
	registry := tool.NewRegistry()
	factory := func(config ServerConfig) (Client, error) {
		return nil, fmt.Errorf("factory boom")
	}
	m := NewManager(registry, factory)
	err := m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "bad", Command: "nope"})
	if err == nil {
		t.Fatal("expected error on factory failure")
	}
}

func TestManager_Disconnect(t *testing.T) {
	m, registry := newTestManager()
	m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "test-srv", Command: "echo"})

	err := m.Disconnect(context.Background(), "srv-1")
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if m.IsConnected("srv-1") {
		t.Error("expected srv-1 to be disconnected")
	}
	// Tool should be unregistered.
	for _, n := range registry.Names() {
		if n == "mcp.test-srv.test-tool" {
			t.Error("tool should have been unregistered after disconnect")
		}
	}
}

func TestManager_Disconnect_NotConnected(t *testing.T) {
	m, _ := newTestManager()
	err := m.Disconnect(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for disconnecting nonexistent server")
	}
}

func TestManager_ConnectAll(t *testing.T) {
	m, _ := newTestManager()
	configs := []ServerConfig{
		{ID: "srv-1", Name: "one", Command: "echo"},
		{ID: "srv-2", Name: "two", Command: "echo"},
	}
	connected := m.ConnectAll(context.Background(), configs)
	if connected != 2 {
		t.Errorf("connected = %d, want 2", connected)
	}
}

func TestManager_DisconnectAll(t *testing.T) {
	m, _ := newTestManager()
	m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "one", Command: "echo"})
	m.Connect(context.Background(), ServerConfig{ID: "srv-2", Name: "two", Command: "echo"})

	m.DisconnectAll(context.Background())
	if m.IsConnected("srv-1") || m.IsConnected("srv-2") {
		t.Error("expected all servers disconnected")
	}
}

func TestManager_ConnectedServers(t *testing.T) {
	m, _ := newTestManager()
	m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "one", Command: "echo"})
	m.Connect(context.Background(), ServerConfig{ID: "srv-2", Name: "two", Command: "echo"})

	ids := m.ConnectedServers()
	if len(ids) != 2 {
		t.Errorf("expected 2 connected servers, got %d", len(ids))
	}
}

func TestManager_CallTool_NotConnected(t *testing.T) {
	m, _ := newTestManager()
	_, err := m.CallTool(context.Background(), "nonexistent", "tool", nil)
	if err == nil {
		t.Fatal("expected error calling tool on disconnected server")
	}
}

func TestManager_CallTool_Success(t *testing.T) {
	registry := tool.NewRegistry()
	expected := &ToolResult{Content: []ContentBlock{{Type: "text", Text: "hello"}}}
	factory := func(config ServerConfig) (Client, error) {
		return &stubClient{
			connected:  true,
			tools:      []ToolDescriptor{{Name: "greet"}},
			callResult: expected,
		}, nil
	}
	m := NewManager(registry, factory)
	m.Connect(context.Background(), ServerConfig{ID: "srv-1", Name: "test", Command: "echo"})

	result, err := m.CallTool(context.Background(), "srv-1", "greet", map[string]any{"name": "world"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.Content[0].Text != "hello" {
		t.Errorf("got %q, want %q", result.Content[0].Text, "hello")
	}
}

// --- stdio validation tests ---

func TestNewStdioClient_EmptyCommand(t *testing.T) {
	_, err := NewStdioClient(ServerConfig{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestNewStdioClient_ValidCommand(t *testing.T) {
	c, err := NewStdioClient(ServerConfig{Command: "echo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestStdioClient_IsConnected_BeforeConnect(t *testing.T) {
	c, _ := NewStdioClient(ServerConfig{Command: "echo"})
	if c.IsConnected() {
		t.Error("should not be connected before Connect()")
	}
}

func TestStdioClient_Call_ClosedClient(t *testing.T) {
	c, _ := NewStdioClient(ServerConfig{Command: "echo"})
	sc := c.(*StdioClient)
	sc.closed = true
	_, err := sc.call(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected error calling closed client")
	}
}
