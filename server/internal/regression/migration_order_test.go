// Package regression: migration ordering and dependency tests.
// Validates migration file numbering, up/down pairing, and forward-reference safety.
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

// migrationNum extracts the leading number from filenames like "042_issue_kind_and_mcp_servers.up.sql".
var migrationNum = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

func TestMigrationNumberingSequential(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(migrationDir, "*.sql"))
	if err != nil {
		t.Fatalf("cannot glob migration files: %v", err)
	}

	nums := make(map[int]string) // number -> first file seen
	var allNums []int

	for _, f := range files {
		base := filepath.Base(f)
		m := migrationNum.FindStringSubmatch(base)
		if m == nil {
			// Skip non-migration SQL files (e.g. _sqlc_agent_memory_schema.sql)
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if prev, exists := nums[n]; exists {
			prevVariant := "up"
			if strings.Contains(prev, ".down.") {
				prevVariant = "down"
			}
			currVariant := "up"
			if strings.Contains(base, ".down.") {
				currVariant = "down"
			}
			if prevVariant == currVariant {
				t.Errorf("duplicate migration number %03d: %s and %s", n, prev, base)
			}
		} else {
			nums[n] = base
			allNums = append(allNums, n)
		}
	}

	sort.Ints(allNums)

	// Check for gaps in numbering.
	if len(allNums) > 0 {
		for i := 1; i < len(allNums); i++ {
			if allNums[i] != allNums[i-1]+1 {
				t.Errorf("migration numbering gap: %03d -> %03d (missing %03d)",
					allNums[i-1], allNums[i], allNums[i-1]+1)
			}
		}
	}
}

func TestMigrationUpHasDown(t *testing.T) {
	upFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob up migration files: %v", err)
	}

	var missing []string
	for _, f := range upFiles {
		base := filepath.Base(f)
		downBase := strings.Replace(base, ".up.sql", ".down.sql", 1)
		downFile := filepath.Join(migrationDir, downBase)
		if _, err := os.Stat(downFile); os.IsNotExist(err) {
			missing = append(missing, "  "+base+" has no matching "+downBase)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("up migrations without matching down files:\n%s",
			strings.Join(missing, "\n"))
	}
}

func TestMigrationDownHasUp(t *testing.T) {
	downFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.down.sql"))
	if err != nil {
		t.Fatalf("cannot glob down migration files: %v", err)
	}

	var missing []string
	for _, f := range downFiles {
		base := filepath.Base(f)
		upBase := strings.Replace(base, ".down.sql", ".up.sql", 1)
		upFile := filepath.Join(migrationDir, upBase)
		if _, err := os.Stat(upFile); os.IsNotExist(err) {
			missing = append(missing, "  "+base+" has no matching "+upBase)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("down migrations without matching up files:\n%s",
			strings.Join(missing, "\n"))
	}
}

// dropTable matches: DROP TABLE [IF EXISTS] "table_name" or DROP TABLE table_name
var dropTable = regexp.MustCompile(`(?i)DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?"?(\w+)"?`)

// createTableIfNotExists matches CREATE TABLE IF NOT EXISTS
var createTableIfNotExists = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\s+"?(\w+)"?`)

func TestMigrationDownDropsCreatedTables(t *testing.T) {
	upFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob up migration files: %v", err)
	}

	var problems []string
	for _, f := range upFiles {
		base := filepath.Base(f)
		downBase := strings.Replace(base, ".up.sql", ".down.sql", 1)
		downFile := filepath.Join(migrationDir, downBase)

		upData, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		downData, err := os.ReadFile(downFile)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			continue
		}

		src := string(upData)

		// Collect tables created with IF NOT EXISTS (may pre-exist, skip).
		ifNotExistsTables := make(map[string]bool)
		for _, m := range createTableIfNotExists.FindAllStringSubmatch(src, -1) {
			ifNotExistsTables[strings.ToLower(m[1])] = true
		}

		// Collect all CREATE TABLE occurrences, skip IF NOT EXISTS ones.
		created := make(map[string]bool)
		for _, m := range createTable.FindAllStringSubmatch(src, -1) {
			table := strings.ToLower(m[1])
			if !ifNotExistsTables[table] {
				created[table] = true
			}
		}

		dropped := make(map[string]bool)
		for _, m := range dropTable.FindAllStringSubmatch(string(downData), -1) {
			dropped[strings.ToLower(m[1])] = true
		}

		for table := range created {
			if !dropped[table] {
				problems = append(problems, fmt.Sprintf("  %s: CREATE TABLE %s but down.sql doesn't DROP it", base, table))
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("up/down table mismatch (up creates table but down doesn't drop it):\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestMigrationFKReferencesValid(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("cannot glob migration files: %v", err)
	}

	allTables := make(map[string]string) // lowercase table -> created in file
	fkRef := regexp.MustCompile(`(?i)REFERENCES\s+"?(\w+)"?`)

	type fkRelation struct {
		fromTable string
		toTable   string
		file      string
	}
	var allFKs []fkRelation

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		base := filepath.Base(f)
		src := string(data)

		for _, m := range createTable.FindAllStringSubmatch(src, -1) {
			allTables[strings.ToLower(m[1])] = base
		}

		lines := strings.Split(src, "\n")
		currentTable := ""
		for _, line := range lines {
			if tm := createTable.FindStringSubmatch(line); tm != nil {
				currentTable = strings.ToLower(tm[1])
			}
			if fkms := fkRef.FindAllStringSubmatch(line, -1); fkms != nil {
				for _, fkm := range fkms {
					refTable := strings.ToLower(fkm[1])
					if currentTable != "" && !isSQLKeyword(refTable) {
						allFKs = append(allFKs, fkRelation{
							fromTable: currentTable,
							toTable:   refTable,
							file:      base,
						})
					}
				}
			}
		}
	}

	var problems []string
	for _, fk := range allFKs {
		if _, exists := allTables[fk.toTable]; !exists {
			problems = append(problems, fmt.Sprintf("  %s: table %s REFERENCES %s but %s is not created in any migration",
				fk.file, fk.fromTable, fk.toTable, fk.toTable))
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("FK references to non-existent tables:\n%s",
			strings.Join(problems, "\n"))
	}
}
