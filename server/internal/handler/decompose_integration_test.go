package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/service"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

func init() {
	// Suppress slog output during tests.
	slog.SetDefault(slog.New(slog.DiscardHandler))
}

// ---------------------------------------------------------------------------
// stubRow — implements pgx.Row for QueryRow calls
// ---------------------------------------------------------------------------

type stubRow struct {
	vals []any
	err  error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i >= len(r.vals) {
			break
		}
		assignVal(d, r.vals[i])
	}
	return nil
}

// assignVal copies src to a pgx scan destination.
func assignVal(dest, src any) {
	switch d := dest.(type) {
	case *pgtype.UUID:
		*d = src.(pgtype.UUID)
	case *string:
		*d = src.(string)
	case *pgtype.Text:
		*d = src.(pgtype.Text)
	case *pgtype.Int4:
		*d = src.(pgtype.Int4)
	case *int32:
		*d = src.(int32)
	case *int64:
		*d = src.(int64)
	case *float64:
		*d = src.(float64)
	case *bool:
		*d = src.(bool)
	case *[]byte:
		if src == nil {
			*d = nil
		} else {
			*d = src.([]byte)
		}
	case *pgtype.Timestamptz:
		*d = src.(pgtype.Timestamptz)
	case *pgtype.Numeric:
		*d = src.(pgtype.Numeric)
	default:
		if dp, ok := dest.(*any); ok {
			*dp = src
		}
	}
}

// ---------------------------------------------------------------------------
// stubRows — implements pgx.Rows for Query (many-rows) calls
// ---------------------------------------------------------------------------

type stubRows struct {
	data [][]any
	idx  int
}

