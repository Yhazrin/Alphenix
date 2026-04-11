// Package regression: database column ↔ migration consistency tests.
// Validates that SQL column references in sqlc query files match real DB columns.
package regression

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// createTableBlock captures: CREATE TABLE name ( body );
var createTableBlock = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(([\s\S]*?)\);`)

// alterTableAddCol matches: ALTER TABLE name ADD COLUMN col_name
var alterTableAddCol = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(\w+)\s+ADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

// insertCols matches: INSERT INTO table (col1, col2, ...)
var insertCols = regexp.MustCompile(`(?i)INSERT\s+INTO\s+(\w+)\s*\(([^)]+)\)`)

// updateSetCols matches: UPDATE table SET col1 = ..., col2 = ...
var updateSetCols = regexp.MustCompile(`(?i)UPDATE\s+(\w+)\s+SET\s+([^\n]+)`)

// returningCols matches: RETURNING col1, col2, ...
var returningCols = regexp.MustCompile(`(?i)RETURNING\s+([^\n]+)`)

func TestSqlQueryColumnsExistInMigrations(t *testing.T) {
	// Step 1: Build table -> columns map from migrations (CREATE TABLE + ALTER TABLE ADD COLUMN).
	tableColumns := make(map[string]map[string]bool)

	migrationFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob migration files: %v", err)
	}

	for _, f := range migrationFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)

		// CREATE TABLE blocks.
		for _, block := range createTableBlock.FindAllStringSubmatch(src, -1) {
			table := strings.ToLower(block[1])
			body := block[2]

			if tableColumns[table] == nil {
				tableColumns[table] = make(map[string]bool)
			}

			for _, line := range strings.Split(body, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "PRIMARY") ||
					strings.HasPrefix(trimmed, "UNIQUE") || strings.HasPrefix(trimmed, "FOREIGN") ||
					strings.HasPrefix(trimmed, "CONSTRAINT") || strings.HasPrefix(trimmed, "CHECK") ||
					trimmed == ")" || trimmed == "(" {
					continue
				}
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					colName := strings.ToLower(parts[0])
					colName = strings.TrimSuffix(colName, ",")
					tableColumns[table][colName] = true
				}
			}
		}

		// ALTER TABLE ADD COLUMN — handle multi-column ALTER TABLE blocks.
		alterTableMatch := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(\w+)`)
		addColMatch := regexp.MustCompile(`(?i)ADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
		currentAlterTable := ""
		for _, line := range strings.Split(src, "\n") {
			if m := alterTableMatch.FindStringSubmatch(line); m != nil {
				currentAlterTable = strings.ToLower(m[1])
			}
			if currentAlterTable != "" {
				if m := addColMatch.FindStringSubmatch(line); m != nil {
					col := strings.ToLower(m[1])
					if tableColumns[currentAlterTable] == nil {
						tableColumns[currentAlterTable] = make(map[string]bool)
					}
					tableColumns[currentAlterTable][col] = true
				}
			}
			// Reset on semicolon (end of statement).
			if strings.Contains(line, ";") {
				currentAlterTable = ""
			}
		}
	}


	// Step 2: Parse sqlc query files for column references.
	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	// SQL keywords to skip.
	skipWords := map[string]bool{
		"select": true, "where": true, "and": true, "or": true,
		"set": true, "values": true, "on": true, "from": true,
		"null": true, "is": true, "not": true, "now": true,
		"returning": true, "limit": true, "offset": true,
		"default": true, "true": true, "false": true,
		"distinct": true, "as": true, "count": true, "coalesce": true,
		"exists": true, "in": true, "like": true, "ilike": true,
		"asc": true, "desc": true, "order": true, "by": true,
		"group": true, "having": true, "case": true, "when": true,
		"then": true, "else": true, "end": true, "between": true,
		"array": true, "unnest": true, "any": true, "all": true,
		"union": true, "intersect": true, "except": true,
		"jsonb_agg": true, "jsonb_build_object": true,
		"row_number": true, "over": true, "partition": true,
		"string_agg": true,
	}

	var problems []string

	for _, qf := range queryFiles {
		data, err := os.ReadFile(qf)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(qf)

		// Check INSERT INTO table (col1, col2) references.
		for _, m := range insertCols.FindAllStringSubmatch(src, -1) {
			table := strings.ToLower(m[1])
			cols := strings.Split(m[2], ",")

			tblCols, exists := tableColumns[table]
			if !exists {
				problems = append(problems, fmt.Sprintf("  %s: INSERT INTO %s — table not found in migrations", base, table))
				continue
			}

			for _, col := range cols {
				col = strings.TrimSpace(strings.ToLower(col))
				// Remove parenthesized expressions like "sqlc.narg(xxx)".
				if strings.Contains(col, "(") || strings.Contains(col, "sqlc.") {
					continue
				}
				if col == "" || skipWords[col] {
					continue
				}
				if !tblCols[col] {
					problems = append(problems, fmt.Sprintf("  %s: INSERT INTO %s references column %q not in CREATE TABLE", base, table, col))
				}
			}
		}

		// Check UPDATE table SET col = ... references.
		for _, m := range updateSetCols.FindAllStringSubmatch(src, -1) {
			table := strings.ToLower(m[1])
			setClause := m[2]

			tblCols, exists := tableColumns[table]
			if !exists {
				continue // table may be checked by INSERT branch
			}

			// Extract column names before "=" signs.
			colAssign := regexp.MustCompile(`(\w+)\s*=`)
			for _, cm := range colAssign.FindAllStringSubmatch(setClause, -1) {
				col := strings.ToLower(cm[1])
				if skipWords[col] || col == "where" {
					continue
				}
				if !tblCols[col] {
					problems = append(problems, fmt.Sprintf("  %s: UPDATE %s SET references column %q not in CREATE TABLE", base, table, col))
				}
			}
		}
	}

	// Deduplicate and sort.
	seen := make(map[string]bool)
	var unique []string
	for _, p := range problems {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	sort.Strings(unique)

	if len(unique) > 0 {
		t.Errorf("sqlc query column references without matching migration columns:\n%s",
			strings.Join(unique, "\n"))
	}
}

func TestNoOrphanedAlterTableColumns(t *testing.T) {
	// Verify that ALTER TABLE ADD COLUMN targets tables that exist in CREATE TABLE.
	tables := make(map[string]bool)

	migrationFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob migration files: %v", err)
	}

	for _, f := range migrationFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)

		for _, block := range createTableBlock.FindAllStringSubmatch(src, -1) {
			tables[strings.ToLower(block[1])] = true
		}
	}

	var problems []string
	for _, f := range migrationFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		for _, m := range alterTableAddCol.FindAllStringSubmatch(src, -1) {
			table := strings.ToLower(m[1])
			if !tables[table] {
				problems = append(problems, fmt.Sprintf("  %s: ALTER TABLE %s — table not created in any migration", base, table))
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("ALTER TABLE targets non-existent tables:\n%s",
			strings.Join(problems, "\n"))
	}
}
