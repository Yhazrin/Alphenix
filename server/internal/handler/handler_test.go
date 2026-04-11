package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/service"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

var testHandler *Handler
var testPool *pgxpool.Pool
var testUserID string
var testWorkspaceID string

const (
	handlerTestEmail         = "handler-test@alphenix.ai"
	handlerTestName          = "Handler Test User"
	handlerTestWorkspaceSlug = "handler-tests"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://alphenix:alphenix@localhost:5432/alphenix?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Skipping tests: could not connect to database: %v\n", err)
		os.Exit(0)
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("Skipping tests: database not reachable: %v\n", err)
		pool.Close()
		os.Exit(0)
	}

	queries := db.New(pool)
	hub := realtime.NewHub([]string{"*"})
	go hub.Run()
	bus := events.New()
	emailSvc := service.NewEmailService()
	testHandler = New(queries, pool, hub, bus, emailSvc, nil, nil, nil)
	testPool = pool

	testUserID, testWorkspaceID, err = setupHandlerTestFixture(ctx, pool)
	if err != nil {
		fmt.Printf("Failed to set up handler test fixture: %v\n", err)
		pool.Close()
		os.Exit(1)
	}

	code := m.Run()
	if err := cleanupHandlerTestFixture(context.Background(), pool); err != nil {
		fmt.Printf("Failed to clean up handler test fixture: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	pool.Close()
	os.Exit(code)
}

func setupHandlerTestFixture(ctx context.Context, pool *pgxpool.Pool) (string, string, error) {
	if err := cleanupHandlerTestFixture(ctx, pool); err != nil {
		return "", "", err
	}

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, handlerTestName, handlerTestEmail).Scan(&userID); err != nil {
		return "", "", err
	}

	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "Handler Tests", handlerTestWorkspaceSlug, "Temporary workspace for handler tests", "HAN").Scan(&workspaceID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		return "", "", err
	}

	var runtimeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, $2, 'cloud', $3, 'online', $4, '{}'::jsonb, now())
		RETURNING id
	`, workspaceID, "Handler Test Runtime", "handler_test_runtime", "Handler test runtime").Scan(&runtimeID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id
		)
		VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'workspace', 1, $4)
	`, workspaceID, "Handler Test Agent", runtimeID, userID); err != nil {
		return "", "", err
	}

	return userID, workspaceID, nil
}

func cleanupHandlerTestFixture(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `DELETE FROM workspace WHERE slug = $1`, handlerTestWorkspaceSlug); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, handlerTestEmail); err != nil {
		return err
	}
	return nil
}

func newRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	return req
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestIssueCRUD(t *testing.T) {
	// Create
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":    "Test issue from Go test",
		"status":   "todo",
		"priority": "medium",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.Title != "Test issue from Go test" {
		t.Fatalf("CreateIssue: expected title 'Test issue from Go test', got '%s'", created.Title)
	}
	if created.Status != "todo" {
		t.Fatalf("CreateIssue: expected status 'todo', got '%s'", created.Status)
	}
	issueID := created.ID

	// Get
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fetched IssueResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != issueID {
		t.Fatalf("GetIssue: expected id '%s', got '%s'", issueID, fetched.ID)
	}

	// Update - partial (only status)
	w = httptest.NewRecorder()
	status := "in_progress"
	req = newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"status": status,
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated IssueResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "in_progress" {
		t.Fatalf("UpdateIssue: expected status 'in_progress', got '%s'", updated.Status)
	}
	if updated.Title != "Test issue from Go test" {
		t.Fatalf("UpdateIssue: title should be preserved, got '%s'", updated.Title)
	}
	if updated.Priority != "medium" {
		t.Fatalf("UpdateIssue: priority should be preserved, got '%s'", updated.Priority)
	}

	// List
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID, nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssues: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var listResp map[string]any
	json.NewDecoder(w.Body).Decode(&listResp)
	issues := listResp["issues"].([]any)
	if len(issues) == 0 {
		t.Fatal("ListIssues: expected at least 1 issue")
	}

	// Delete
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.DeleteIssue(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteIssue: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetIssue after delete: expected 404, got %d", w.Code)
	}
}

func TestCommentCRUD(t *testing.T) {
	// Create an issue first
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Comment test issue",
	})
	testHandler.CreateIssue(w, req)
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)
	issueID := issue.ID

	// Create comment
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Test comment from Go test",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List comments
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID+"/comments", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListComments: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var comments []CommentResponse
	json.NewDecoder(w.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("ListComments: expected 1 comment, got %d", len(comments))
	}
	if comments[0].Content != "Test comment from Go test" {
		t.Fatalf("ListComments: expected content 'Test comment from Go test', got '%s'", comments[0].Content)
	}

	// Cleanup
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.DeleteIssue(w, req)
}

func TestAgentCRUD(t *testing.T) {
	// List agents
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/agents?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAgents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var agents []AgentResponse
	json.NewDecoder(w.Body).Decode(&agents)
	if len(agents) == 0 {
		t.Fatal("ListAgents: expected at least 1 agent")
	}

	// Update agent status
	agentID := agents[0].ID
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+agentID, map[string]any{
		"status": "idle",
	})
	req = withURLParam(req, "id", agentID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated AgentResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "idle" {
		t.Fatalf("UpdateAgent: expected status 'idle', got '%s'", updated.Status)
	}
	if updated.Name != agents[0].Name {
		t.Fatalf("UpdateAgent: name should be preserved, got '%s'", updated.Name)
	}
}