func (r *stubRows) Close()                                        {}
func (r *stubRows) Err() error                                    { return nil }
func (r *stubRows) CommandTag() pgconn.CommandTag                 { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription  { return nil }
func (r *stubRows) Next() bool {
	r.idx++
	return r.idx <= len(r.data)
}
func (r *stubRows) Scan(dest ...any) error {
	row := r.data[r.idx-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		assignVal(d, row[i])
	}
	return nil
}
func (r *stubRows) Values() ([]any, error) { return r.data[r.idx-1], nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

// ---------------------------------------------------------------------------
// stubDBTX — implements db.DBTX, pattern-matches SQL for dispatch
// ---------------------------------------------------------------------------

type stubDBTX struct {
	fq *fakeQueries
}

func (s *stubDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (s *stubDBTX) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	trimmed := strings.TrimSpace(sql)
	switch {
	case strings.Contains(trimmed, "FROM agent") && strings.Contains(trimmed, "archived_at IS NULL"):
		return s.listAgentsRows()
	default:
		return &stubRows{}, nil
	}
}

func (s *stubDBTX) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	trimmed := strings.TrimSpace(sql)
	switch {
	case strings.Contains(trimmed, "FROM agent_runtime"):
		return s.getAgentRuntimeRow(args)
	case strings.Contains(trimmed, "FROM issue") && strings.Contains(trimmed, "workspace_id = $2"):
		return s.getIssueInWorkspaceRow(args)
	case strings.Contains(trimmed, "UPDATE workspace") && strings.Contains(trimmed, "issue_counter"):
		return s.incrementCounterRow()
	case strings.Contains(trimmed, "INSERT INTO issue_dependency"):
		return s.createIssueDependencyRow(args)
	case strings.Contains(trimmed, "INSERT INTO issue"):
		return s.createIssueRow(args)
	default:
		return &stubRow{err: fmt.Errorf("unhandled SQL: %s", trimmed[:minInt(80, len(trimmed))])}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- DBTX handlers ---

func (s *stubDBTX) listAgentsRows() (pgx.Rows, error) {
	if s.fq.failOn == "ListAgents" {
		return nil, fmt.Errorf("injected error")
	}
	var rows [][]any
	for _, a := range s.fq.agents {
		rows = append(rows, []any{
			a.ID, a.WorkspaceID, a.Name, a.AvatarUrl, a.RuntimeMode,
			a.RuntimeConfig, a.Visibility, a.Status, a.MaxConcurrentTasks,
			a.OwnerID, a.CreatedAt, a.UpdatedAt, a.Description,
			a.RuntimeID, a.Instructions, a.ArchivedAt, a.ArchivedBy,
		})
	}
	return &stubRows{data: rows}, nil
}

func (s *stubDBTX) getAgentRuntimeRow(args []any) pgx.Row {
	if s.fq.failOn == "GetAgentRuntime" {
		return &stubRow{err: fmt.Errorf("injected error")}
	}
	rt := db.AgentRuntime{
		ID:          pgtype.UUID{Valid: true},
		Provider:    "claude",
		Metadata:    []byte(`{}`),
		RuntimeMode: "cli",
		Status:      "active",
	}
	if len(args) > 0 {
		if id, ok := args[0].(pgtype.UUID); ok {
			rt.ID = id
		}
	}
	return &stubRow{vals: []any{
		rt.ID, rt.WorkspaceID, rt.DaemonID, rt.Name, rt.RuntimeMode,
		rt.Provider, rt.Status, rt.DeviceInfo, rt.Metadata, rt.LastSeenAt,
		rt.CreatedAt, rt.UpdatedAt, rt.InstanceID, rt.OwnerUserID,
		rt.ApprovalStatus, rt.Visibility, rt.TrustLevel, rt.DrainMode,
		rt.Paused, rt.Tags, rt.MaxConcurrentTasksOverride, rt.LastClaimedAt,
		rt.SuccessCount24h, rt.FailureCount24h, rt.AvgTaskDurationMs,
	}}
}

func (s *stubDBTX) getIssueInWorkspaceRow(args []any) pgx.Row {
	if s.fq.failOn == "GetIssueInWorkspace" {
		return &stubRow{err: fmt.Errorf("injected error")}
	}
	var issueID pgtype.UUID
	if len(args) > 0 {
		issueID, _ = args[0].(pgtype.UUID)
	}
	s.fq.mu.Lock()
	issue, ok := s.fq.issues[issueID]
	s.fq.mu.Unlock()
	if !ok {
		return &stubRow{err: fmt.Errorf("issue not found")}
	}
	return &stubRow{vals: []any{
		issue.ID, issue.WorkspaceID, issue.Title, issue.Description,
		issue.Status, issue.Priority, issue.AssigneeType, issue.AssigneeID,
		issue.CreatorType, issue.CreatorID, issue.ParentIssueID,
		issue.AcceptanceCriteria, issue.ContextRefs, issue.Position,
		issue.DueDate, issue.CreatedAt, issue.UpdatedAt, issue.Number,
		issue.IssueKind, issue.RepoID,
	}}
}

func (s *stubDBTX) incrementCounterRow() pgx.Row {
	if s.fq.failOn == "IncrementIssueCounter" {
		return &stubRow{err: fmt.Errorf("injected error")}
	}
	s.fq.mu.Lock()
	val := int32(len(s.fq.createdIssues) + 1)
	s.fq.mu.Unlock()
	return &stubRow{vals: []any{val}}
}

func (s *stubDBTX) createIssueRow(args []any) pgx.Row {
	if s.fq.failOn == "CreateIssue" {
		return &stubRow{err: fmt.Errorf("injected error")}
	}
	s.fq.mu.Lock()
	idx := len(s.fq.createdIssues) + 1
	s.fq.mu.Unlock()

	id := pgtype.UUID{}
	copy(id.Bytes[:], []byte{0x11, 0x11, byte(idx), 0x01})
	id.Valid = true

	var wsID, parentID pgtype.UUID
	var title, status string
	var desc, assigneeType pgtype.Text
	var number int32
	if len(args) > 0 {
		wsID, _ = args[0].(pgtype.UUID)
	}
	if len(args) > 1 {
		title, _ = args[1].(string)
	}
	if len(args) > 2 {
		desc, _ = args[2].(pgtype.Text)
	}
	if len(args) > 3 {
		status, _ = args[3].(string)
	}
	if len(args) > 5 {
		assigneeType, _ = args[5].(pgtype.Text)
	}
	if len(args) > 9 {
		parentID, _ = args[9].(pgtype.UUID)
	}
	if len(args) > 12 {
		number, _ = args[12].(int32)
	}

	s.fq.mu.Lock()
	s.fq.createdIssues = append(s.fq.createdIssues, db.CreateIssueParams{
		WorkspaceID:   wsID,
		Title:         title,
		Description:   desc,
		Status:        status,
		AssigneeType:  assigneeType,
		ParentIssueID: parentID,
		Number:        number,
	})
	issue := db.Issue{
		ID:            id,
		WorkspaceID:   wsID,
		Title:         title,
		Description:   desc,
		Status:        status,
		ParentIssueID: parentID,
		Number:        number,
	}
	s.fq.issues[id] = issue
	s.fq.mu.Unlock()

	return &stubRow{vals: []any{
		issue.ID, issue.WorkspaceID, issue.Title, issue.Description,
		issue.Status, "none", pgtype.Text{}, pgtype.UUID{},
		"", pgtype.UUID{}, issue.ParentIssueID,
		[]byte(nil), []byte(nil), float64(0),
		pgtype.Timestamptz{}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
		issue.Number, "task", pgtype.UUID{},
	}}
}

func (s *stubDBTX) createIssueDependencyRow(args []any) pgx.Row {
	if s.fq.failOn == "CreateIssueDependency" {
		return &stubRow{err: fmt.Errorf("injected error")}
	}
	var issueID, dependsOnID pgtype.UUID
	var depType string
	if len(args) > 0 {
		issueID, _ = args[0].(pgtype.UUID)
	}
	if len(args) > 1 {
		dependsOnID, _ = args[1].(pgtype.UUID)
	}
	if len(args) > 2 {
		depType, _ = args[2].(string)
	}

	s.fq.mu.Lock()
	s.fq.issueDeps = append(s.fq.issueDeps, db.IssueDependency{
		IssueID:          issueID,
		DependsOnIssueID: dependsOnID,
		Type:             depType,
	})
	s.fq.mu.Unlock()

	depID := pgtype.UUID{}
	copy(depID.Bytes[:], []byte{0xdd, 0xdd, byte(len(s.fq.issueDeps)), 0x01})
	depID.Valid = true

	return &stubRow{vals: []any{depID, issueID, dependsOnID, depType}}
}

// ---------------------------------------------------------------------------
// fakeQueries — implements service.RunQuerier for RunOrchestrator
// ---------------------------------------------------------------------------

type fakeQueries struct {
	failOn        string
	agents        []db.Agent
	runs          map[pgtype.UUID]db.Run
	artifacts     []db.RunArtifact
	issues        map[pgtype.UUID]db.Issue
	issueDeps     []db.IssueDependency
	mu            sync.Mutex
	createdIssues []db.CreateIssueParams
	failRunCalls  int
	failRunIDs    []string
}

func newFakeQueries() *fakeQueries {
	return &fakeQueries{
		runs:   make(map[pgtype.UUID]db.Run),
		issues: make(map[pgtype.UUID]db.Issue),
	}
}

// --- RunQuerier interface methods ---

func (f *fakeQueries) CreateRun(_ context.Context, arg db.CreateRunParams) (db.Run, error) {
	if f.failOn == "CreateRun" {
		return db.Run{}, fmt.Errorf("injected error")
	}
	id := pgtype.UUID{}
	copy(id.Bytes[:], []byte{0xde, 0xca, byte(len(f.runs) + 1), 0x01})
	id.Valid = true
	run := db.Run{
		ID:          id,
		WorkspaceID: arg.WorkspaceID,
		IssueID:     arg.IssueID,
		AgentID:     arg.AgentID,
		Phase:       arg.Phase,
		Status:      arg.Status,
	}
	f.mu.Lock()
	f.runs[id] = run
	f.mu.Unlock()
	return run, nil
}

func (f *fakeQueries) StartRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	if f.failOn == "StartRun" {
		return db.Run{}, fmt.Errorf("injected error")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	run := f.runs[id]
	run.Phase = "executing"
	run.Status = "executing"
	f.runs[id] = run
	return run, nil
}

func (f *fakeQueries) GetRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	if f.failOn == "GetRun" {
		return db.Run{}, fmt.Errorf("injected error")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok {
		return db.Run{}, fmt.Errorf("run not found")
	}
	return run, nil
}

func (f *fakeQueries) FailRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	if f.failOn == "FailRun" {
		return db.Run{}, fmt.Errorf("injected error")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failRunCalls++
	f.failRunIDs = append(f.failRunIDs, uuidToString(id))
	run := f.runs[id]
	run.Phase = "failed"
	run.Status = "failed"
	f.runs[id] = run
	return run, nil
}

func (f *fakeQueries) CompleteRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	return db.Run{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) CancelRun(_ context.Context, id pgtype.UUID) (db.Run, error) {
	return db.Run{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) UpdateRunPhase(_ context.Context, arg db.UpdateRunPhaseParams) (db.Run, error) {
	return db.Run{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) UpdateRunTokens(_ context.Context, arg db.UpdateRunTokensParams) (db.Run, error) {
	return db.Run{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) GetNextStepSeq(_ context.Context, _ pgtype.UUID) (int32, error) {
	return 0, fmt.Errorf("not implemented")
}
func (f *fakeQueries) CreateRunStep(_ context.Context, arg db.CreateRunStepParams) (db.RunStep, error) {
	return db.RunStep{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) CompleteRunStep(_ context.Context, arg db.CompleteRunStepParams) (db.RunStep, error) {
	return db.RunStep{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) GetNextTodoSeq(_ context.Context, _ pgtype.UUID) (int32, error) {
	return 0, fmt.Errorf("not implemented")
}
func (f *fakeQueries) CreateRunTodo(_ context.Context, arg db.CreateRunTodoParams) (db.RunTodo, error) {
	return db.RunTodo{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) UpdateRunTodo(_ context.Context, arg db.UpdateRunTodoParams) (db.RunTodo, error) {
	return db.RunTodo{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) CreateRunHandoff(_ context.Context, arg db.CreateRunHandoffParams) (db.RunHandoff, error) {
	return db.RunHandoff{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) CreateRunContinuation(_ context.Context, arg db.CreateRunContinuationParams) (db.RunContinuation, error) {
	return db.RunContinuation{}, fmt.Errorf("not implemented")
}
func (f *fakeQueries) CreateRunArtifact(_ context.Context, arg db.CreateRunArtifactParams) (db.RunArtifact, error) {
	if f.failOn == "CreateRunArtifact" {
		return db.RunArtifact{}, fmt.Errorf("injected error")
	}
	artifact := db.RunArtifact{
		RunID:        arg.RunID,
		ArtifactType: arg.ArtifactType,
		Name:         arg.Name,
		Content:      arg.Content,
		MimeType:     arg.MimeType,
	}
	f.mu.Lock()
	f.artifacts = append(f.artifacts, artifact)
	f.mu.Unlock()
	return artifact, nil
}
func (f *fakeQueries) ListRunsByWorkspace(_ context.Context, arg db.ListRunsByWorkspaceParams) ([]db.Run, error) {
	return nil, nil
}
func (f *fakeQueries) ListRunsByIssue(_ context.Context, _ pgtype.UUID) ([]db.Run, error) {
	return nil, nil
}
func (f *fakeQueries) ListRunSteps(_ context.Context, _ pgtype.UUID) ([]db.RunStep, error) {
	return nil, nil
}
func (f *fakeQueries) ListRunTodos(_ context.Context, _ pgtype.UUID) ([]db.RunTodo, error) {
	return nil, nil
}
func (f *fakeQueries) ListRunArtifacts(_ context.Context, _ pgtype.UUID) ([]db.RunArtifact, error) {
	return nil, nil
}
func (f *fakeQueries) CreateRunEvent(_ context.Context, arg db.CreateRunEventParams) (db.RunEvent, error) {
	return db.RunEvent{}, nil
}
func (f *fakeQueries) ListRunEvents(_ context.Context, arg db.ListRunEventsParams) ([]db.RunEvent, error) {
	return nil, nil
}
func (f *fakeQueries) ListRunEventsAll(_ context.Context, arg db.ListRunEventsAllParams) ([]db.RunEvent, error) {
	return nil, nil
}
func (f *fakeQueries) ListRunArtifactsByType(_ context.Context, arg db.ListRunArtifactsByTypeParams) ([]db.RunArtifact, error) {
	if f.failOn == "ListRunArtifactsByType" {
		return nil, fmt.Errorf("injected error")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []db.RunArtifact
	for _, a := range f.artifacts {
		if a.ArtifactType == arg.ArtifactType {
			result = append(result, a)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Stub TxStarter + Tx
// ---------------------------------------------------------------------------

type stubTxStarter struct {
	dbtx *stubDBTX
}

func (s *stubTxStarter) Begin(_ context.Context) (pgx.Tx, error) {
	return &stubTx{dbtx: s.dbtx}, nil
}

type stubTx struct {
	dbtx       *stubDBTX
	committed  bool
	rolledBack bool
}

func (t *stubTx) Begin(_ context.Context) (pgx.Tx, error) { return t, nil }
func (t *stubTx) Commit(_ context.Context) error           { t.committed = true; return nil }
func (t *stubTx) Rollback(_ context.Context) error         { t.rolledBack = true; return nil }
func (t *stubTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}
func (t *stubTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *stubTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (t *stubTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, fmt.Errorf("not implemented")
}
func (t *stubTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *stubTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &stubRows{}, nil
}
func (t *stubTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	// Delegate to stubDBTX so qtx (Queries.WithTx) calls resolve properly.
	return t.dbtx.QueryRow(context.Background(), sql, args...)
}
func (t *stubTx) Conn() *pgx.Conn { return nil }

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func makeTestIssue(id pgtype.UUID, workspaceID pgtype.UUID, kind string) db.Issue {
	return db.Issue{
		ID:          id,
		WorkspaceID: workspaceID,
		Title:       "Test Issue",
		Description: pgtype.Text{String: "Test description", Valid: true},
		Status:      "todo",
		Priority:    "none",
		IssueKind:   kind,
		Number:      1,
		CreatorType: "member",
		CreatorID:   workspaceID,
	}
}

func makeTestAgent(id, workspaceID, runtimeID pgtype.UUID) db.Agent {
	return db.Agent{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        "Test Agent",
		RuntimeID:   runtimeID,
		Status:      "active",
	}
}

func testDecomposeHandler(fq *fakeQueries) *Handler {
	hub := realtime.NewHub([]string{"*"})
	go hub.Run()
	bus := events.New()

	dbtx := &stubDBTX{fq: fq}
	queries := db.New(dbtx)

	compactor := service.NewCompactor()
	ro := service.NewRunOrchestrator(fq, compactor, hub, bus)
	taskSvc := service.NewTaskService(queries, hub, bus)

	return &Handler{
		Queries:         queries,
		TxStarter:       &stubTxStarter{dbtx: dbtx},
		Hub:             hub,
		Bus:             bus,
		RunOrchestrator: ro,
		TaskService:     taskSvc,
		prefixCache:     sync.Map{},
	}
}

// ---------------------------------------------------------------------------
// HIGH-1: ExecuteRun error → FailRun called + run status failed
// ---------------------------------------------------------------------------

func TestDecompose_ExecuteRunError_CallsFailRun(t *testing.T) {
	fq := newFakeQueries()

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	runtimeID := pgtype.UUID{}
	copy(runtimeID.Bytes[:], []byte{0x03, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	runtimeID.Valid = true

	agentID := pgtype.UUID{}
	copy(agentID.Bytes[:], []byte{0x04, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	agentID.Valid = true

	fq.agents = []db.Agent{makeTestAgent(agentID, wsID, runtimeID)}
	fq.issues[issueID] = makeTestIssue(issueID, wsID, "goal")

	h := testDecomposeHandler(fq)

	// Inject StartRun failure so ExecuteRun fails in the goroutine.
	fq.failOn = "StartRun"

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/issues/"+uuidToString(issueID)+"/decompose", strings.NewReader(body))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose", h.Decompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}

	var resp DecomposeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "running" {
		t.Errorf("initial status = %q, want running", resp.Status)
	}

	// Wait for the async goroutine to execute.
	time.Sleep(500 * time.Millisecond)

	// Verify FailRun was called.
	fq.mu.Lock()
	calls := fq.failRunCalls
	fq.mu.Unlock()

	if calls == 0 {
		t.Error("expected FailRun to be called after ExecuteRun error, but it was not called")
	}
}

// ---------------------------------------------------------------------------
// HIGH-2: CreateIssueDependency error → 500 response + transaction rollback
// ---------------------------------------------------------------------------

func TestDecompose_ConfirmDependencyError_Returns500AndRollsBack(t *testing.T) {
	fq := newFakeQueries()
	fq.failOn = "CreateIssueDependency"

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	fq.issues[issueID] = makeTestIssue(issueID, wsID, "goal")

	h := testDecomposeHandler(fq)

	body := `{
		"subtasks": [
			{"title": "A", "description": "desc A", "deliverable": "del A", "depends_on": []},
			{"title": "B", "description": "desc B", "deliverable": "del B", "depends_on": [0]}
		]
	}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/issues/"+uuidToString(issueID)+"/decompose/confirm",
		strings.NewReader(body))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose/confirm", h.ConfirmDecompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp["error"] != "failed to create subtask dependency" {
		t.Errorf("error = %q, want 'failed to create subtask dependency'", resp["error"])
	}

	// Verify no dependencies were created (the stub returns error on first call).
	fq.mu.Lock()
	depCount := len(fq.issueDeps)
	fq.mu.Unlock()

	if depCount != 0 {
		t.Errorf("expected 0 dependencies created (transaction rolled back), got %d", depCount)
	}
}

// ---------------------------------------------------------------------------
// ConfirmDecompose success path (sanity check)
// ---------------------------------------------------------------------------

func TestDecompose_ConfirmSuccess_CreatesIssuesAndDeps(t *testing.T) {
	fq := newFakeQueries()

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	fq.issues[issueID] = makeTestIssue(issueID, wsID, "goal")

	h := testDecomposeHandler(fq)

	body := `{
		"subtasks": [
			{"title": "A", "description": "desc A", "deliverable": "del A", "depends_on": []},
			{"title": "B", "description": "desc B", "deliverable": "del B", "depends_on": [0]}
		]
	}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/issues/"+uuidToString(issueID)+"/decompose/confirm",
		strings.NewReader(body))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose/confirm", h.ConfirmDecompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	total, ok := resp["total"].(float64)
	if !ok || int(total) != 2 {
		t.Errorf("total = %v, want 2", resp["total"])
	}

	fq.mu.Lock()
	issueCount := len(fq.createdIssues)
	depCount := len(fq.issueDeps)
	fq.mu.Unlock()

	if issueCount != 2 {
		t.Errorf("created issues = %d, want 2", issueCount)
	}
	if depCount != 1 {
		t.Errorf("created dependencies = %d, want 1", depCount)
	}
}

// ---------------------------------------------------------------------------
// Decompose handler — validation paths
// ---------------------------------------------------------------------------

func TestDecompose_NonGoalIssue_Returns400(t *testing.T) {
	fq := newFakeQueries()

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	fq.issues[issueID] = makeTestIssue(issueID, wsID, "task") // not a goal

	h := testDecomposeHandler(fq)

	req := httptest.NewRequest(http.MethodPost, "/api/issues/"+uuidToString(issueID)+"/decompose", strings.NewReader("{}"))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose", h.Decompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "only goal") {
		t.Errorf("error = %q, want 'only goal'", resp["error"])
	}
}

func TestDecompose_NoAgents_Returns400(t *testing.T) {
	fq := newFakeQueries()
	fq.agents = nil

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	fq.issues[issueID] = makeTestIssue(issueID, wsID, "goal")

	h := testDecomposeHandler(fq)

	req := httptest.NewRequest(http.MethodPost, "/api/issues/"+uuidToString(issueID)+"/decompose", strings.NewReader("{}"))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose", h.Decompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDecompose_NoAgentWithRuntime_Returns400(t *testing.T) {
	fq := newFakeQueries()

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	agentID := pgtype.UUID{}
	copy(agentID.Bytes[:], []byte{0x04, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	agentID.Valid = true

	fq.agents = []db.Agent{{
		ID:          agentID,
		WorkspaceID: wsID,
		Name:        "No Runtime Agent",
		Status:      "active",
		// RuntimeID is zero (not valid)
	}}
	fq.issues[issueID] = makeTestIssue(issueID, wsID, "goal")

	h := testDecomposeHandler(fq)

	req := httptest.NewRequest(http.MethodPost, "/api/issues/"+uuidToString(issueID)+"/decompose", strings.NewReader("{}"))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose", h.Decompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDecompose_ConfirmEmptySubtasks_Returns400(t *testing.T) {
	fq := newFakeQueries()

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	fq.issues[issueID] = makeTestIssue(issueID, wsID, "goal")

	h := testDecomposeHandler(fq)
	h.TaskService = service.NewTaskService(db.New(&stubDBTX{fq: fq}), h.Hub, h.Bus)

	req := httptest.NewRequest(http.MethodPost,
		"/api/issues/"+uuidToString(issueID)+"/decompose/confirm",
		strings.NewReader(`{"subtasks":[]}`))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose/confirm", h.ConfirmDecompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDecompose_ConfirmInvalidJSON_Returns400(t *testing.T) {
	fq := newFakeQueries()

	wsID := pgtype.UUID{}
	copy(wsID.Bytes[:], []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	wsID.Valid = true

	issueID := pgtype.UUID{}
	copy(issueID.Bytes[:], []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	issueID.Valid = true

	fq.issues[issueID] = makeTestIssue(issueID, wsID, "goal")

	h := testDecomposeHandler(fq)

	req := httptest.NewRequest(http.MethodPost,
		"/api/issues/"+uuidToString(issueID)+"/decompose/confirm",
		strings.NewReader(`{invalid`))
	req.Header.Set("X-User-ID", uuidToString(wsID))
	req.Header.Set("X-Workspace-ID", uuidToString(wsID))

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/issues/{id}/decompose/confirm", h.ConfirmDecompose)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
