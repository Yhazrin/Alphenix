package logger

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/lmittmann/tint"
)

// Init initializes the global slog logger.
// In production (APP_ENV=production or LOG_FORMAT=json), outputs structured JSON.
// Otherwise, outputs colored terminal text via tint.
// Reads LOG_LEVEL env var (debug, info, warn, error). Default: debug.
func Init() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	var handler slog.Handler
	if useJSON() {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level:      level,
			TimeFormat: "15:04:05.000",
		})
	}
	slog.SetDefault(slog.New(handler))
}

// NewLogger creates a named slog logger.
// Useful for standalone processes (daemon, migrate) that want a component prefix.
func NewLogger(component string) *slog.Logger {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	var handler slog.Handler
	if useJSON() {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level:      level,
			TimeFormat: "15:04:05.000",
		})
	}
	return slog.New(handler).With("component", component)
}

// RequestAttrs extracts request_id and user_id from an HTTP request
// for use in handler-level structured logging.
func RequestAttrs(r *http.Request) []any {
	attrs := make([]any, 0, 4)
	if rid := chimw.GetReqID(r.Context()); rid != "" {
		attrs = append(attrs, "request_id", rid)
	}
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		attrs = append(attrs, "user_id", uid)
	}
	return attrs
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

func useJSON() bool {
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		return true
	}
	return strings.ToLower(os.Getenv("APP_ENV")) == "production"
}
