package daemon

import (
	"context"
	"testing"
	"time"
)

// ── envOrDefault ──────────────────────────────────────────────────────────

func TestEnvOrDefault_FromEnv(t *testing.T) {
	t.Setenv("TEST_HELPER_KEY", "from-env")
	got := envOrDefault("TEST_HELPER_KEY", "fallback")
	if got != "from-env" {
		t.Errorf("envOrDefault = %q, want %q", got, "from-env")
	}
}

func TestEnvOrDefault_Unset(t *testing.T) {
	t.Setenv("TEST_HELPER_UNSET", "")
	got := envOrDefault("TEST_HELPER_UNSET", "fallback")
	if got != "fallback" {
		t.Errorf("envOrDefault = %q, want %q", got, "fallback")
	}
}

func TestEnvOrDefault_TrimsWhitespace(t *testing.T) {
	t.Setenv("TEST_HELPER_WS", "  value  ")
	got := envOrDefault("TEST_HELPER_WS", "fallback")
	if got != "value" {
		t.Errorf("envOrDefault = %q, want %q", got, "value")
	}
}

func TestEnvOrDefault_WhitespaceOnly(t *testing.T) {
	t.Setenv("TEST_HELPER_WSONLY", "   ")
	got := envOrDefault("TEST_HELPER_WSONLY", "fallback")
	if got != "fallback" {
		t.Errorf("envOrDefault = %q, want %q (whitespace-only should use fallback)", got, "fallback")
	}
}

// ── durationFromEnv ───────────────────────────────────────────────────────

func TestDurationFromEnv_Valid(t *testing.T) {
	t.Setenv("TEST_DUR_KEY", "5s")
	got, err := durationFromEnv("TEST_DUR_KEY", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 5*time.Second {
		t.Errorf("duration = %v, want %v", got, 5*time.Second)
	}
}

func TestDurationFromEnv_Unset(t *testing.T) {
	t.Setenv("TEST_DUR_UNSET", "")
	got, err := durationFromEnv("TEST_DUR_UNSET", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 10*time.Second {
		t.Errorf("duration = %v, want %v", got, 10*time.Second)
	}
}

func TestDurationFromEnv_Invalid(t *testing.T) {
	t.Setenv("TEST_DUR_BAD", "notaduration")
	_, err := durationFromEnv("TEST_DUR_BAD", time.Second)
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestDurationFromEnv_Complex(t *testing.T) {
	t.Setenv("TEST_DUR_COMPLEX", "1h30m")
	got, err := durationFromEnv("TEST_DUR_COMPLEX", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 90*time.Minute {
		t.Errorf("duration = %v, want %v", got, 90*time.Minute)
	}
}

// ── intFromEnv ────────────────────────────────────────────────────────────

func TestIntFromEnv_Valid(t *testing.T) {
	t.Setenv("TEST_INT_KEY", "42")
	got, err := intFromEnv("TEST_INT_KEY", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("int = %d, want 42", got)
	}
}

func TestIntFromEnv_Unset(t *testing.T) {
	t.Setenv("TEST_INT_UNSET", "")
	got, err := intFromEnv("TEST_INT_UNSET", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 7 {
		t.Errorf("int = %d, want 7", got)
	}
}

func TestIntFromEnv_Invalid(t *testing.T) {
	t.Setenv("TEST_INT_BAD", "abc")
	_, err := intFromEnv("TEST_INT_BAD", 0)
	if err == nil {
		t.Error("expected error for invalid integer")
	}
}

func TestIntFromEnv_Negative(t *testing.T) {
	t.Setenv("TEST_INT_NEG", "-5")
	got, err := intFromEnv("TEST_INT_NEG", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != -5 {
		t.Errorf("int = %d, want -5", got)
	}
}

// ── sleepWithContext ──────────────────────────────────────────────────────

func TestSleepWithContext_CompletesNormally(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	err := sleepWithContext(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("sleep returned too early: %v", elapsed)
	}
}

func TestSleepWithContext_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := sleepWithContext(ctx, 10*time.Second)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSleepWithContext_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := sleepWithContext(ctx, 10*time.Second)
	if err == nil {
		t.Fatal("expected error from timed-out context")
	}
}
