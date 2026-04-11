package util

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// ParseUUID
// ---------------------------------------------------------------------------

func TestParseUUID_Valid(t *testing.T) {
	u := ParseUUID("550e8400-e29b-41d4-a716-446655440000")
	if !u.Valid {
		t.Error("expected Valid=true for valid UUID string")
	}
}

func TestParseUUID_Invalid(t *testing.T) {
	u := ParseUUID("not-a-uuid")
	if u.Valid {
		t.Error("expected Valid=false for invalid UUID string")
	}
}

func TestParseUUID_Empty(t *testing.T) {
	u := ParseUUID("")
	if u.Valid {
		t.Error("expected Valid=false for empty string")
	}
}

// ---------------------------------------------------------------------------
// UUIDToString
// ---------------------------------------------------------------------------

func TestUUIDToString_Valid(t *testing.T) {
	u := pgtype.UUID{
		Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00},
		Valid: true,
	}
	got := UUIDToString(u)
	want := "550e8400-e29b-41d4-a716-446655440000"
	if got != want {
		t.Errorf("UUIDToString() = %q, want %q", got, want)
	}
}

func TestUUIDToString_Invalid(t *testing.T) {
	got := UUIDToString(pgtype.UUID{})
	if got != "" {
		t.Errorf("UUIDToString(invalid) = %q, want empty", got)
	}
}

func TestUUIDToString_ZeroBytes(t *testing.T) {
	u := pgtype.UUID{Bytes: [16]byte{}, Valid: true}
	got := UUIDToString(u)
	if got != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("UUIDToString(zero) = %q", got)
	}
}

// ---------------------------------------------------------------------------
// TextToPtr
// ---------------------------------------------------------------------------

func TestTextToPtr_Valid(t *testing.T) {
	ptr := TextToPtr(pgtype.Text{String: "hello", Valid: true})
	if ptr == nil {
		t.Fatal("expected non-nil")
	}
	if *ptr != "hello" {
		t.Errorf("TextToPtr() = %q, want %q", *ptr, "hello")
	}
}

func TestTextToPtr_Invalid(t *testing.T) {
	ptr := TextToPtr(pgtype.Text{})
	if ptr != nil {
		t.Errorf("TextToPtr(invalid) should return nil, got %v", *ptr)
	}
}

func TestTextToPtr_ValidEmptyString(t *testing.T) {
	ptr := TextToPtr(pgtype.Text{String: "", Valid: true})
	if ptr == nil {
		t.Fatal("expected non-nil for Valid=true empty string")
	}
	if *ptr != "" {
		t.Errorf("TextToPtr() = %q, want empty", *ptr)
	}
}

// ---------------------------------------------------------------------------
// PtrToText
// ---------------------------------------------------------------------------

func TestPtrToText_NonNil(t *testing.T) {
	s := "world"
	got := PtrToText(&s)
	if !got.Valid {
		t.Error("expected Valid=true")
	}
	if got.String != "world" {
		t.Errorf("PtrToText() = %q, want %q", got.String, "world")
	}
}

func TestPtrToText_Nil(t *testing.T) {
	got := PtrToText(nil)
	if got.Valid {
		t.Error("expected Valid=false for nil pointer")
	}
}

// ---------------------------------------------------------------------------
// StrToText
// ---------------------------------------------------------------------------

func TestStrToText_NonEmpty(t *testing.T) {
	got := StrToText("hello")
	if !got.Valid {
		t.Error("expected Valid=true")
	}
	if got.String != "hello" {
		t.Errorf("StrToText() = %q, want %q", got.String, "hello")
	}
}

func TestStrToText_Empty(t *testing.T) {
	got := StrToText("")
	if got.Valid {
		t.Error("expected Valid=false for empty string")
	}
}

// ---------------------------------------------------------------------------
// TimestampToString
// ---------------------------------------------------------------------------

func TestTimestampToString_Valid(t *testing.T) {
	ts := pgtype.Timestamptz{
		Time:  time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC),
		Valid: true,
	}
	got := TimestampToString(ts)
	if got != "2026-03-15T10:30:00Z" {
		t.Errorf("TimestampToString() = %q, want RFC3339", got)
	}
}

func TestTimestampToString_Invalid(t *testing.T) {
	got := TimestampToString(pgtype.Timestamptz{})
	if got != "" {
		t.Errorf("TimestampToString(invalid) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// TimestampToPtr
// ---------------------------------------------------------------------------

func TestTimestampToPtr_Valid(t *testing.T) {
	ts := pgtype.Timestamptz{
		Time:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
	ptr := TimestampToPtr(ts)
	if ptr == nil {
		t.Fatal("expected non-nil")
	}
	if *ptr != "2026-01-01T00:00:00Z" {
		t.Errorf("TimestampToPtr() = %q", *ptr)
	}
}

func TestTimestampToPtr_Invalid(t *testing.T) {
	ptr := TimestampToPtr(pgtype.Timestamptz{})
	if ptr != nil {
		t.Errorf("TimestampToPtr(invalid) should return nil, got %v", *ptr)
	}
}

// ---------------------------------------------------------------------------
// UUIDToPtr
// ---------------------------------------------------------------------------

func TestUUIDToPtr_Valid(t *testing.T) {
	u := pgtype.UUID{
		Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00},
		Valid: true,
	}
	ptr := UUIDToPtr(u)
	if ptr == nil {
		t.Fatal("expected non-nil")
	}
	if *ptr != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("UUIDToPtr() = %q", *ptr)
	}
}

func TestUUIDToPtr_Invalid(t *testing.T) {
	ptr := UUIDToPtr(pgtype.UUID{})
	if ptr != nil {
		t.Errorf("UUIDToPtr(invalid) should return nil, got %v", *ptr)
	}
}
