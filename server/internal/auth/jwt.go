package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

const defaultJWTSecret = "multica-dev-secret-change-in-production"

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

func IsProduction() bool {
	return os.Getenv("APP_ENV") == "production"
}

func JWTSecret() []byte {
	jwtSecretOnce.Do(func() {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			if IsProduction() {
				panic("JWT_SECRET environment variable must be set in production")
			}
			secret = defaultJWTSecret
			slog.Warn("using default JWT secret; set JWT_SECRET environment variable for production")
		}
		jwtSecret = []byte(secret)
	})

	return jwtSecret
}

// GeneratePATToken creates a new personal access token: "mul_" + 40 random hex chars.
func GeneratePATToken() (string, error) {
	b := make([]byte, 20) // 20 bytes = 40 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate PAT token: %w", err)
	}
	return "mul_" + hex.EncodeToString(b), nil
}

// GenerateDaemonToken creates a new daemon auth token: "mdt_" + 40 random hex chars.
func GenerateDaemonToken() (string, error) {
	b := make([]byte, 20) // 20 bytes = 40 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate daemon token: %w", err)
	}
	return "mdt_" + hex.EncodeToString(b), nil
}

// HashToken returns the hex-encoded SHA-256 hash of a token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
