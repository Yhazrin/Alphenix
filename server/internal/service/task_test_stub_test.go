package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
	pgvector_go "github.com/pgvector/pgvector-go"
)

// stubRow represents one row result returned by QueryRow.
type stubRow struct {
	values []any
	err    error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i >= len(r.values) {
			break
		}
		if r.values[i] == nil {
			continue
		}
		switch v := d.(type) {
		case *int32:
			*v = r.values[i].(int32)
		case *int64:
			*v = r.values[i].(int64)
		case *string:
			*v = r.values[i].(string)
		case *bool:
			*v = r.values[i].(bool)
		case *float64:
			*v = r.values[i].(float64)
		case *pgtype.UUID:
			*v = r.values[i].(pgtype.UUID)
		case *[]byte:
			if b, ok := r.values[i].([]byte); ok {
				*v = b
			}
		case *pgtype.Timestamptz:
			*v = r.values[i].(pgtype.Timestamptz)
		case *pgtype.Text:
			*v = r.values[i].(pgtype.Text)
		case *pgtype.Int4:
			*v = r.values[i].(pgtype.Int4)
		case *[]string:
			if s, ok := r.values[i].([]string); ok {
				*v = s
			}
		case *pgtype.Interval:
			*v = r.values[i].(pgtype.Interval)
		case *pgvector_go.Vector:
			*v = r.values[i].(pgvector_go.Vector)
		case *interface{}:
			*v = r.values[i]
		default:
			return fmt.Errorf("stubRow.Scan: unsupported type %T at index %d", d, i)
		}
	}
	return nil
}

// stubRows implements pgx.Rows for Query responses.
type stubRows struct {
	rows  [][]any
	idx   int
	err   error
	types []uint32
}

func (r *stubRows) Close() {}
func (r *stubRows) Err() error {
	return r.err
}
func (r *stubRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *stubRows) Next() bool {
	r.idx++
	return r.idx <= len(r.rows)
}
func (r *stubRows) Scan(dest ...any) error {
	if r.idx < 1 || r.idx > len(r.rows) {
		return fmt.Errorf("stubRows.Scan: index out of range")
	}
	row := r.rows[r.idx-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		if row[i] == nil {
				continue
			}
		switch v := d.(type) {
		case *int32:
			*v = row[i].(int32)
		case *int64:
			*v = row[i].(int64)
		case *string:
			*v = row[i].(string)
		case *bool:
			*v = row[i].(bool)
		case *float64:
			*v = row[i].(float64)
		case *pgtype.UUID:
			*v = row[i].(pgtype.UUID)
		case *[]byte:
			if b, ok := row[i].([]byte); ok {
				*v = b
			}
		case *pgtype.Timestamptz:
			*v = row[i].(pgtype.Timestamptz)
		case *pgtype.Text:
			*v = row[i].(pgtype.Text)
		case *pgtype.Int4:
			*v = row[i].(pgtype.Int4)
		case *pgvector_go.Vector:
			*v = row[i].(pgvector_go.Vector)
		case *interface{}:
			*v = row[i]
		case *float32:
			*v = row[i].(float32)
		default:
			return fmt.Errorf("stubRows.Scan: unsupported type %T at index %d", d, i)
		}
	}
	return nil
}
func (r *stubRows) RawValues() [][]byte { return nil }
func (r *stubRows) Values() ([]any, error) {
	if r.idx >= 1 && r.idx <= len(r.rows) {
		return r.rows[r.idx-1], nil
	}
	return nil, fmt.Errorf("stubRows.Values: index out of range")
}
func (r *stubRows) Conn() *pgx.Conn { return nil }

// stubDBTX implements the db.DTX interface for testing SelectRuntime and other
// methods that only use QueryRow/Query.
type stubDBTX struct {
	// queryRowQueues maps SQL substring matches to queues of stubRow results.
	queryRowQueues map[string][]stubRow
	// queryResponses maps SQL substring matches to Query row sets.
	queryResponses map[string][][]any
	// queryErr is returned by Query when set (simulates DB connection errors).
	queryErr error
}

func (s *stubDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (s *stubDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	for key, responses := range s.queryResponses {
		if strings.Contains(sql, key) {
			return &stubRows{rows: responses}, nil
		}
	}
	return &stubRows{}, nil
}

func (s *stubDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	for key, queue := range s.queryRowQueues {
		if strings.Contains(sql, key) {
			if len(queue) == 0 {
				return stubRow{err: fmt.Errorf("no more stub responses for %q", key)}
			}
			row := queue[0]
			s.queryRowQueues[key] = queue[1:]
			return row
		}
	}
	return stubRow{err: fmt.Errorf("no stub configured for SQL: %s", sql)}
}

// newTestTaskServiceWithDB creates a TaskService backed by the given stub.
func newTestTaskServiceWithDB(dbc db.DBTX) *TaskService {
	return &TaskService{
		Queries: db.New(dbc),
	}
}

// makeTestUUID creates a deterministic UUID for testing from a string prefix.
func makeTestUUID(prefix string) pgtype.UUID {
	id := [16]byte{}
	for i := 0; i < 16 && i < len(prefix); i++ {
		id[i] = prefix[i]
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}
