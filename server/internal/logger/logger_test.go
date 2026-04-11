package logger

import (
	"log/slog"
	"testing"
)

// ---------------------------------------------------------------------------
// parseLevel
// ---------------------------------------------------------------------------

func TestParseLevel_Debug(t *testing.T) {
	tests := []string{"", "debug", "DEBUG", "  debug  ", "unknown"}
	for _, s := range tests {
		if got := parseLevel(s); got != slog.LevelDebug {
			t.Errorf("parseLevel(%q) = %v, want LevelDebug", s, got)
		}
	}
}

func TestParseLevel_Info(t *testing.T) {
	tests := []string{"info", "INFO", " Info "}
	for _, s := range tests {
		if got := parseLevel(s); got != slog.LevelInfo {
			t.Errorf("parseLevel(%q) = %v, want LevelInfo", s, got)
		}
	}
}

func TestParseLevel_Warn(t *testing.T) {
	tests := []string{"warn", "warning", "WARN", "Warning"}
	for _, s := range tests {
		if got := parseLevel(s); got != slog.LevelWarn {
			t.Errorf("parseLevel(%q) = %v, want LevelWarn", s, got)
		}
	}
}

func TestParseLevel_Error(t *testing.T) {
	tests := []string{"error", "ERROR", "  error  "}
	for _, s := range tests {
		if got := parseLevel(s); got != slog.LevelError {
			t.Errorf("parseLevel(%q) = %v, want LevelError", s, got)
		}
	}
}
