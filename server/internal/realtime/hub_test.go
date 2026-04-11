package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/multica-ai/alphenix/server/internal/auth"
)

const testWorkspaceID = "test-workspace"
const testUserID = "test-user"

// mockMembershipChecker always returns true.
type mockMembershipChecker struct{}

func (m *mockMembershipChecker) IsMember(_ context.Context, _, _ string) bool {
	return true
}

func makeTestToken(t *testing.T) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": testUserID,
	})
	signed, err := token.SignedString(auth.JWTSecret())
	if err != nil {
		t.Fatalf("failed to sign test JWT: %v", err)
	}
	return signed
}

func newTestHub(t *testing.T) (*Hub, *httptest.Server) {
	t.Helper()
	hub := NewHub([]string{"*"})
	go hub.Run()

	mc := &mockMembershipChecker{}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, mc, w, r)
	})
	server := httptest.NewServer(mux)
	return hub, server
}

func connectWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	token := makeTestToken(t)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + token + "&workspace_id=" + testWorkspaceID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	return conn
}

// totalClients counts all clients across all rooms.
func totalClients(hub *Hub) int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	count := 0
	for _, clients := range hub.rooms {
		count += len(clients)
	}
	return count
}

func TestHub_ClientRegistration(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	count := totalClients(hub)
	if count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn1 := connectWS(t, server)
	defer conn1.Close()
	conn2 := connectWS(t, server)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	msg := []byte(`{"type":"issue:created","data":"test"}`)
	hub.Broadcast(msg)

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received1, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("client 1 read error: %v", err)
	}
	if string(received1) != string(msg) {
		t.Fatalf("client 1: expected %s, got %s", msg, received1)
	}

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received2, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("client 2 read error: %v", err)
	}
	if string(received2) != string(msg) {
		t.Fatalf("client 2: expected %s, got %s", msg, received2)
	}
}

func TestHub_ClientDisconnect(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)

	time.Sleep(50 * time.Millisecond)

	countBefore := totalClients(hub)
	if countBefore != 1 {
		t.Fatalf("expected 1 client before disconnect, got %d", countBefore)
	}

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	countAfter := totalClients(hub)
	if countAfter != 0 {
		t.Fatalf("expected 0 clients after disconnect, got %d", countAfter)
	}
}

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	const numClients = 5
	conns := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conns[i] = connectWS(t, server)
		defer conns[i].Close()
	}

	time.Sleep(50 * time.Millisecond)

	count := totalClients(hub)
	if count != numClients {
		t.Fatalf("expected %d clients, got %d", numClients, count)
	}

	msg := []byte(`{"type":"test","count":5}`)
	hub.Broadcast(msg)

	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, received, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read error: %v", i, err)
		}
		if string(received) != string(msg) {
			t.Fatalf("client %d: expected %s, got %s", i, msg, received)
		}
	}
}

func TestHub_MultipleBroadcasts(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	messages := []string{
		`{"type":"issue:created"}`,
		`{"type":"issue:updated"}`,
		`{"type":"issue:deleted"}`,
	}

	for _, msg := range messages {
		hub.Broadcast([]byte(msg))
	}

	for i, expected := range messages {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, received, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("message %d read error: %v", i, err)
		}
		if string(received) != expected {
			t.Fatalf("message %d: expected %s, got %s", i, expected, received)
		}
	}
}

// ---------------------------------------------------------------------------
// checkOrigin
// ---------------------------------------------------------------------------

func TestCheckOrigin_EmptyOrigin(t *testing.T) {
	fn := checkOrigin([]string{"https://example.com"})
	req := httptest.NewRequest("GET", "/ws", nil)
	if !fn(req) {
		t.Error("request without Origin should be allowed")
	}
}

func TestCheckOrigin_ExactMatch(t *testing.T) {
	fn := checkOrigin([]string{"https://app.example.com"})
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://app.example.com")
	if !fn(req) {
		t.Error("exact origin match should be allowed")
	}
}

