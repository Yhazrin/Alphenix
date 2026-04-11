package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestCfBase64Encode_Basic(t *testing.T) {
	got := cfBase64Encode([]byte("hello world"))
	// CloudFront encoding: standard base64 with +→-, =→_, /→~
	// "hello world" → "aGVsbG8gd29ybGQ=" → "aGVsbG8gd29ybGQ="
	if got == "" {
		t.Error("expected non-empty output")
	}
	// Should not contain standard base64 special chars
	for _, ch := range got {
		if ch == '+' || ch == '/' || ch == '=' {
			t.Errorf("CloudFront base64 should not contain %q, got %q", string(ch), got)
		}
	}
}

func TestCfBase64Encode_Replacements(t *testing.T) {
	// "many A's" produces base64 with + and = chars
	input := make([]byte, 3)
	for i := range input {
		input[i] = 0xfb
	}
	got := cfBase64Encode(input)
	// 0xFB 0xFB 0xFB → base64 "+/v7" → CloudFront "-~v7"
	if got != "-~v7" {
		t.Errorf("got %q, want %q", got, "-~v7")
	}
}

func TestCfBase64Encode_Empty(t *testing.T) {
	got := cfBase64Encode([]byte{})
	// empty → "" in standard base64, after replacer stays ""
	if got != "" {
		t.Errorf("empty input should produce empty output, got %q", got)
	}
}

func TestParseRSAPrivateKey_PKCS1(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	parsed, err := parseRSAPrivateKey(pemBytes)
	if err != nil {
		t.Fatalf("parseRSAPrivateKey failed: %v", err)
	}
	if parsed.N.Cmp(key.N) != 0 {
		t.Error("parsed key N does not match original")
	}
}

func TestParseRSAPrivateKey_PKCS8(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	derBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal PKCS8: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derBytes,
	})

	parsed, err := parseRSAPrivateKey(pemBytes)
	if err != nil {
		t.Fatalf("parseRSAPrivateKey failed: %v", err)
	}
	if parsed.N.Cmp(key.N) != 0 {
		t.Error("parsed key N does not match original")
	}
}

func TestParseRSAPrivateKey_NoPEMBlock(t *testing.T) {
	_, err := parseRSAPrivateKey([]byte("not a pem block"))
	if err == nil {
		t.Error("expected error for non-PEM input")
	}
}

func TestParseRSAPrivateKey_InvalidDER(t *testing.T) {
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("garbage"),
	})

	_, err := parseRSAPrivateKey(pemBytes)
	if err == nil {
		t.Error("expected error for invalid DER bytes")
	}
}
