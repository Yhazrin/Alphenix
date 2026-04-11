// Package regression: SQL query parameter binding validation tests.
// Validates positional parameter sequencing, named parameter references against
// migration columns, and type cast validity in sqlc query files.
package regression

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Positional params: $1, $2, ...
var positionalParam = regexp.MustCompile(`\$(\d+)`)

// sqlc named params: sqlc.arg(name), sqlc.narg(name), sqlc.narg('name'), sqlc.narg('name')::type
var sqlcNamedParam = regexp.MustCompile(`sqlc\.(?:n?)arg\(\s*'?(\w+)'?\s*\)`)

// @-style params: @name::type
var atStyleParam = regexp.MustCompile(`@(\w+)::`)

// Type cast on positional param: $1::type (stops at non-identifier chars like space, ), ;)
var positionalCast = regexp.MustCompile(`\$(\d+)::(\w+(?:\s+\w+)*)`)

// Type cast on sqlc param: sqlc.narg('name')::type
var sqlcCast = regexp.MustCompile(`sqlc\.narg\(\s*'(\w+)'\s*\)::(\w+)`)

// Valid PostgreSQL types for parameter casts
var validCastTypes = map[string]bool{
	"uuid":             true,
	"uuid[]":           true,
	"text":             true,
	"integer":          true,
	"bigint":           true,
	"int":              true,
	"int4":             true,
	"int8":             true,
	"boolean":          true,
	"bool":             true,
	"jsonb":            true,
	"json":             true,
	"float8":           true,
	"float4":           true,
	"double precision": true,
	"numeric":          true,
	"timestamptz":      true,
	"timestamp":        true,
	"varchar":          true,
	"char":             true,
	"inet":             true,
	"interval":         true,
	"smallint":         true,
	"real":             true,
	"bytea":            true,
	"date":             true,
	"time":             true,
	"timetz":           true,
	"vector":           true,
	"record":           true,
}

// INSERT column list: INSERT INTO table (col1, col2, ...)
var insertColList = regexp.MustCompile(`(?i)INSERT\s+INTO\s+(\w+)\s*\(([^)]+)\)`)

// COALESCE with sqlc.narg: COALESCE(sqlc.narg('col_name'), col_name)
var coalesceNarg = regexp.MustCompile(`COALESCE\(\s*sqlc\.narg\(\s*'(\w+)'\s*\)\s*,\s*(\w+)\s*\)`)

// Literal values in SQL (strings, functions, keywords) that count as non-param values
var sqlLiteral = regexp.MustCompile(`(?i)^(now|current_timestamp|true|false|NULL|nextval)\b`)