func TestCheckOrigin_NoMatch(t *testing.T) {
	fn := checkOrigin([]string{"https://allowed.com"})
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://evil.com")
	if fn(req) {
		t.Error("non-matching origin should be rejected")
	}
}

func TestCheckOrigin_Wildcard(t *testing.T) {
	fn := checkOrigin([]string{"*"})
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://anything.com")
	if !fn(req) {
		t.Error("wildcard should allow any origin")
	}
}

func TestCheckOrigin_EmptyAllowedList(t *testing.T) {
	fn := checkOrigin(nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://example.com")
	if fn(req) {
		t.Error("empty allowed list should reject all origins")
	}
}

func TestCheckOrigin_CaseInsensitive(t *testing.T) {
	fn := checkOrigin([]string{"https://Example.COM"})
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://example.com")
	if !fn(req) {
		t.Error("origin comparison should be case-insensitive")
	}
}

// ---------------------------------------------------------------------------
// BroadcastToWorkspace
// ---------------------------------------------------------------------------

func TestHub_BroadcastToWorkspace(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn1 := connectWS(t, server)
	defer conn1.Close()
	conn2 := connectWS(t, server)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	msg := []byte(`{"type":"workspace_event"}`)
	hub.BroadcastToWorkspace(testWorkspaceID, msg)

	for i, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, received, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read error: %v", i, err)
		}
		if string(received) != string(msg) {
			t.Fatalf("client %d: expected %s, got %s", i, msg, received)
		}
	}
}

func TestHub_BroadcastToWorkspace_DifferentWorkspace(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	hub.BroadcastToWorkspace("other-workspace", []byte(`{"type":"other"}`))

	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("client should not receive message from different workspace")
	}
}

func TestHub_BroadcastToWorkspace_EmptyRoom(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	hub.BroadcastToWorkspace("no-clients", []byte(`{}`))
}

// ---------------------------------------------------------------------------
// SendToUser
// ---------------------------------------------------------------------------

func TestHub_SendToUser(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	msg := []byte(`{"type":"user_notification"}`)
	hub.SendToUser(testUserID, msg)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(received) != string(msg) {
		t.Fatalf("expected %s, got %s", msg, received)
	}
}

func TestHub_SendToUser_NoMatch(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	hub.SendToUser("different-user", []byte(`{}`))

	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("should not receive message targeted at different user")
	}
}

func TestHub_SendToUser_ExcludeWorkspace(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	hub.SendToUser(testUserID, []byte(`{}`), testWorkspaceID)

	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("should not receive message when workspace is excluded")
	}
}

// ---------------------------------------------------------------------------
// CloseBroadcast
// ---------------------------------------------------------------------------

func TestHub_CloseBroadcast_StopsRun(t *testing.T) {
	hub := NewHub([]string{"*"})
	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	hub.CloseBroadcast()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after CloseBroadcast")
	}
}

func TestHub_CloseBroadcast_Idempotent(t *testing.T) {
	hub := NewHub([]string{"*"})
	go hub.Run()

	hub.CloseBroadcast()
	hub.CloseBroadcast()
}

// ---------------------------------------------------------------------------
// NewHub
// ---------------------------------------------------------------------------

func TestHub_NewHub(t *testing.T) {
	hub := NewHub([]string{"https://example.com"})
	if hub == nil {
		t.Fatal("NewHub returned nil")
	}
	if len(hub.rooms) != 0 {
		t.Error("new hub should have no rooms")
	}
}

// ---------------------------------------------------------------------------
// Membership rejection
//-----------

type rejectingMembershipChecker struct{}

func (m *rejectingMembershipChecker) IsMember(_ context.Context, _, _ string) bool {
	return false
}

func TestHandleWebSocket_MembershipForbidden(t *testing.T) {
	hub := NewHub([]string{"*"})
	go hub.Run()
	defer hub.CloseBroadcast()

	mc := &rejectingMembershipChecker{}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, mc, w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	token := makeTestToken(t)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + token + "&workspace_id=" + testWorkspaceID

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected WebSocket handshake to fail for non-member")
	}
	if resp == nil {
		t.Fatal("expected HTTP response on rejected handshake")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
