package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// --- strPtrToText ---

func TestStrPtrToText_Nil(t *testing.T) {
	got := strPtrToText(nil)
	if got.Valid {
		t.Error("nil pointer should return invalid Text")
	}
	if got.String != "" {
		t.Errorf("nil pointer should return empty string, got %q", got.String)
	}
}

func TestStrPtrToText_NonNil(t *testing.T) {
	s := "hello"
	got := strPtrToText(&s)
	if !got.Valid {
		t.Error("non-nil pointer should return valid Text")
	}
	if got.String != "hello" {
		t.Errorf("got %q, want %q", got.String, "hello")
	}
}

func TestStrPtrToText_EmptyString(t *testing.T) {
	s := ""
	got := strPtrToText(&s)
	if !got.Valid {
		t.Error("pointer to empty string should still be valid")
	}
	if got.String != "" {
		t.Errorf("got %q, want empty", got.String)
	}
}

// --- ptrToUUID ---

func TestPtrToUUID_Nil(t *testing.T) {
	got := ptrToUUID(nil)
	if got.Valid {
		t.Error("nil pointer should return invalid UUID")
	}
}

func TestPtrToUUID_NonNil(t *testing.T) {
	u := pgtype.UUID{Valid: true}
	for i := range u.Bytes {
		u.Bytes[i] = byte(i)
	}
	got := ptrToUUID(&u)
	if !got.Valid {
		t.Error("non-nil pointer should return valid UUID")
	}
	if got != u {
		t.Error("should copy the UUID value exactly")
	}
}

// --- priorityToInt ---

func TestPriorityToInt_AllLevels(t *testing.T) {
	tests := []struct {
		p    string
		want int32
	}{
		{"urgent", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"", 0},
		{"unknown", 0},
		{"URGENT", 0}, // case-sensitive
		{"High", 0},
	}

	for _, tt := range tests {
		got := priorityToInt(tt.p)
		if got != tt.want {
			t.Errorf("priorityToInt(%q) = %d, want %d", tt.p, got, tt.want)
		}
	}
}