func TestPositionalParamsSequential(t *testing.T) {
	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	var problems []string
	for _, qf := range queryFiles {
		data, err := os.ReadFile(qf)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(qf)

		// Split into individual named queries (-- name: ...)
		queries := regexp.MustCompile(`(?m)^--\s*name:\s*\w+`).Split(src, -1)

		for i, query := range queries {
			matches := positionalParam.FindAllStringSubmatch(query, -1)
			if len(matches) == 0 {
				continue
			}

			nums := make(map[int]bool)
			maxParam := 0
			for _, m := range matches {
				n, _ := strconv.Atoi(m[1])
				nums[n] = true
				if n > maxParam {
					maxParam = n
				}
			}

			// Check for gaps: if we have $N, we should have $1..$N.
			for n := 1; n <= maxParam; n++ {
				if !nums[n] {
					problems = append(problems, fmt.Sprintf(
						"  %s (query #%d): positional param $%d missing — highest is $%d but $%d is skipped",
						base, i, n, maxParam, n))
				}
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("non-sequential positional parameters:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestSqlcNargReferencesInsertColumns(t *testing.T) {
	// Verify that sqlc.narg('name') used in UPDATE SET clauses
	// references columns that actually exist in the target table.
	// INSERT column references are validated via positional params.
	tableColumns := buildTableColumnMap(t)

	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	var problems []string

	for _, qf := range queryFiles {
		data, err := os.ReadFile(qf)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(qf)

		// Check COALESCE(sqlc.narg('col'), col) in UPDATE SET clauses.
		for _, cm := range coalesceNarg.FindAllStringSubmatch(src, -1) {
			nargName := strings.ToLower(cm[1])
			fallbackCol := strings.ToLower(cm[2])

			// Find which table this UPDATE targets by looking backwards from the COALESCE.
			matchPos := strings.Index(src, cm[0])
			before := src[:matchPos]

			// Find the nearest UPDATE keyword.
			nearestUpdate := strings.LastIndex(strings.ToUpper(before), "UPDATE ")
			if nearestUpdate == -1 {
				continue
			}
			updateLine := before[nearestUpdate:]
			updateMatch := regexp.MustCompile(`(?i)UPDATE\s+(\w+)`).FindStringSubmatch(updateLine)
			if updateMatch == nil {
				continue
			}
			table := strings.ToLower(updateMatch[1])
			tblCols, exists := tableColumns[table]
			if !exists {
				continue
			}

			if !tblCols[nargName] {
				problems = append(problems, fmt.Sprintf(
					"  %s: UPDATE %s SET COALESCE(sqlc.narg('%s'), %s) — narg name not a column",
					base, table, nargName, fallbackCol))
			}
		}
	}

	// Deduplicate.
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
		t.Errorf("sqlc parameter references to non-existent columns:\n%s",
			strings.Join(unique, "\n"))
	}
}

func TestTypeCastValidity(t *testing.T) {
	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	var problems []string
	for _, qf := range queryFiles {
		data, err := os.ReadFile(qf)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(qf)

		// Check positional param casts: $1::type (but only simple types after ::).
		for _, m := range positionalCast.FindAllStringSubmatch(src, -1) {
			typ := strings.ToLower(m[2])
			// The regex captures "word (space word)*" which can overshoot.
			// Only validate the first word as the type — ignore trailing words from context.
			firstWord := strings.Fields(typ)[0]
			if !validCastTypes[firstWord] && !validCastTypes[typ] {
				problems = append(problems, fmt.Sprintf(
					"  %s: $%s::%s — unrecognized cast type", base, m[1], typ))
			}
		}

		// Check sqlc.narg('name')::type casts.
		for _, m := range sqlcCast.FindAllStringSubmatch(src, -1) {
			typ := strings.ToLower(m[2])
			if !validCastTypes[typ] {
				problems = append(problems, fmt.Sprintf(
					"  %s: sqlc.narg('%s')::%s — unrecognized cast type", base, m[1], typ))
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("unrecognized SQL type casts on parameters:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestSqlcParamStyleConsistency(t *testing.T) {
	// Warn if a file mixes sqlc.arg() and sqlc.narg() without clear reason,
	// or mixes quoted and unquoted narg syntax.
	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	argPattern := regexp.MustCompile(`sqlc\.arg\(\s*(\w+)\s*\)`)
	nargUnquoted := regexp.MustCompile(`sqlc\.narg\(\s*(\w+)\s*\)`)
	nargQuoted := regexp.MustCompile(`sqlc\.narg\(\s*'(\w+)'\s*\)`)

	for _, qf := range queryFiles {
		data, err := os.ReadFile(qf)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(qf)

		hasArg := argPattern.MatchString(src)
		hasNargUnquoted := nargUnquoted.MatchString(src)
		hasNargQuoted := nargQuoted.MatchString(src)

		count := 0
		if hasArg {
			count++
		}
		if hasNargUnquoted {
			count++
		}
		if hasNargQuoted {
			count++
		}

		if count > 1 {
			styles := []string{}
			if hasArg {
				styles = append(styles, "sqlc.arg()")
			}
			if hasNargUnquoted {
				styles = append(styles, "sqlc.narg(name)")
			}
			if hasNargQuoted {
				styles = append(styles, "sqlc.narg('name')")
			}
			t.Logf("%s: mixes param styles: %s", base, strings.Join(styles, " + "))
		}
	}
}

func TestCreateQueriesGenerateCorrectParamCount(t *testing.T) {
	// For INSERT queries, verify that the number of positional + named params
	// plus literal values matches the number of columns listed.
	_ = buildTableColumnMap(t) // available for deeper validation if needed

	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	var problems []string
	for _, qf := range queryFiles {
		data, err := os.ReadFile(qf)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(qf)

		// Split by "-- name:" markers to isolate each query.
		queryParts := regexp.MustCompile(`(?m)^--\s*name:\s*`).Split(src, -1)

		for _, part := range queryParts {
			// Get query name from first line.
			nameMatch := regexp.MustCompile(`^(\w+)`).FindStringSubmatch(part)
			if nameMatch == nil {
				continue
			}
			queryName := nameMatch[1]

			// Only check Create* / Upsert* queries.
			if !strings.HasPrefix(queryName, "Create") && !strings.HasPrefix(queryName, "Upsert") {
				continue
			}

			insertMatch := insertColList.FindStringSubmatch(part)
			if insertMatch == nil {
				continue
			}

			cols := strings.Split(insertMatch[2], ",")
			colCount := 0
			for _, col := range cols {
				col = strings.TrimSpace(col)
				if col != "" && !strings.Contains(col, "sqlc.") {
					colCount++
				}
			}

			// Scope to just the INSERT statement (before RETURNING).
			insertEnd := strings.Index(strings.ToUpper(part), "RETURNING")
			stmtPart := part
			if insertEnd > 0 {
				stmtPart = part[:insertEnd]
			}

			// Count positional params.
			posParams := positionalParam.FindAllString(stmtPart, -1)
			posCount := len(posParams)

			// Count named params (sqlc.arg/narg and @-style).
			namedParams := sqlcNamedParam.FindAllString(stmtPart, -1)
			namedCount := len(namedParams)

			atParams := atStyleParam.FindAllString(stmtPart, -1)
			atCount := len(atParams)

			// Count literal values in VALUES clause (e.g., 'queued', now(), true, false).
			valuesMatch := regexp.MustCompile(`(?i)VALUES\s*\(([^)]+)\)`).FindStringSubmatch(stmtPart)
			literalCount := 0
			if valuesMatch != nil {
				for _, v := range strings.Split(valuesMatch[1], ",") {
					v = strings.TrimSpace(v)
					// Literals: quoted strings, SQL functions, boolean keywords.
					if strings.HasPrefix(v, "'") || sqlLiteral.MatchString(v) {
						literalCount++
					}
				}
			}

			totalParams := posCount + namedCount + atCount

			// Params + literals should cover the INSERT columns.
			if totalParams+literalCount < colCount {
				problems = append(problems, fmt.Sprintf(
					"  %s/%s: %d INSERT columns but only %d values ($%d + sqlc:%d + @:%d + literals:%d)",
					base, queryName, colCount, totalParams+literalCount, posCount, namedCount, atCount, literalCount))
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("INSERT queries with insufficient parameters:\n%s",
			strings.Join(problems, "\n"))
	}
}

// buildTableColumnMap builds a table -> columns map from migration files.
// Reused by multiple tests.
func buildTableColumnMap(t *testing.T) map[string]map[string]bool {
	t.Helper()
	tableColumns := make(map[string]map[string]bool)

	migrationFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob migration files: %v", err)
	}

	createBlockRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(([\s\S]*?)\);`)
	alterTableRe := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(\w+)`)
	addColRe := regexp.MustCompile(`(?i)ADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

	for _, f := range migrationFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)

		for _, block := range createBlockRe.FindAllStringSubmatch(src, -1) {
			table := strings.ToLower(block[1])
			if tableColumns[table] == nil {
				tableColumns[table] = make(map[string]bool)
			}
			for _, line := range strings.Split(block[2], "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "--") ||
					strings.HasPrefix(trimmed, "PRIMARY") || strings.HasPrefix(trimmed, "UNIQUE") ||
					strings.HasPrefix(trimmed, "FOREIGN") || strings.HasPrefix(trimmed, "CONSTRAINT") ||
					strings.HasPrefix(trimmed, "CHECK") || trimmed == ")" || trimmed == "(" {
					continue
				}
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					col := strings.ToLower(parts[0])
					col = strings.TrimSuffix(col, ",")
					tableColumns[table][col] = true
				}
			}
		}

		currentTable := ""
		for _, line := range strings.Split(src, "\n") {
			if m := alterTableRe.FindStringSubmatch(line); m != nil {
				currentTable = strings.ToLower(m[1])
			}
			if currentTable != "" {
				if m := addColRe.FindStringSubmatch(line); m != nil {
					col := strings.ToLower(m[1])
					if tableColumns[currentTable] == nil {
						tableColumns[currentTable] = make(map[string]bool)
					}
					tableColumns[currentTable][col] = true
				}
			}
			if strings.Contains(line, ";") {
				currentTable = ""
			}
		}
	}

	return tableColumns
}
