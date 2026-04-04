package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/multica-ai/multicode/server/pkg/migrations"
)

func main() {
	if len(os.Args) < 2 {
		slog.Info("Usage: go run ./cmd/migrate <up|down|status|redo>")
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

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("failed to set goose dialect", "error", err)
		os.Exit(1)
	}

	// Build migrations from embed FS. We can't use goose's built-in
	// filesystem collection because it treats NNN_name.up.sql and
	// NNN_name.down.sql as duplicate versions (both resolve to N).
	// Instead, we manually parse and group them into goose.Migration
	// entries with the correct up/down direction.
	migrationsList, err := buildMigrations(migrations.EmbedMigrations)
	if err != nil {
		slog.Error("failed to build migrations", "error", err)
		os.Exit(1)
	}

	slog.Info("Loaded migrations", "count", len(migrationsList))

	p, err := goose.NewProvider(goose.DialectPostgres, db, nil,
		goose.WithGoMigrations(migrationsList...),
	)
	if err != nil {
		slog.Error("failed to create goose provider", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	var results []*goose.MigrationResult
	switch command {
	case "up":
		results, err = p.Up(ctx)
	case "up-by-one":
		res, e := p.UpByOne(ctx)
		if e == nil {
			results = []*goose.MigrationResult{res}
		}
		err = e
	case "down":
		res, e := p.Down(ctx)
		if e == nil {
			results = []*goose.MigrationResult{res}
		}
		err = e
	case "down-to":
		if len(os.Args) < 3 {
			slog.Error("Usage: go run ./cmd/migrate down-to <version>")
			os.Exit(1)
		}
		v, e := strconv.ParseInt(os.Args[2], 10, 64)
		if e != nil {
			slog.Error("invalid version", "error", e)
			os.Exit(1)
		}
		results, err = p.DownTo(ctx, v)
	case "status":
		statuses, e := p.Status(ctx)
		if e != nil {
			err = e
			break
		}
		for _, s := range statuses {
			slog.Info("migration", "version", s.Source.Version, "state", s.State, "source", s.Source.Path)
		}
		return
	case "redo":
		res, e := p.Down(ctx)
		if e != nil {
			err = e
			break
		}
		results = append(results, res)
		upResults, e := p.Up(ctx)
		if e != nil {
			err = e
			break
		}
		results = append(results, upResults...)
	default:
		slog.Error("unknown command", "command", command)
		os.Exit(1)
	}

	if err != nil {
		slog.Error("migration failed", "command", command, "error", err)
		os.Exit(1)
	}

	for _, r := range results {
		slog.Info("migration", "version", r.Source.Version, "direction", r.Direction, "duration", r.Duration)
	}

	slog.Info("Done.")
}

// migrationFile represents a parsed migration filename.
type migrationFile struct {
	version   int64
	direction string // "up" or "down"
	path      string
}

var migrationRegex = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

// buildMigrations reads SQL files from the embed FS and constructs
// goose.Migration entries grouped by version with correct up/down direction.
func buildMigrations(fsys fs.FS) ([]*goose.Migration, error) {
	// Collect all .sql files from the "migrations" subdirectory.
	var files []migrationFile
	err := fs.WalkDir(fsys, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		matches := migrationRegex.FindStringSubmatch(base)
		if matches == nil {
			return nil
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid version in %s: %w", base, err)
		}
		direction := matches[3]
		files = append(files, migrationFile{
			version:   version,
			direction: direction,
			path:      path,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking migrations: %w", err)
	}

	// Group by version.
	type versionPair struct {
		upPath   string
		downPath string
	}
	pairs := make(map[int64]*versionPair)
	for _, f := range files {
		p, ok := pairs[f.version]
		if !ok {
			p = &versionPair{}
			pairs[f.version] = p
		}
		switch f.direction {
		case "up":
			if p.upPath != "" {
				return nil, fmt.Errorf("duplicate up migration for version %d: %s and %s", f.version, p.upPath, f.path)
			}
			p.upPath = f.path
		case "down":
			if p.downPath != "" {
				return nil, fmt.Errorf("duplicate down migration for version %d: %s and %s", f.version, p.downPath, f.path)
			}
			p.downPath = f.path
		}
	}

	// Sort versions.
	versions := make([]int64, 0, len(pairs))
	for v := range pairs {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	// Build migrations using RunTx (avoids Go type resolution issue with RunDB).
	result := make([]*goose.Migration, 0, len(versions))
	for _, v := range versions {
		p := pairs[v]

		var upFn, downFn *goose.GoFunc

		if p.upPath != "" {
			sqlBytes, err := fs.ReadFile(fsys, p.upPath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", p.upPath, err)
			}
			sqlContent := string(sqlBytes)
			path := p.upPath
			upFn = &goose.GoFunc{
				RunTx: makeSQLFunc(sqlContent, path),
			}
		}

		if p.downPath != "" {
			sqlBytes, err := fs.ReadFile(fsys, p.downPath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", p.downPath, err)
			}
			sqlContent := string(sqlBytes)
			path := p.downPath
			downFn = &goose.GoFunc{
				RunTx: makeSQLFunc(sqlContent, path),
			}
		}

		m := goose.NewGoMigration(v, upFn, downFn)
		base := filepath.Base(p.upPath)
		if base == "" && p.downPath != "" {
			base = filepath.Base(p.downPath)
		}
		m.Source = filepath.Join("migrations", base)
		result = append(result, m)
	}

	return result, nil
}

// makeSQLFunc creates a Go migration function that executes raw SQL within a transaction.
func makeSQLFunc(sqlContent, path string) func(ctx context.Context, tx *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, sqlContent); err != nil {
			return fmt.Errorf("executing %s: %w", path, err)
		}
		return nil
	}
}
