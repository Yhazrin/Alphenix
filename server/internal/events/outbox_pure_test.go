package events

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// ---------------------------------------------------------------------------
// signPayload
// ---------------------------------------------------------------------------

func TestSignPayload_Deterministic(t *testing.T) {
	secret := "my-webhook-secret"
	body := []byte(`{"event":"task.created","id":"123"}`)

	sig1 := signPayload(secret, body)
	sig2 := signPayload(secret, body)

	if sig1 != sig2 {
		t.Error("same input should produce same signature")
	}
}

func TestSignPayload_MatchesManualHMAC(t *testing.T) {
	secret := "test-secret"
	body := []byte(`hello world`)

	got := signPayload(secret, body)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Errorf("signPayload() = %q, want %q", got, want)
	}
}

func TestSignPayload_DifferentInputsDifferentOutput(t *testing.T) {
	secret := "s"
	sig1 := signPayload(secret, []byte("body1"))
	sig2 := signPayload(secret, []byte("body2"))

	if sig1 == sig2 {
		t.Error("different bodies should produce different signatures")
	}
}

func TestSignPayload_DifferentSecretsDifferentOutput(t *testing.T) {
	body := []byte(`same body`)
	sig1 := signPayload("secret1", body)
	sig2 := signPayload("secret2", body)

	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestSignPayload_EmptyBody(t *testing.T) {
	sig := signPayload("secret", []byte{})
	if sig == "" {
		t.Error("empty body should still produce a non-empty signature")
	}
	// HMAC-SHA256 of empty body is deterministic
	sig2 := signPayload("secret", []byte{})
	if sig != sig2 {
		t.Error("empty body signatures should match")
	}
}

func TestSignPayload_EmptySecret(t *testing.T) {
	sig := signPayload("", []byte("body"))
	if sig == "" {
		t.Error("empty secret should still produce a non-empty signature")
	}
}

func TestSignPayload_HexLength(t *testing.T) {
	// HMAC-SHA256 produces 32 bytes = 64 hex chars
	sig := signPayload("key", []byte("msg"))
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64 hex chars", len(sig))
	}
}

// ---------------------------------------------------------------------------
// matchesEventType
// ---------------------------------------------------------------------------

func TestMatchesEventType_EmptyPatterns(t *testing.T) {
	// Empty patterns means match all events
	if !matchesEventType("task.created", nil) {
		t.Error("nil patterns should match any event")
	}
	if !matchesEventType("task.created", []string{}) {
		t.Error("empty patterns should match any event")
	}
}

func TestMatchesEventType_ExactMatch(t *testing.T) {
	patterns := []string{"task.created", "task.updated", "task.deleted"}
	if !matchesEventType("task.created", patterns) {
		t.Error("should match when eventType is in patterns")
	}
	if !matchesEventType("task.deleted", patterns) {
		t.Error("should match when eventType is in patterns")
	}
}

func TestMatchesEventType_NoMatch(t *testing.T) {
	patterns := []string{"task.created", "task.updated"}
	if matchesEventType("issue.created", patterns) {
		t.Error("should not match when eventType is not in patterns")
	}
}

func TestMatchesEventType_SinglePattern(t *testing.T) {
	patterns := []string{"team.created"}
	if !matchesEventType("team.created", patterns) {
		t.Error("should match single pattern")
	}
	if matchesEventType("team.deleted", patterns) {
		t.Error("should not match different event with single pattern")
	}
}

func TestMatchesEventType_CaseSensitive(t *testing.T) {
	patterns := []string{"Task.Created"}
	if matchesEventType("task.created", patterns) {
		t.Error("matching should be case-sensitive")
	}
}

func TestMatchesEventType_EmptyEventType(t *testing.T) {
	patterns := []string{"task.created"}
	if matchesEventType("", patterns) {
		t.Error("empty event type should not match non-empty patterns")
	}
}

func TestMatchesEventType_EmptyEventTypeEmptyPatterns(t *testing.T) {
	// empty patterns → match all, including empty event type
	if !matchesEventType("", []string{}) {
		t.Error("empty patterns should match everything")
	}
}
