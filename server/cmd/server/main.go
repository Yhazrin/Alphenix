package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/internal/logger"
	"github.com/multica-ai/alphenix/server/internal/middleware"
	"github.com/multica-ai/alphenix/server/internal/realtime"
	"github.com/multica-ai/alphenix/server/internal/tool"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// outboxPollInterval defines how often the outbox worker checks for new events.
const outboxPollInterval = 5 * time.Second

func main() {
	logger.Init()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://alphenix:alphenix@localhost:5432/alphenix?sslmode=disable"
	}

	// Connect to database with configured connection pool.
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		slog.Error("unable to parse database URL", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("unable to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	bus := events.New()
	realtime.InitTicketStore()
	hub := realtime.NewHub(allowedOrigins())
	go hub.Run()
	registerListeners(bus, hub)

	queries := db.New(pool)

	// Set up outbox for reliable event delivery
	outboxRepo := events.NewOutboxRepository(pool)
	bus.WithOutbox(outboxRepo)

	// Start outbox worker for async external event delivery
	outboxWorker := events.NewOutboxWorker(outboxRepo, events.NoOpPublisher{}, outboxPollInterval)
	outboxCtx, outboxCancel := context.WithCancel(context.Background())
	go outboxWorker.Start(outboxCtx)

	// Handler priority is explicit: subscribers (10) → activity (20) → notifications (30).
	// Registration order within each tier does not matter — the bus sorts by priority.
	registerSubscriberListeners(bus, queries)
	registerActivityListeners(bus, queries)
	registerNotificationListeners(bus, queries)

	// Initialize MCP Client Manager — connects to active MCP servers and
	// registers their tools in the shared registry for agent use.
	toolRegistry := tool.DefaultRegistry()
	mcpManager := initMCPClientManager(ctx, queries, toolRegistry)

	r := NewRouter(pool, hub, bus, toolRegistry)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Start background sweepers.
	sweepCtx, sweepCancel := context.WithCancel(context.Background())
	go runRuntimeSweeper(sweepCtx, queries, bus)
	go runMemorySweeper(sweepCtx, queries)

	// Start PAT last-used update worker pool (3 workers, 256-deep buffer).
	middleware.StartPATUpdatePool(queries, 3)

	// Graceful shutdown
	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	sweepCancel()
	outboxCancel()
	shutdownMCPClientManager(mcpManager)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
