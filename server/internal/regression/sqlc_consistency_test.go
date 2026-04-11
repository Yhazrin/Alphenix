// Package regression: sqlc <-> migration cross-reference tests.
// Validates that sqlc-generated code stays in sync with migrations and query sources.
package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	migrationDir = "../../migrations"
	queryDir     = "../../pkg/db/queries"
	generatedDir = "../../pkg/db/generated"
)

// structDecl matches: type SomeStruct struct {
var structDecl = regexp.MustCompile(`^type\s+(\w+)\s+struct\s*\{`)

// createTable matches: CREATE TABLE [IF NOT EXISTS] "table_name" or CREATE TABLE table_name
var createTable = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?"?(\w+)"?`)

func TestSqlcModelsHaveMigrations(t *testing.T) {
	modelsFile := filepath.Join(generatedDir, "models.go")
	content, err := os.ReadFile(modelsFile)
	if err != nil {
		t.Fatalf("cannot read %s: %v", modelsFile, err)
	}

	var structs []string
	for _, line := range strings.Split(string(content), "\n") {
		if m := structDecl.FindStringSubmatch(line); m != nil {
			structs = append(structs, m[1])
		}
	}

	migrationFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob migration files: %v", err)
	}

	tables := make(map[string]string) // lowercase table name -> migration file
	for _, f := range migrationFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("cannot read %s: %v", f, err)
			continue
		}
		for _, m := range createTable.FindAllStringSubmatch(string(data), -1) {
			tables[strings.ToLower(m[1])] = filepath.Base(f)
		}
	}

	var problems []string
	for _, s := range structs {
		snake := camelToSnake(s)
		singular := strings.TrimSuffix(snake, "s")
		plural := snake + "s"

		if _, ok := tables[snake]; ok {
			continue
		}
		if _, ok := tables[singular]; ok {
			continue
		}
		if _, ok := tables[plural]; ok {
			continue
		}
		problems = append(problems, "  sqlc struct "+s+" -> tried \""+snake+"\", \""+singular+"\", \""+plural+"\" -- no matching CREATE TABLE found")
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("sqlc model structs without matching migration tables:\n%s",
			strings.Join(problems, "\n"))
	}
}

// camelToSnake converts CamelCase to snake_case.
func camelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func TestSqlcQueryFilesHaveGeneratedCode(t *testing.T) {
	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	var missing []string
	for _, qf := range queryFiles {
		base := filepath.Base(qf)
		genFile := filepath.Join(generatedDir, base+".go")
		if _, err := os.Stat(genFile); os.IsNotExist(err) {
			missing = append(missing, "  query "+base+" has no generated "+base+".go")
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("sqlc query files missing generated Go code (run 'sqlc generate'):\n%s",
			strings.Join(missing, "\n"))
	}
}

func TestGeneratedCodeNotOrphaned(t *testing.T) {
	genFiles, err := filepath.Glob(filepath.Join(generatedDir, "*.sql.go"))
	if err != nil {
		t.Fatalf("cannot glob generated files: %v", err)
	}

	var orphans []string
	for _, gf := range genFiles {
		base := filepath.Base(gf)
		stem := strings.TrimSuffix(base, ".go")
		queryFile := filepath.Join(queryDir, stem)
		if _, err := os.Stat(queryFile); os.IsNotExist(err) {
			orphans = append(orphans, "  generated "+base+" has no source query file "+stem)
		}
	}
	sort.Strings(orphans)

	if len(orphans) > 0 {
		t.Errorf("orphaned generated files (query source deleted but generated code remains — run 'sqlc generate'):\n%s",
			strings.Join(orphans, "\n"))
	}
}

// tableFromSQLRef matches: FROM|INTO|UPDATE <name>  (with optional quotes)
var tableFromSQLRef = regexp.MustCompile(`(?i)(?:FROM|INTO|UPDATE)\s+"?(\w+)"?`)

func isSQLKeyword(s string) bool {
	keywords := map[string]bool{
		"select": true, "where": true, "and": true, "or": true,
		"set": true, "values": true, "on": true, "join": true,
		"inner": true, "left": true, "right": true, "outer": true,
		"order": true, "group": true, "by": true, "having": true,
		"limit": true, "offset": true, "as": true, "is": true,
		"not": true, "null": true, "in": true, "exists": true,
		"case": true, "when": true, "then": true, "else": true,
		"end": true, "distinct": true, "between": true, "like": true,
		"ilike": true, "similar": true, "to": true, "returning": true,
		"conflict": true, "do": true, "nothing": true, "using": true,
		"array": true, "unnest": true, "coalesce": true, "count": true,
		"sum": true, "avg": true, "min": true, "max": true,
		"lateral": true, "cross": true, "natural": true,
		"window": true, "partition": true, "over": true, "rows": true,
		"range": true, "groups": true, "unbounded": true, "preceding": true,
		"following": true, "current": true, "row": true,
		"first": true, "last": true, "filter": true,
		"started_at": true,
		"skip": true, "the": true, "for": true, "update": true,
		"no": true, "other": true, "already": true,
	}
	return keywords[s]
}

func TestMigrationTablesHaveSqlcModels(t *testing.T) {
	modelsFile := filepath.Join(generatedDir, "models.go")
	content, err := os.ReadFile(modelsFile)
	if err != nil {
		t.Fatalf("cannot read %s: %v", modelsFile, err)
	}

	sqlcTables := make(map[string]string) // snake_case table -> struct name
	for _, line := range strings.Split(string(content), "\n") {
		if m := structDecl.FindStringSubmatch(line); m != nil {
			snake := camelToSnake(m[1])
			sqlcTables[snake] = m[1]
			sqlcTables[strings.TrimSuffix(snake, "s")] = m[1]
			sqlcTables[snake+"s"] = m[1]
		}
	}

	migrationFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob migration files: %v", err)
	}

	migrationTables := make(map[string]string) // table -> migration file
	for _, f := range migrationFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range createTable.FindAllStringSubmatch(string(data), -1) {
			migrationTables[strings.ToLower(m[1])] = filepath.Base(f)
		}
	}

	queryFiles, err := filepath.Glob(filepath.Join(queryDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob query files: %v", err)
	}

	tablesReferencedInQueries := make(map[string]bool)
	for _, f := range queryFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range tableFromSQLRef.FindAllStringSubmatch(string(data), -1) {
			table := strings.ToLower(m[1])
			if !isSQLKeyword(table) {
				tablesReferencedInQueries[table] = true
			}
		}
	}

	var problems []string
	for table := range tablesReferencedInQueries {
		if _, inMigrations := migrationTables[table]; !inMigrations {
			problems = append(problems, "  table \""+table+"\" referenced in sqlc queries but no CREATE TABLE in migrations")
			continue
		}
		if _, inSqlc := sqlcTables[table]; !inSqlc {
			problems = append(problems, "  table \""+table+"\" in migrations and queries but no sqlc model struct (run 'sqlc generate')")
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("migration <-> sqlc model mismatch:\n%s",
			strings.Join(problems, "\n"))
	}
}
