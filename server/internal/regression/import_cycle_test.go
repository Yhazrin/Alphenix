// Package regression: Go import cycle detection tests.
// Parses import statements from Go source files and verifies no circular dependencies exist.
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

const modulePrefix = "github.com/multica-ai/alphenix/server/internal/"

var importLine = regexp.MustCompile(`"` + regexp.QuoteMeta(modulePrefix) + `(\w+(?:/\w+)?)"`)

func fullPackagePath(dir string) string {
	return strings.TrimPrefix(dir, "internal/")
}

func TestNoImportCycles(t *testing.T) {
	deps := make(map[string]map[string]bool)

	err := filepath.Walk(filepath.Join("../../internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		dir := filepath.Dir(path)
		pkg := fullPackagePath(dir)

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if deps[pkg] == nil {
			deps[pkg] = make(map[string]bool)
		}

		for _, m := range importLine.FindAllStringSubmatch(string(content), -1) {
			importedPkg := m[1]
			if importedPkg != pkg {
				deps[pkg][importedPkg] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk internal directory: %v", err)
	}

	var problems []string
	for startPkg := range deps {
		visited := make(map[string]bool)
		path := []string{startPkg}

		var dfs func(current string) bool
		dfs = func(current string) bool {
			visited[current] = true
			for dep := range deps[current] {
				if dep == startPkg && len(path) > 1 {
					path = append(path, dep)
					problems = append(problems, "  cycle: "+strings.Join(path, " -> "))
					return true
				}
				if !visited[dep] {
					path = append(path, dep)
					if dfs(dep) {
						return true
					}
					path = path[:len(path)-1]
				}
			}
			return false
		}
		dfs(startPkg)
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("import cycles detected:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestPackageDependencyLayers(t *testing.T) {
	deps := make(map[string]map[string]bool)

	err := filepath.Walk(filepath.Join("../../internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		dir := filepath.Dir(path)
		pkg := fullPackagePath(dir)

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if deps[pkg] == nil {
			deps[pkg] = make(map[string]bool)
		}

		for _, m := range importLine.FindAllStringSubmatch(string(content), -1) {
			importedPkg := m[1]
			if importedPkg != pkg {
				deps[pkg][importedPkg] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk internal directory: %v", err)
	}

	layers := make(map[string]int)
	changed := true
	for changed {
		changed = false
		for pkg, pkgDeps := range deps {
			maxDepLayer := -1
			for dep := range pkgDeps {
				if l, ok := layers[dep]; ok {
					if l > maxDepLayer {
						maxDepLayer = l
					}
				}
			}
			newLayer := maxDepLayer + 1
			if newLayer > layers[pkg] {
				layers[pkg] = newLayer
				changed = true
			}
		}
	}

	t.Logf("package dependency layers:")
	layerGroups := make(map[int][]string)
	for pkg, layer := range layers {
		layerGroups[layer] = append(layerGroups[layer], pkg)
	}
	var layerNums []int
	for l := range layerGroups {
		layerNums = append(layerNums, l)
	}
	sort.Ints(layerNums)
	for _, l := range layerNums {
		sort.Strings(layerGroups[l])
		t.Logf("  layer %d: %s", l, strings.Join(layerGroups[l], ", "))
	}

	var violations []string
	for pkg, pkgDeps := range deps {
		pkgLayer := layers[pkg]
		for dep := range pkgDeps {
			if depLayer, ok := layers[dep]; ok {
				if depLayer >= pkgLayer {
					violations = append(violations, fmt.Sprintf("  %s (layer %d) imports %s (layer %d)",
						pkg, pkgLayer, dep, depLayer))
				}
			}
		}
	}
	sort.Strings(violations)

	if len(violations) > 0 {
		t.Errorf("layer violations (package imports from same or higher layer):\n%s",
			strings.Join(violations, "\n"))
	}
}
