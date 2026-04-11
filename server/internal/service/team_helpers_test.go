package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestStrPtrToText_NilTeam(t *testing.T) {
	got := strPtrToText(nil)
	if got.Valid {
		t.Error("nil pointer should produce invalid Text")
	}
}

func TestStrPtrToText_Empty(t *testing.T) {
	s := ""
	got := strPtrToText(&s)
	if !got.Valid {
		t.Error("non-nil pointer should produce valid Text")
	}
	if got.String != "" {
		t.Errorf("expected empty string, got %q", got.String)
	}
}

func TestStrPtrToText_Value(t *testing.T) {
	s := "hello"
	got := strPtrToText(&s)
	if !got.Valid || got.String != "hello" {
		t.Errorf("expected valid 'hello', got %+v", got)
	}
}

func TestPtrToUUID_NilTeam(t *testing.T) {
	got := ptrToUUID(nil)
	if got.Valid {
		t.Error("nil pointer should produce invalid UUID")
	}
}

func TestPtrToUUID_Valid(t *testing.T) {
	u := pgtype.UUID{Bytes: [16]byte{1, 2, 3}, Valid: true}
	got := ptrToUUID(&u)
	if !got.Valid {
		t.Error("should produce valid UUID")
	}
	if got.Bytes != u.Bytes {
		t.Errorf("bytes mismatch: got %v, want %v", got.Bytes, u.Bytes)
	}
}