func TestWorkspaceCRUD(t *testing.T) {
	// List workspaces
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/workspaces", nil)
	testHandler.ListWorkspaces(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListWorkspaces: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var workspaces []WorkspaceResponse
	json.NewDecoder(w.Body).Decode(&workspaces)
	if len(workspaces) == 0 {
		t.Fatal("ListWorkspaces: expected at least 1 workspace")
	}

	// Get workspace
	wsID := workspaces[0].ID
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/workspaces/"+wsID, nil)
	req = withURLParam(req, "id", wsID)
	testHandler.GetWorkspace(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetWorkspace: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSendCode(t *testing.T) {
	w := httptest.NewRecorder()
	body := map[string]string{"email": "sendcode-test@alphenix.ai"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] == "" {
		t.Fatal("SendCode: expected non-empty message")
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM verification_code WHERE email = $1`, "sendcode-test@alphenix.ai")
	})
}

func TestSendCodeRateLimit(t *testing.T) {
	const email = "ratelimit-test@alphenix.ai"
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM verification_code WHERE email = $1`, email)
	})

	// First request should succeed
	w := httptest.NewRecorder()
	body := map[string]string{"email": email}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode (first): expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Second request within 60s should be rate limited
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(body)
	req = httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("SendCode (second): expected 429, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyCode(t *testing.T) {
	const email = "verify-test@alphenix.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	// Send code first
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Read code from DB
	dbCode, err := testHandler.Queries.GetLatestVerificationCode(ctx, email)
	if err != nil {
		t.Fatalf("GetLatestVerificationCode: %v", err)
	}

	// Verify with correct code
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": dbCode.Code})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("VerifyCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Token == "" {
		t.Fatal("VerifyCode: expected non-empty token")
	}
	if resp.User.Email != email {
		t.Fatalf("VerifyCode: expected email '%s', got '%s'", email, resp.User.Email)
	}
}

func TestVerifyCodeWrongCode(t *testing.T) {
	const email = "wrong-code-test@alphenix.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
	})

	// Send code
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)

	// Verify with wrong code
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": "000000"})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("VerifyCode (wrong code): expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyCodeBruteForceProtection(t *testing.T) {
	const email = "bruteforce-test@alphenix.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
	})

	// Send code
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Read actual code so we can try it after lockout
	dbCode, err := testHandler.Queries.GetLatestVerificationCode(ctx, email)
	if err != nil {
		t.Fatalf("GetLatestVerificationCode: %v", err)
	}

	// Exhaust all 5 attempts with wrong codes
	for i := 0; i < 5; i++ {
		w = httptest.NewRecorder()
		buf.Reset()
		json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": "000000"})
		req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
		req.Header.Set("Content-Type", "application/json")
		testHandler.VerifyCode(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400, got %d", i+1, w.Code)
		}
	}

	// Now even the correct code should be rejected (code is locked out)
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": dbCode.Code})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("after lockout: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyCodeCreatesWorkspace(t *testing.T) {
	const email = "workspace-verify-test@alphenix.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	// Send code
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)

	// Read code from DB
	dbCode, err := testHandler.Queries.GetLatestVerificationCode(ctx, email)
	if err != nil {
		t.Fatalf("GetLatestVerificationCode: %v", err)
	}

	// Verify
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": dbCode.Code})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("VerifyCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	user, err := testHandler.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}

	workspaces, err := testHandler.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("ListWorkspaces: expected 1 workspace, got %d", len(workspaces))
	}
	if !strings.Contains(workspaces[0].Name, "Workspace") {
		t.Fatalf("expected auto-created workspace name, got %q", workspaces[0].Name)
	}
}

func TestResolveActor(t *testing.T) {
	ctx := context.Background()

	// Look up the agent created by the test fixture.
	var agentID string
	err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentID)
	if err != nil {
		t.Fatalf("failed to find test agent: %v", err)
	}

	// Create a task for the agent so we can test X-Task-ID validation.
	var issueID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
		 VALUES ($1, 'resolveActor test', 'todo', 'none', 'member', $2, 9999, 0)
		 RETURNING id`, testWorkspaceID, testUserID,
	).Scan(&issueID)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	// Look up runtime_id for the agent.
	var runtimeID string
	err = testPool.QueryRow(ctx, `SELECT runtime_id FROM agent WHERE id = $1`, agentID).Scan(&runtimeID)
	if err != nil {
		t.Fatalf("failed to get agent runtime_id: %v", err)
	}

	var taskID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority)
		 VALUES ($1, $2, $3, 'queued', 0)
		 RETURNING id`, agentID, runtimeID, issueID,
	).Scan(&taskID)
	if err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	})

	tests := []struct {
		name            string
		agentIDHeader   string
		taskIDHeader    string
		wantActorType   string
		wantIsAgent     bool
	}{
		{
			name:          "no headers returns member",
			wantActorType: "member",
		},
		{
			name:          "valid agent ID returns agent",
			agentIDHeader: agentID,
			wantActorType: "agent",
			wantIsAgent:   true,
		},
		{
			name:          "non-existent agent ID returns member",
			agentIDHeader: "00000000-0000-0000-0000-000000000099",
			wantActorType: "member",
		},
		{
			name:          "valid agent + valid task returns agent",
			agentIDHeader: agentID,
			taskIDHeader:  taskID,
			wantActorType: "agent",
			wantIsAgent:   true,
		},
		{
			name:          "valid agent + wrong task returns member",
			agentIDHeader: agentID,
			taskIDHeader:  "00000000-0000-0000-0000-000000000099",
			wantActorType: "member",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest("GET", "/test", nil)
			if tt.agentIDHeader != "" {
				req.Header.Set("X-Agent-ID", tt.agentIDHeader)
			}
			if tt.taskIDHeader != "" {
				req.Header.Set("X-Task-ID", tt.taskIDHeader)
			}

			actorType, actorID := testHandler.resolveActor(req, testUserID, testWorkspaceID)

			if actorType != tt.wantActorType {
				t.Errorf("actorType = %q, want %q", actorType, tt.wantActorType)
			}
			if tt.wantIsAgent {
				if actorID != tt.agentIDHeader {
					t.Errorf("actorID = %q, want agent %q", actorID, tt.agentIDHeader)
				}
			} else {
				if actorID != testUserID {
					t.Errorf("actorID = %q, want user %q", actorID, testUserID)
				}
			}
		})
	}
}

