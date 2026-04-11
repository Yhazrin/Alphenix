package service

import (
	"os"
	"testing"
)

func TestNewEmailService_NoAPIKey(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	svc := NewEmailService()
	if svc == nil {
		t.Fatal("NewEmailService should never return nil")
	}
	if svc.client != nil {
		t.Error("client should be nil when RESEND_API_KEY is not set")
	}
}

func TestNewEmailService_DefaultFromEmail(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("RESEND_FROM_EMAIL", "")
	svc := NewEmailService()
	if svc.fromEmail != "noreply@alphenix.ai" {
		t.Errorf("default from email = %q, want %q", svc.fromEmail, "noreply@alphenix.ai")
	}
}

func TestNewEmailService_CustomFromEmail(t *testing.T) {
	t.Setenv("RESEND_FROM_EMAIL", "custom@example.com")
	svc := NewEmailService()
	if svc.fromEmail != "custom@example.com" {
		t.Errorf("from email = %q, want %q", svc.fromEmail, "custom@example.com")
	}
}

func TestNewEmailService_WithAPIKey(t *testing.T) {
	if os.Getenv("TEST_EMAIL_INTEGRATION") == "" {
		t.Skip("set TEST_EMAIL_INTEGRATION to run email integration test")
	}
	t.Setenv("RESEND_API_KEY", "re_test_key")
	svc := NewEmailService()
	if svc.client == nil {
		t.Error("client should be non-nil when RESEND_API_KEY is set")
	}
}

func TestSendVerificationCode_DevFallback(t *testing.T) {
	svc := &EmailService{client: nil, fromEmail: "test@example.com"}
	// Should not error — falls back to stdout.
	err := svc.SendVerificationCode("user@example.com", "123456")
	if err != nil {
		t.Errorf("dev fallback should not return error, got: %v", err)
	}
}

func TestSendVerificationCode_NilClientNoPanic(t *testing.T) {
	svc := &EmailService{}
	err := svc.SendVerificationCode("user@example.com", "999999")
	if err != nil {
		t.Errorf("nil client should not error, got: %v", err)
	}
}
