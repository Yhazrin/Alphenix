package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// HashToken
// ---------------------------------------------------------------------------

func TestHashToken_Deterministic(t *testing.T) {
	got1 := HashToken("test-token")
	got2 := HashToken("test-token")
	if got1 != got2 {
		t.Error("HashToken should be deterministic")
	}
}

func TestHashToken_CorrectSHA256(t *testing.T) {
	token := "mul_abcdef1234567890"
	h := sha256.Sum256([]byte(token))
	want := hex.EncodeToString(h[:])
	if got := HashToken(token); got != want {
		t.Errorf("HashToken() = %q, want %q", got, want)
	}
}

func TestHashToken_EmptyString(t *testing.T) {
	h := sha256.Sum256([]byte(""))
	want := hex.EncodeToString(h[:])
	if got := HashToken(""); got != want {
		t.Errorf("HashToken(\"\") = %q, want %q", got, want)
	}
}

func TestHashToken_DifferentInputsDifferentOutput(t *testing.T) {
	h1 := HashToken("token-a")
	h2 := HashToken("token-b")
	if h1 == h2 {
		t.Error("different tokens should produce different hashes")
	}
}

func TestHashToken_OutputLength(t *testing.T) {
	got := HashToken("anything")
	if len(got) != 64 {
		t.Errorf("HashToken output length = %d, want 64 (SHA-256 hex)", len(got))
	}
}

// ---------------------------------------------------------------------------
// GeneratePATToken
// ---------------------------------------------------------------------------

func TestGeneratePATToken_Prefix(t *testing.T) {
	token, err := GeneratePATToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(token, "mul_") {
		t.Errorf("PAT token should start with 'mul_', got %q", token)
	}
}

func TestGeneratePATToken_Length(t *testing.T) {
	token, err := GeneratePATToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "mul_" (4) + 40 hex chars = 44
	if len(token) != 44 {
		t.Errorf("PAT token length = %d, want 44", len(token))
	}
}

func TestGeneratePATToken_HexPart(t *testing.T) {
	token, err := GeneratePATToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hexPart := token[4:]
	if _, err := hex.DecodeString(hexPart); err != nil {
		t.Errorf("PAT token hex part is not valid hex: %q", hexPart)
	}
}

func TestGeneratePATToken_Uniqueness(t *testing.T) {
	t1, _ := GeneratePATToken()
	t2, _ := GeneratePATToken()
	if t1 == t2 {
		t.Error("two generated PAT tokens should be different")
	}
}

// ---------------------------------------------------------------------------
// GenerateDaemonToken
// ---------------------------------------------------------------------------

func TestGenerateDaemonToken_Prefix(t *testing.T) {
	token, err := GenerateDaemonToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(token, "mdt_") {
		t.Errorf("daemon token should start with 'mdt_', got %q", token)
	}
}

func TestGenerateDaemonToken_Length(t *testing.T) {
	token, err := GenerateDaemonToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "mdt_" (4) + 40 hex chars = 44
	if len(token) != 44 {
		t.Errorf("daemon token length = %d, want 44", len(token))
	}
}

func TestGenerateDaemonToken_HexPart(t *testing.T) {
	token, err := GenerateDaemonToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hexPart := token[4:]
	if _, err := hex.DecodeString(hexPart); err != nil {
		t.Errorf("daemon token hex part is not valid hex: %q", hexPart)
	}
}

func TestGenerateDaemonToken_Uniqueness(t *testing.T) {
	t1, _ := GenerateDaemonToken()
	t2, _ := GenerateDaemonToken()
	if t1 == t2 {
		t.Error("two generated daemon tokens should be different")
	}
}

// ---------------------------------------------------------------------------
// IsProduction
// ---------------------------------------------------------------------------

func TestIsProduction_DefaultFalse(t *testing.T) {
	// Default (no APP_ENV set) should return false.
	t.Setenv("APP_ENV", "")
	if IsProduction() {
		t.Error("expected IsProduction()=false when APP_ENV is empty")
	}
}

func TestIsProduction_ExplicitProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	if !IsProduction() {
		t.Error("expected IsProduction()=true when APP_ENV=production")
	}
}

func TestIsProduction_NonProductionValues(t *testing.T) {
	for _, val := range []string{"staging", "development", "dev", "test", "PRODUCTION"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv("APP_ENV", val)
			if IsProduction() {
				t.Errorf("expected IsProduction()=false for APP_ENV=%q", val)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JWTSecret (default path — sync.Once limits us to one test per process)
// ---------------------------------------------------------------------------

func TestJWTSecret_DefaultInNonProduction(t *testing.T) {
	// This test relies on JWTSecret() already being called with the default
	// path (no JWT_SECRET, non-production). Since sync.Once prevents
	// re-initialization, we just verify the returned value is non-nil.
	t.Setenv("APP_ENV", "")
	secret := JWTSecret()
	if secret == nil {
		t.Error("JWTSecret() should never return nil")
	}
	if len(secret) == 0 {
		t.Error("JWTSecret() should not return empty bytes")
	}
}

// ---------------------------------------------------------------------------
// GeneratePATToken / GenerateDaemonToken format validation
// ---------------------------------------------------------------------------

func TestGeneratePATToken_ValidHexSuffix(t *testing.T) {
	token, err := GeneratePATToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After "mul_", remaining 40 chars must be valid lowercase hex.
	hexPart := token[4:]
	if len(hexPart) != 40 {
		t.Errorf("hex part length = %d, want 40", len(hexPart))
	}
	for i, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex char %q at position %d", string(c), i)
			break
		}
	}
}

func TestGenerateDaemonToken_ValidHexSuffix(t *testing.T) {
	token, err := GenerateDaemonToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hexPart := token[4:]
	if len(hexPart) != 40 {
		t.Errorf("hex part length = %d, want 40", len(hexPart))
	}
	for i, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex char %q at position %d", string(c), i)
			break
		}
	}
}
