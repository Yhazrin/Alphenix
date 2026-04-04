package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/multica-ai/multicode/server/pkg/migrations"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/migrate <up|down|status|redo>")
		os.Exit(1)
	}

	command := os.Args[1]

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://multicode:multicode@localhost:5432/multicode?sslmode=disable"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		slog.Error("unable to ping database", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Create a session-level advisory lock so concurrent migration runs don't
	// collide. The lock is held for the duration of the migration and released
	// automatically when the connection closes (even on crash).
	sessionLocker, err := lock.NewPostgresSessionLocker(
		lock.WithLockTimeout(5, 60), // retry every 5s, up to 60 times (5 min total)
	)
	if err != nil {
		slog.Error("failed to create session locker", "error", err)
		os.Exit(1)
	}

	migrationsFS, err := fs.Sub(migrations.EmbedMigrations, "migrations")
	if err != nil {
		slog.Error("failed to create sub-FS for migrations", "error", err)
		os.Exit(1)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrationsFS,
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		slog.Error("failed to create goose provider", "error", err)
		os.Exit(1)
	}

	switch command {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			slog.Error("migration up failed", "error", err)
			os.Exit(1)
		}
		for _, r := range results {
			slog.Info("migrated", "version", r.Source.Version, "direction", r.Direction, "duration", r.Duration)
		}
	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			slog.Error("migration down failed", "error", err)
			os.Exit(1)
		}
		slog.Info("rolled back", "version", result.Source.Version, "direction", result.Direction, "duration", result.Duration)
	case "redo":
		downResult, err := provider.Down(ctx)
		if err != nil {
			slog.Error("migration redo (down) failed", "error", err)
			os.Exit(1)
		}
		slog.Info("redo down", "version", downResult.Source.Version, "duration", downResult.Duration)
		upResults, err := provider.Up(ctx)
		if err != nil {
			slog.Error("migration redo (up) failed", "error", err)
			os.Exit(1)
		}
		for _, r := range upResults {
			slog.Info("redo up", "version", r.Source.Version, "duration", r.Duration)
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			slog.Error("migration status failed", "error", err)
			os.Exit(1)
		}
		for _, s := range statuses {
			fmt.Printf("%-6s  %d  %s\n", s.State, s.Source.Version, s.Source.Path)
		}
	default:
		fmt.Println("Usage: go run ./cmd/migrate <up|down|status|redo>")
		os.Exit(1)
	}

	fmt.Println("Done.")
}