func TestDaemonRegisterMissingWorkspaceReturns404(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`{
		"workspace_id":"00000000-0000-0000-0000-000000000001",
		"daemon_id":"local-daemon",
		"device_name":"test-machine",
		"runtimes":[{"name":"Local Codex","type":"codex","version":"1.0.0","status":"online"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)

	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DaemonRegister: expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "workspace not found") {
		t.Fatalf("DaemonRegister: expected workspace not found error, got %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Webhook CRUD
// ---------------------------------------------------------------------------

func TestWebhookCRUD(t *testing.T) {
	// Create
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/webhooks?workspace_id="+testWorkspaceID, map[string]any{
		"url":         "https://example.com/hook",
		"event_types": []string{"issue:created", "run:completed"},
	})
	testHandler.CreateWebhook(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateWebhook: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created WebhookResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.URL != "https://example.com/hook" {
		t.Fatalf("CreateWebhook: expected url 'https://example.com/hook', got '%s'", created.URL)
	}
	if len(created.EventTypes) != 2 {
		t.Fatalf("CreateWebhook: expected 2 event types, got %d", len(created.EventTypes))
	}
	if !created.IsActive {
		t.Fatal("CreateWebhook: expected is_active=true")
	}
	webhookID := created.ID

	// Get
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/webhooks/"+webhookID, nil)
	req = withURLParam(req, "id", webhookID)
	testHandler.GetWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetWebhook: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fetched WebhookResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != webhookID {
		t.Fatalf("GetWebhook: expected id '%s', got '%s'", webhookID, fetched.ID)
	}

	// List
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/webhooks?workspace_id="+testWorkspaceID, nil)
	testHandler.ListWebhooks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListWebhooks: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var webhooks []WebhookResponse
	json.NewDecoder(w.Body).Decode(&webhooks)
	found := false
	for _, wh := range webhooks {
		if wh.ID == webhookID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ListWebhooks: created webhook not found in list")
	}

	// Update — deactivate
	w = httptest.NewRecorder()
	isActive := false
	req = newRequest("PUT", "/api/webhooks/"+webhookID, map[string]any{
		"is_active": isActive,
	})
	req = withURLParam(req, "id", webhookID)
	testHandler.UpdateWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateWebhook: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated WebhookResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.IsActive {
		t.Fatal("UpdateWebhook: expected is_active=false after update")
	}

	// Delete
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/webhooks/"+webhookID, nil)
	req = withURLParam(req, "id", webhookID)
	testHandler.DeleteWebhook(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteWebhook: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/webhooks/"+webhookID, nil)
	req = withURLParam(req, "id", webhookID)
	testHandler.GetWebhook(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetWebhook after delete: expected 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Daemon Register — validation (no DB needed)
// ---------------------------------------------------------------------------

func TestDaemonRegister_MissingDaemonID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`{
		"workspace_id":"`+testWorkspaceID+`",
		"device_name":"test-machine",
		"runtimes":[{"name":"Local","type":"codex","status":"online"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "daemon_id is required") {
		t.Fatalf("expected daemon_id error, got: %s", w.Body.String())
	}
}

func TestDaemonRegister_MissingWorkspaceID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`{
		"daemon_id":"local-daemon",
		"device_name":"test-machine",
		"runtimes":[{"name":"Local","type":"codex","status":"online"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "workspace_id is required") {
		t.Fatalf("expected workspace_id error, got: %s", w.Body.String())
	}
}

func TestDaemonRegister_EmptyRuntimes(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`{
		"workspace_id":"`+testWorkspaceID+`",
		"daemon_id":"local-daemon",
		"device_name":"test-machine",
		"runtimes":[]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "at least one runtime is required") {
		t.Fatalf("expected runtimes error, got: %s", w.Body.String())
	}
}

func TestDaemonRegister_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDaemonRegister_Success(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`{
		"workspace_id":"`+testWorkspaceID+`",
		"daemon_id":"test-daemon-`+testWorkspaceID[:8]+`",
		"device_name":"test-machine",
		"cli_version":"1.0.0",
		"runtimes":[
			{"name":"Test Runtime","type":"codex","version":"1.0.0","status":"online"}
		]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	runtimes, ok := resp["runtimes"].([]any)
	if !ok || len(runtimes) == 0 {
		t.Fatal("DaemonRegister: expected runtimes in response")
	}
	repos, ok := resp["repos"].([]any)
	if !ok {
		t.Fatal("DaemonRegister: expected repos array in response")
	}
	_ = repos
}

func TestDaemonDeregister_EmptyRuntimeIDs(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/deregister", bytes.NewBufferString(`{
		"runtime_ids":[]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonDeregister(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDaemonHeartbeat_MissingRuntimeID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/heartbeat", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonHeartbeat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Skill CRUD
// ---------------------------------------------------------------------------

func TestSkillCRUD(t *testing.T) {
	// Create
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/skills?workspace_id="+testWorkspaceID, map[string]any{
		"name":        "Test Skill",
		"description": "A test skill",
		"content":     "You are a helpful test assistant.",
		"files": []map[string]string{
			{"path": "instructions.md", "content": "# Instructions\nDo stuff."},
		},
	})
	testHandler.CreateSkill(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateSkill: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created SkillWithFilesResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.Name != "Test Skill" {
		t.Fatalf("CreateSkill: expected name 'Test Skill', got '%s'", created.Name)
	}
	if len(created.Files) != 1 {
		t.Fatalf("CreateSkill: expected 1 file, got %d", len(created.Files))
	}
	skillID := created.ID

	// Get
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/skills/"+skillID, nil)
	req = withURLParam(req, "id", skillID)
	testHandler.GetSkill(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetSkill: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fetched SkillWithFilesResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != skillID {
		t.Fatalf("GetSkill: expected id '%s', got '%s'", skillID, fetched.ID)
	}
	if fetched.Name != "Test Skill" {
		t.Fatalf("GetSkill: expected name 'Test Skill', got '%s'", fetched.Name)
	}

	// List
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/skills?workspace_id="+testWorkspaceID, nil)
	testHandler.ListSkills(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListSkills: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var skills []SkillResponse
	json.NewDecoder(w.Body).Decode(&skills)
	found := false
	for _, s := range skills {
		if s.ID == skillID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ListSkills: created skill not found in list")
	}

	// Update
	w = httptest.NewRecorder()
	newDesc := "Updated description"
	req = newRequest("PUT", "/api/skills/"+skillID, map[string]any{
		"description": newDesc,
	})
	req = withURLParam(req, "id", skillID)
	testHandler.UpdateSkill(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateSkill: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated SkillResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Description != newDesc {
		t.Fatalf("UpdateSkill: expected description '%s', got '%s'", newDesc, updated.Description)
	}

	// Delete
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/skills/"+skillID, nil)
	req = withURLParam(req, "id", skillID)
	testHandler.DeleteSkill(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteSkill: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/skills/"+skillID, nil)
	req = withURLParam(req, "id", skillID)
	testHandler.GetSkill(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetSkill after delete: expected 404, got %d", w.Code)
	}
}

func TestCreateSkill_MissingName(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/skills?workspace_id="+testWorkspaceID, map[string]any{
		"description": "no name",
	})
	testHandler.CreateSkill(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSkill_DuplicateName(t *testing.T) {
	// Create first skill
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/skills?workspace_id="+testWorkspaceID, map[string]any{
		"name":    "Duplicate Skill Test",
		"content": "first",
	})
	testHandler.CreateSkill(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateSkill (first): expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created SkillWithFilesResponse
	json.NewDecoder(w.Body).Decode(&created)

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM skill WHERE id = $1`, parseUUID(created.ID))
	})

	// Try to create duplicate
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/skills?workspace_id="+testWorkspaceID, map[string]any{
		"name":    "Duplicate Skill Test",
		"content": "second",
	})
	testHandler.CreateSkill(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("CreateSkill (duplicate): expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSkill_InvalidFilePath(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/skills?workspace_id="+testWorkspaceID, map[string]any{
		"name":  "Bad Path Skill",
		"files": []map[string]string{{"path": "../../etc/passwd", "content": "bad"}},
	})
	testHandler.CreateSkill(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSkill_AbsoluteFilePath(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/skills?workspace_id="+testWorkspaceID, map[string]any{
		"name":  "Abs Path Skill",
		"files": []map[string]string{{"path": "/etc/passwd", "content": "bad"}},
	})
	testHandler.CreateSkill(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for absolute path, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSkill_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/skills/00000000-0000-0000-0000-000000000999", nil)
	req = withURLParam(req, "id", "00000000-0000-0000-0000-000000000999")
	testHandler.GetSkill(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetSkill not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// validateFilePath — pure unit tests
// ---------------------------------------------------------------------------

func TestValidateFilePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"instructions.md", true},
		{"sub/dir/file.txt", true},
		{"", false},
		{"../../etc/passwd", false},
		{"/etc/passwd", false},
		{"a/../../../b", false},
		{"./file.txt", true},
		{"dir/../file.txt", true}, // Clean resolves to "file.txt"
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := validateFilePath(tc.path); got != tc.want {
				t.Errorf("validateFilePath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Issue CRUD — additional integration tests (DB required)
// ---------------------------------------------------------------------------

func TestCreateIssue_TitleTooLong(t *testing.T) {
	longTitle := strings.Repeat("a", 501)
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": longTitle,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for title > 500 chars, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateIssue_EmptyTitle(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateIssue_DefaultStatusAndPriority(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Default status/priority test",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.Status != "backlog" {
		t.Fatalf("expected default status 'backlog', got '%s'", created.Status)
	}
	if created.Priority != "none" {
		t.Fatalf("expected default priority 'none', got '%s'", created.Priority)
	}

	// Cleanup
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/issues/"+created.ID, nil)
	req = withURLParam(req, "id", created.ID)
	testHandler.DeleteIssue(w, req)
}

func TestCreateIssue_InvalidDueDate(t *testing.T) {
	w := httptest.NewRecorder()
	badDate := "not-a-date"
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":    "Bad due date test",
		"due_date": badDate,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid due_date, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListIssues_LimitClamping(t *testing.T) {
	// Negative limit should be clamped to 0
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&limit=-5", nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	issues := resp["issues"].([]any)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues with limit=0 (clamped from -5), got %d", len(issues))
	}

	// Limit > 200 should be clamped to 200
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&limit=999", nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	json.NewDecoder(w.Body).Decode(&resp)
	issues = resp["issues"].([]any)
	// Should succeed — just clamped to 200 max
	if len(issues) > 200 {
		t.Fatalf("expected at most 200 issues with limit=999 (clamped), got %d", len(issues))
	}
}

func TestListIssues_InvalidLimitIgnored(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&limit=abc", nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (invalid limit silently ignored), got %d: %s", w.Code, w.Body.String())
	}
}

func TestListIssues_CursorPagination(t *testing.T) {
	ctx := context.Background()

	// Create 3 issues with distinct positions to test cursor pagination
	issueIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		var id string
		err := testPool.QueryRow(ctx, `
			INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position, issue_kind)
			VALUES ($1, $2, 'todo', 'none', 'member', $3, $4, $5, 'task')
			RETURNING id
		`, testWorkspaceID, fmt.Sprintf("Cursor test issue %d", i), testUserID, 8000+i, float64(i)*10.0).Scan(&id)
		if err != nil {
			t.Fatalf("failed to create cursor test issue %d: %v", i, err)
		}
		issueIDs[i] = id
	}

	t.Cleanup(func() {
		for _, id := range issueIDs {
			testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, parseUUID(id))
		}
	})

	// Fetch first page with limit=1
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&limit=1", nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssues page 1: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var page1 map[string]any
	json.NewDecoder(w.Body).Decode(&page1)
	page1Issues := page1["issues"].([]any)
	if len(page1Issues) == 0 {
		t.Fatal("ListIssues page 1: expected at least 1 issue")
	}

	nextCursor, hasNext := page1["next_cursor"]
	if !hasNext {
		t.Fatal("ListIssues page 1: expected next_cursor in response")
	}

	// Fetch second page using cursor
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&limit=1&cursor="+nextCursor.(string), nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssues page 2: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var page2 map[string]any
	json.NewDecoder(w.Body).Decode(&page2)
	page2Issues := page2["issues"].([]any)
	if len(page2Issues) == 0 {
		t.Fatal("ListIssues page 2: expected at least 1 issue")
	}

	// Verify no overlap between pages
	page1ID := page1Issues[0].(map[string]any)["id"].(string)
	page2ID := page2Issues[0].(map[string]any)["id"].(string)
	if page1ID == page2ID {
		t.Fatalf("ListIssues: page 1 and page 2 returned the same issue %s", page1ID)
	}
}

func TestListIssues_InvalidCursor(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&cursor=not-valid-cursor", nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid cursor, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListIssues_StatusFilter(t *testing.T) {
	ctx := context.Background()

	// Create an issue with a specific status
	var issueID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position, issue_kind)
		VALUES ($1, 'Status filter test', 'in_progress', 'none', 'member', $2, 9000, 100.0, 'task')
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&issueID)
	if err != nil {
		t.Fatalf("failed to create filter test issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, parseUUID(issueID))
	})

	// Filter by status=in_progress — should include our issue
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&status=in_progress", nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	issues := resp["issues"].([]any)
	found := false
	for _, iss := range issues {
		if iss.(map[string]any)["id"] == issueID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("status filter: expected to find issue with status=in_progress")
	}

	// Filter by status=done — should NOT include our issue
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&status=done", nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	json.NewDecoder(w.Body).Decode(&resp)
	issues = resp["issues"].([]any)
	for _, iss := range issues {
		if iss.(map[string]any)["id"] == issueID {
			t.Fatal("status filter: found issue with status=in_progress when filtering by status=done")
		}
	}
}

func TestGetIssue_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues/00000000-0000-0000-0000-000000000999", nil)
	req = withURLParam(req, "id", "00000000-0000-0000-0000-000000000999")
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Workspace — pure unit tests
// ---------------------------------------------------------------------------

func TestGenerateIssuePrefix(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
	}{
		{"Jiayuan's Workspace", "JIA"},
		{"My Team", "MYT"},
		{"AB", "AB"},
		{"A", "A"},
		{"12345", "WS"},
		{"Hello World", "HEL"},
		{"", "WS"},
		{"a", "A"},
		{"Test-Project", "TES"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateIssuePrefix(tc.name)
			if got != tc.prefix {
				t.Errorf("generateIssuePrefix(%q) = %q, want %q", tc.name, got, tc.prefix)
			}
		})
	}
}

func TestGenerateIssuePrefixMaxLength(t *testing.T) {
	got := generateIssuePrefix("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if len(got) > 3 {
		t.Fatalf("expected prefix max 3 chars, got %d chars: %q", len(got), got)
	}
}

// ---------------------------------------------------------------------------
// Comment — integration tests (DB required)
// ---------------------------------------------------------------------------

func TestListComments_InvalidLimit(t *testing.T) {
	// Create an issue first
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Comment pagination test",
	})
	testHandler.CreateIssue(w, req)
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)

	t.Cleanup(func() {
		w := httptest.NewRecorder()
		req = newRequest("DELETE", "/api/issues/"+issue.ID, nil)
		req = withURLParam(req, "id", issue.ID)
		testHandler.DeleteIssue(w, req)
	})

	// Invalid limit (non-numeric)
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issue.ID+"/comments?limit=abc", nil)
	req = withURLParam(req, "id", issue.ID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid limit, got %d: %s", w.Code, w.Body.String())
	}

	// Invalid limit (zero)
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issue.ID+"/comments?limit=0", nil)
	req = withURLParam(req, "id", issue.ID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit=0, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListComments_InvalidOffset(t *testing.T) {
	// Create an issue first
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Comment offset test",
	})
	testHandler.CreateIssue(w, req)
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)

	t.Cleanup(func() {
		w := httptest.NewRecorder()
		req = newRequest("DELETE", "/api/issues/"+issue.ID, nil)
		req = withURLParam(req, "id", issue.ID)
		testHandler.DeleteIssue(w, req)
	})

	// Negative offset
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issue.ID+"/comments?offset=-1", nil)
	req = withURLParam(req, "id", issue.ID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative offset, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListComments_InvalidSince(t *testing.T) {
	// Create an issue first
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Comment since test",
	})
	testHandler.CreateIssue(w, req)
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)

	t.Cleanup(func() {
		w := httptest.NewRecorder()
		req = newRequest("DELETE", "/api/issues/"+issue.ID, nil)
		req = withURLParam(req, "id", issue.ID)
		testHandler.DeleteIssue(w, req)
	})

	// Invalid since format
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issue.ID+"/comments?since=not-a-date", nil)
	req = withURLParam(req, "id", issue.ID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid since, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateComment_EmptyContent(t *testing.T) {
	// Create an issue first
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Empty comment test",
	})
	testHandler.CreateIssue(w, req)
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)

	t.Cleanup(func() {
		w := httptest.NewRecorder()
		req = newRequest("DELETE", "/api/issues/"+issue.ID, nil)
		req = withURLParam(req, "id", issue.ID)
		testHandler.DeleteIssue(w, req)
	})

	// Empty content
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/"+issue.ID+"/comments", map[string]any{
		"content": "",
	})
	req = withURLParam(req, "id", issue.ID)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty content, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListComments_Pagination(t *testing.T) {
	ctx := context.Background()

	// Create an issue
	var issueID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position, issue_kind)
		VALUES ($1, 'Comment pagination test', 'todo', 'none', 'member', $2, 9500, 200.0, 'task')
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&issueID)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create 3 comments
	for i := 0; i < 3; i++ {
		_, err := testPool.Exec(ctx, `
			INSERT INTO comment (issue_id, workspace_id, author_type, author_id, content, type)
			VALUES ($1, $2, 'member', $3, $4, 'text')
		`, parseUUID(issueID), parseUUID(testWorkspaceID), testUserID, fmt.Sprintf("Comment %d", i))
		if err != nil {
			t.Fatalf("failed to create comment %d: %v", i, err)
		}
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM comment WHERE issue_id = $1`, parseUUID(issueID))
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, parseUUID(issueID))
	})

	// Paginated: limit=1, offset=0
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues/"+issueID+"/comments?limit=1&offset=0", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var comments []CommentResponse
	json.NewDecoder(w.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment with limit=1, got %d", len(comments))
	}

	// Paginated: limit=1, offset=1
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID+"/comments?limit=1&offset=1", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var comments2 []CommentResponse
	json.NewDecoder(w.Body).Decode(&comments2)
	if len(comments2) != 1 {
		t.Fatalf("expected 1 comment with limit=1&offset=1, got %d", len(comments2))
	}
	if comments2[0].ID == comments[0].ID {
		t.Fatal("offset=1 returned same comment as offset=0")
	}
}

// ---------------------------------------------------------------------------
// Daemon Register — upsert behavior (DB required)
// ---------------------------------------------------------------------------

func TestDaemonRegisterDuplicateUpserts(t *testing.T) {
	body := `{
		"workspace_id":"` + testWorkspaceID + `",
		"daemon_id":"upsert-daemon-` + testWorkspaceID[:8] + `",
		"device_name":"test-machine",
		"runtimes":[{"name":"Claude","type":"claude","version":"1.0","status":"online"}]
	}`

	// First register
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first register: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var first []AgentRuntimeResponse
	json.NewDecoder(w.Body).Decode(&first)

	// Second register (upsert) — same daemon, updated version
	body2 := `{
		"workspace_id":"` + testWorkspaceID + `",
		"daemon_id":"upsert-daemon-` + testWorkspaceID[:8] + `",
		"device_name":"test-machine",
		"runtimes":[{"name":"Claude","type":"claude","version":"2.0","status":"online"}]
	}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(body2))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("second register: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var second []AgentRuntimeResponse
	json.NewDecoder(w.Body).Decode(&second)
	if len(second) != 1 {
		t.Fatalf("expected 1 runtime, got %d", len(second))
	}
	// Runtime ID should be the same (upsert)
	if second[0].ID != first[0].ID {
		t.Errorf("expected same runtime ID on upsert, got first=%s second=%s", first[0].ID, second[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Skill — additional tests (multi-file create, frontmatter edge cases)
// ---------------------------------------------------------------------------

func TestCreateSkillWithMultipleFiles(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/skills?workspace_id="+testWorkspaceID, map[string]any{
		"name":        "Multi-File Skill " + testWorkspaceID[:8],
		"description": "Has multiple files",
		"content":     "# Skill",
		"files": []map[string]any{
			{"path": "helper.sh", "content": "#!/bin/bash\necho hello"},
			{"path": "config.yaml", "content": "key: value"},
			{"path": "lib/utils.go", "content": "package lib"},
		},
	})
	testHandler.CreateSkill(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateSkill: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created SkillWithFilesResponse
	json.NewDecoder(w.Body).Decode(&created)
	if len(created.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(created.Files))
	}
	paths := map[string]bool{}
	for _, f := range created.Files {
		paths[f.Path] = true
	}
	if !paths["helper.sh"] || !paths["config.yaml"] || !paths["lib/utils.go"] {
		t.Fatalf("expected files helper.sh, config.yaml, lib/utils.go, got %v", paths)
	}

	// Cleanup
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/skills/"+created.ID+"?workspace_id="+testWorkspaceID, nil)
	req = withURLParam(req, "id", created.ID)
	testHandler.DeleteSkill(w, req)
}

// ---------------------------------------------------------------------------
// validateFilePath — additional edge cases (extends existing test)
// ---------------------------------------------------------------------------

func TestValidateFilePath_WindowsAndTraversal(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"C:\\Windows\\system.ini", false},       // absolute Windows path
		{"src/../../../etc/passwd", false},        // deep traversal
		{"helper.sh", true},                       // simple filename
		{"a/b/c.txt", true},                       // nested relative
		{"src/../config.yaml", true},              // cleans to config.yaml
	}
	for _, tc := range cases {
		got := validateFilePath(tc.path)
		if got != tc.want {
			t.Errorf("validateFilePath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// parseSkillFrontmatter — pure unit tests
// ---------------------------------------------------------------------------

func TestParseSkillFrontmatter(t *testing.T) {
	cases := []struct {
		content  string
		wantName string
		wantDesc string
	}{
		{
			content:  "---\nname: my-skill\ndescription: Does things\n---\n# Content",
			wantName: "my-skill",
			wantDesc: "Does things",
		},
		{
			content:  "---\nname: \"quoted skill\"\ndescription: 'single quoted'\n---\n",
			wantName: "quoted skill",
			wantDesc: "single quoted",
		},
		{
			content:  "# No frontmatter\nJust content",
			wantName: "",
			wantDesc: "",
		},
		{
			content:  "---\nname: only-name\n---\n",
			wantName: "only-name",
			wantDesc: "",
		},
		{
			content:  "---\nno-colon-line\n---\n",
			wantName: "",
			wantDesc: "",
		},
		{
			content:  "---\nname: spaced description\n  description:   padded  \n---\n",
			wantName: "spaced description",
			wantDesc: "padded",
		},
	}
	for i, tc := range cases {
		name, desc := parseSkillFrontmatter(tc.content)
		if name != tc.wantName {
			t.Errorf("case %d: name = %q, want %q", i, name, tc.wantName)
		}
		if desc != tc.wantDesc {
			t.Errorf("case %d: description = %q, want %q", i, desc, tc.wantDesc)
		}
	}
}

// ---------------------------------------------------------------------------
// Run handler — validation (no DB needed for 400/404 paths)
// ---------------------------------------------------------------------------

func TestCreateRun_NoWorkspaceID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/runs", map[string]any{
		"issue_id": "some-issue",
		"agent_id": "some-agent",
	})
	req.Header.Del("X-Workspace-ID")
	testHandler.CreateRun(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateRun_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/runs", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testHandler.CreateRun(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetRun_InvalidID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/runs/not-a-uuid", nil)
	req = withURLParam(req, "runId", "not-a-uuid")
	testHandler.GetRun(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListRuns_NoWorkspaceID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/runs", nil)
	req.Header.Del("X-Workspace-ID")
	testHandler.ListRuns(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListRuns_InvalidLimit(t *testing.T) {
	t.Parallel()
	// limit > 200 should be ignored and default to 50
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/runs?limit=999", nil)
	testHandler.ListRuns(w, req)
	// Should still succeed (DB call may fail but handler won't reject the limit)
	if w.Code == http.StatusBadRequest {
		t.Fatalf("handler rejected valid request shape: %s", w.Body.String())
	}
}

func TestRecordStep_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/runs/fake-id/record", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req = withURLParam(req, "runId", "fake-id")
	testHandler.RecordStep(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateRunTodo_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/runs/fake-id/todos", strings.NewReader("nope"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req = withURLParam(req, "runId", "fake-id")
	testHandler.CreateRunTodo(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRunTodo_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/todos/fake-id", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req = withURLParam(req, "todoId", "fake-id")
	testHandler.UpdateRunTodo(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExecuteRun_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/runs/fake-id/execute", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req = withURLParam(req, "runId", "fake-id")
	testHandler.ExecuteRun(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExecuteRun_RunNotFound(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	fakeID := "00000000-0000-0000-0000-000000000000"
	req := newRequest("POST", "/api/runs/"+fakeID+"/execute", map[string]any{
		"provider":        "claude",
		"executable_path": "/usr/bin/claude",
		"prompt":          "hello",
	})
	req = withURLParam(req, "runId", fakeID)
	testHandler.ExecuteRun(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListRunEvents_InvalidRunID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/runs/not-uuid/events", nil)
	req = withURLParam(req, "runId", "not-uuid")
	testHandler.ListRunEvents(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Daemon handler — validation (no DB needed for 400 paths)
// ---------------------------------------------------------------------------

// (daemon validation tests already covered in existing handler_test.go and daemon_test.go)

func TestStartTask_InvalidID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/tasks/not-uuid/start", nil)
	req = withURLParam(req, "taskId", "not-uuid")
	testHandler.StartTask(w, req)
	// Invalid UUID will cause DB error → 400
	if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 400/500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCompleteTask_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/tasks/fake/complete", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req = withURLParam(req, "taskId", "fake")
	testHandler.CompleteTask(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFailTask_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/tasks/fake/fail", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req = withURLParam(req, "taskId", "fake")
	testHandler.FailTask(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTaskStatus_InvalidID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/daemon/tasks/not-uuid/status", nil)
	req = withURLParam(req, "taskId", "not-uuid")
	testHandler.GetTaskStatus(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReportTaskMessages_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/tasks/fake/messages", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req = withURLParam(req, "taskId", "fake")
	testHandler.ReportTaskMessages(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListTaskMessages_InvalidTaskID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/daemon/tasks/not-uuid/messages", nil)
	req = withURLParam(req, "taskId", "not-uuid")
	testHandler.ListTaskMessages(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChainTask_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/tasks/fake/chain", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req = withURLParam(req, "taskId", "fake")
	testHandler.ChainTask(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChainTask_MissingTargetAgentID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	b, _ := json.Marshal(map[string]string{"target_agent_id": "", "chain_reason": "test"})
	req := httptest.NewRequest("POST", "/api/daemon/tasks/fake/chain", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req = withURLParam(req, "taskId", "fake")
	testHandler.ChainTask(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListTasksByIssue_InvalidID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues/not-uuid/tasks", nil)
	req = withURLParam(req, "id", "not-uuid")
	testHandler.ListTasksByIssue(w, req)
	// Invalid UUID → DB error → 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReportTaskProgress_InvalidBody(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/tasks/fake/progress", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req = withURLParam(req, "taskId", "fake")
	testHandler.ReportTaskProgress(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Inbox handler — validation (no DB needed for 401/404 paths)
// ---------------------------------------------------------------------------

func TestListInbox_MissingUserID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/inbox", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	// No X-User-ID header
	testHandler.ListInbox(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCountUnreadInbox_MissingUserID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/inbox/count", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testHandler.CountUnreadInbox(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMarkAllInboxRead_MissingUserID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/inbox/read-all", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testHandler.MarkAllInboxRead(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArchiveAllInbox_MissingUserID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/inbox/archive-all", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testHandler.ArchiveAllInbox(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArchiveAllReadInbox_MissingUserID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/inbox/archive-read", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testHandler.ArchiveAllReadInbox(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArchiveCompletedInbox_MissingUserID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/inbox/archive-completed", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testHandler.ArchiveCompletedInbox(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMarkInboxRead_NotFound(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	fakeID := "00000000-0000-0000-0000-000000000000"
	req := newRequest("POST", "/api/inbox/"+fakeID+"/read", nil)
	req = withURLParam(req, "id", fakeID)
	testHandler.MarkInboxRead(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArchiveInboxItem_NotFound(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	fakeID := "00000000-0000-0000-0000-000000000000"
	req := newRequest("POST", "/api/inbox/"+fakeID+"/archive", nil)
	req = withURLParam(req, "id", fakeID)
	testHandler.ArchiveInboxItem(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoadInboxItemForUser_MissingWorkspace(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	fakeID := "00000000-0000-0000-0000-000000000000"
	req := newRequest("POST", "/api/inbox/"+fakeID+"/read", nil)
	req.Header.Del("X-Workspace-ID")
	req = withURLParam(req, "id", fakeID)
	testHandler.MarkInboxRead(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// runEventToMap — pure unit test
// ---------------------------------------------------------------------------

func TestRunEventToMap(t *testing.T) {
	t.Parallel()
	ev := db.RunEvent{
		EventType: "step:completed",
		Payload:   []byte(`{"tool":"bash","result":"ok"}`),
	}
	m := runEventToMap(ev)
	if m["event_type"] != "step:completed" {
		t.Errorf("expected event_type=step:completed, got %v", m["event_type"])
	}
	payload, ok := m["payload"].(map[string]any)
	if !ok {
		t.Fatal("payload should be a map")
	}
	if payload["tool"] != "bash" {
		t.Errorf("expected tool=bash in payload, got %v", payload["tool"])
	}
}

func TestRunEventToMap_InvalidPayload(t *testing.T) {
	t.Parallel()
	ev := db.RunEvent{
		EventType: "step:started",
		Payload:   []byte("not json"),
	}
	m := runEventToMap(ev)
	payload, ok := m["payload"].(map[string]any)
	if !ok {
		t.Fatal("payload should be a map even for invalid JSON")
	}
	if len(payload) != 0 {
		t.Errorf("expected empty payload for invalid JSON, got %v", payload)
	}
}
