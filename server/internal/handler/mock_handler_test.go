package handler

// mock_handler_test.go — lightweight mock framework for httptest-based handler tests.
//
// This file provides stubs that let handler tests run without a real database
// or external services. It is meant for Tier 2 httptest integration tests where
// each test constructs its own Handler with controlled stubs.
//
// Usage:
//
//	mh := newMockHandler()
//	h := mh.h
//	mh.db.addQueryRow("SQL_KEY", val1, val2)   // configure stub QueryRow
//	mh.db.addQuery("SQL_KEY", rows)              // configure stub Query
//	mh.db.setExec("SQL_KEY", tag, err)           // configure stub Exec
//
// Reuses stubRow/stubRows/assignVal from decompose_integration_test.go.

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// mockDBTX — implements db.DBTX (also satisfies txStarter and dbExecutor)
// ---------------------------------------------------------------------------

type stubExecResult struct {
	tag pgconn.CommandTag
	err error
}

type mockDBTX struct {
	mu            sync.RWMutex
	queryRowQueue map[string][]*stubRow // SQL substring → queue of rows
	queryResults  map[string][][]any    // SQL substring → row sets
	execResults   map[string]stubExecResult
	beginFn       func(ctx context.Context) (pgx.Tx, error)
	txMock        *mockTx
}

func newMockDBTX() *mockDBTX {
	return &mockDBTX{
		queryRowQueue: make(map[string][]*stubRow),
		queryResults:  make(map[string][][]any),
		execResults:   make(map[string]stubExecResult),
	}
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, res := range m.execResults {
		if contains(sql, key) {
			return res.tag, res.err
		}
	}
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, rows := range m.queryResults {
		if contains(sql, key) {
			return &stubRows{data: rows}, nil
		}
	}
	return &stubRows{}, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, queue := range m.queryRowQueue {
		if contains(sql, key) {
			if len(queue) == 0 {
				return &stubRow{err: pgx.ErrNoRows}
			}
			row := queue[0]
			m.queryRowQueue[key] = queue[1:]
			return row
		}
	}
	return &stubRow{err: pgx.ErrNoRows}
}

func (m *mockDBTX) Begin(ctx context.Context) (pgx.Tx, error) {
	if m.beginFn != nil {
		return m.beginFn(ctx)
	}
	if m.txMock != nil {
		return m.txMock, nil
	}
	return &mockTx{}, nil
}

// addQueryRow registers a stubRow for the given SQL substring.
// Multiple calls queue up; each QueryRow consumes one.
func (m *mockDBTX) addQueryRow(sqlKey string, vals ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryRowQueue[sqlKey] = append(m.queryRowQueue[sqlKey], &stubRow{vals: vals})
}

// addQueryRowErr registers an error response for QueryRow.
func (m *mockDBTX) addQueryRowErr(sqlKey string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryRowQueue[sqlKey] = append(m.queryRowQueue[sqlKey], &stubRow{err: err})
}

// addQuery registers rows for Query.
func (m *mockDBTX) addQuery(sqlKey string, rows [][]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryResults[sqlKey] = rows
}

// setExec registers an Exec result.
func (m *mockDBTX) setExec(sqlKey string, tag pgconn.CommandTag, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execResults[sqlKey] = stubExecResult{tag: tag, err: err}
}

// ---------------------------------------------------------------------------
// mockTx — implements pgx.Tx
// ---------------------------------------------------------------------------

type mockTx struct {
	execErr     error
	queryErr    error
	queryRowRow pgx.Row
	commitErr   error
	rollbackErr error
}

func (t *mockTx) Begin(ctx context.Context) (pgx.Tx, error) { return &mockTx{}, nil }
func (t *mockTx) Commit(ctx context.Context) error           { return t.commitErr }
func (t *mockTx) Rollback(ctx context.Context) error         { return t.rollbackErr }
func (t *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *mockTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (t *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, t.execErr
}
func (t *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &stubRows{}, t.queryErr
}
func (t *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.queryRowRow != nil {
		return t.queryRowRow
	}
	return &stubRow{err: pgx.ErrNoRows}
}
func (t *mockTx) Conn() *pgx.Conn { return nil }

// ---------------------------------------------------------------------------
// mockHandler — orchestrates all stubs for a testable Handler
// ---------------------------------------------------------------------------

type mockHandler struct {
	db *mockDBTX
	tx *mockTx
	h  *Handler
}

func newMockHandler() *mockHandler {
	dbtx := newMockDBTX()
	tx := &mockTx{}
	dbtx.txMock = tx

	hub := realtime.NewHub([]string{"*"})
	go hub.Run()
	bus := events.New()

	h := &Handler{
		Queries:     db.New(dbtx),
		DB:          dbtx,
		TxStarter:   dbtx,
		Hub:         hub,
		Bus:         bus,
		PingStore:   NewPingStore(),
		UpdateStore: NewUpdateStore(),
		prefixCache: sync.Map{},
	}

	return &mockHandler{db: dbtx, tx: tx, h: h}
}

// ---------------------------------------------------------------------------
// Chi router helper for httptest
// ---------------------------------------------------------------------------

// newTestChiRequest creates an *http.Request with chi URL params injected.
func newTestChiRequest(method, path string, body any, urlParams map[string]string) *http.Request {
	req := newRequest(method, path, body)
	if len(urlParams) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range urlParams {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	return req
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
