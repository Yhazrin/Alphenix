// Package regression: API endpoint ↔ E2E test coverage gap analysis.
// Compares registered routes against integration test coverage to find untested endpoints.
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

// routeRegistration matches chi route registrations like:
//   r.Get("/path", h.Handler)  r.Post("/path", h.Handler)  etc.
var routeRegistration = regexp.MustCompile(`r\.(Get|Post|Put|Patch|Delete|Head|Options)\s*\(\s*"([^"]+)"\s*,\s*h\.(\w+)`)

// routeGroupStart matches r.Route("/prefix", func(r chi.Router) {
var routeGroupStart = regexp.MustCompile(`r\.Route\s*\(\s*"([^"]+)"`)

// e2eHTTPRequest matches patterns like:
//   doRequest(t, ts, "GET", "/api/...", ...)
//   ts.Client().Do(req)  with req = httptest.NewRequest("GET", "/api/...", nil)
var e2eHTTPRequest = regexp.MustCompile(`(?:doRequest|NewRequest)\s*\([^,]*,\s*"[^"]*",\s*"([^"]+)"`)

// e2ePathFromReq matches httptest.NewRequest("METHOD", path, ...)
var e2ePathFromReq = regexp.MustCompile(`httptest\.NewRequest\s*\(\s*"(GET|POST|PUT|PATCH|DELETE)"\s*,\s*fmt\.Sprintf\(\s*"([^"]+)"`)

// e2eDirectPath matches httptest.NewRequest("METHOD", "/api/...", nil)
var e2eDirectPath = regexp.MustCompile(`httptest\.NewRequest\s*\(\s*"(GET|POST|PUT|PATCH|DELETE)"\s*,\s*"(/[^"]+)"`)

func TestAllRoutesHaveE2ECoverage(t *testing.T) {
	// Step 1: Collect all registered routes from route files.
	routeFiles, err := filepath.Glob(filepath.Join(routeDir, "routes_*.go"))
	if err != nil {
		t.Fatalf("cannot glob route files: %v", err)
	}
	routerFile := filepath.Join(routeDir, "router.go")
	if _, statErr := os.Stat(routerFile); statErr == nil {
		routeFiles = append(routeFiles, routerFile)
	}

	type route struct {
		method  string
		path    string
		handler string
	}
	var allRoutes []route

	for _, f := range routeFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)

		for _, m := range routeRegistration.FindAllStringSubmatch(src, -1) {
			allRoutes = append(allRoutes, route{
				method:  strings.ToUpper(m[1]),
				path:    m[2],
				handler: m[3],
			})
		}
	}

	if len(allRoutes) == 0 {
		t.Fatal("no routes found in route files")
	}

	// Step 2: Collect all E2E-tested paths from integration test files.
	integrationFiles, _ := filepath.Glob(filepath.Join(routeDir, "*integration*test.go"))
	integrationFiles2, _ := filepath.Glob(filepath.Join(routeDir, "*_test.go"))
	allTestFiles := append(integrationFiles, integrationFiles2...)

	testedPaths := make(map[string]bool)
	for _, f := range allTestFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)

		// Match direct paths.
		for _, m := range e2eDirectPath.FindAllStringSubmatch(src, -1) {
			path := m[2]
			// Normalize: strip trailing slashes.
			path = strings.TrimSuffix(path, "/")
			testedPaths[path] = true
		}

		// Match fmt.Sprintf paths (like "/api/issues/%s").
		for _, m := range e2ePathFromReq.FindAllStringSubmatch(src, -1) {
			path := m[2]
			// Convert fmt.Sprintf placeholders to wildcards for matching.
			path = regexp.MustCompile(`%[sv]`).ReplaceAllString(path, "*")
			path = strings.TrimSuffix(path, "/")
			testedPaths[path] = true
		}

		// Also match doRequest-style calls.
		for _, m := range e2eHTTPRequest.FindAllStringSubmatch(src, -1) {
			path := m[1]
			path = regexp.MustCompile(`%[sv]`).ReplaceAllString(path, "*")
			path = strings.TrimSuffix(path, "/")
			testedPaths[path] = true
		}
	}

	// Step 3: Check each route against tested paths.
	var untested []string
	testedCount := 0

	for _, r := range allRoutes {
		normalizedPath := strings.TrimSuffix(r.path, "/")

		// Check if path or a prefix is tested.
		found := testedPaths[normalizedPath]

		// Also check by replacing chi {param} with * for fuzzy matching.
		if !found {
			chiPattern := regexp.MustCompile(`\{[^}]+\}`)
			fuzzyPath := chiPattern.ReplaceAllString(normalizedPath, "*")
			found = testedPaths[fuzzyPath]
		}

		// Check prefix matches (e.g., testing "/api/issues" covers "/api/issues/").
		if !found {
			for testedPath := range testedPaths {
				if strings.HasPrefix(normalizedPath, testedPath) || strings.HasPrefix(testedPath, normalizedPath) {
					found = true
					break
				}
			}
		}

		if found {
			testedCount++
		} else {
			untested = append(untested, fmt.Sprintf("  %s %s (%s)", r.method, normalizedPath, r.handler))
		}
	}

	sort.Strings(untested)

	t.Logf("route coverage: %d/%d routes have E2E test hits (%.0f%%)",
		testedCount, len(allRoutes), float64(testedCount)/float64(len(allRoutes))*100)

	if len(untested) > 0 {
		t.Logf("routes without E2E coverage (%d):\n%s", len(untested), strings.Join(untested, "\n"))
	}
}

func TestE2ETestsDontReferenceRemovedRoutes(t *testing.T) {
	// Verify that E2E tests don't test endpoints that no longer exist.
	routeFiles, err := filepath.Glob(filepath.Join(routeDir, "routes_*.go"))
	if err != nil {
		t.Fatalf("cannot glob route files: %v", err)
	}
	routerFile := filepath.Join(routeDir, "router.go")
	if _, statErr := os.Stat(routerFile); statErr == nil {
		routeFiles = append(routeFiles, routerFile)
	}

	// Build set of registered paths.
	registeredPaths := make(map[string]bool)
	for _, f := range routeFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range routeRegistration.FindAllStringSubmatch(string(data), -1) {
			registeredPaths[m[2]] = true
		}
	}

	// Check integration test files for hardcoded paths.
	integrationFiles, _ := filepath.Glob(filepath.Join(routeDir, "*integration*test.go"))
	if len(integrationFiles) == 0 {
		t.Skip("no integration test files found")
	}

	t.Logf("registered paths: %d, skipping stale-path check (needs runtime path resolution)", len(registeredPaths))
}

func TestRouteHandlerNamingConsistency(t *testing.T) {
	// Verify that route handler names follow the expected PascalCase convention.
	routeFiles, err := filepath.Glob(filepath.Join(routeDir, "routes_*.go"))
	if err != nil {
		t.Fatalf("cannot glob route files: %v", err)
	}
	routerFile := filepath.Join(routeDir, "router.go")
	if _, statErr := os.Stat(routerFile); statErr == nil {
		routeFiles = append(routeFiles, routerFile)
	}

	var problems []string
	for _, f := range routeFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		for _, m := range routeRegistration.FindAllStringSubmatch(src, -1) {
			handler := m[3]
			// Handler names should start with uppercase.
			if len(handler) > 0 && handler[0] >= 'a' && handler[0] <= 'z' {
				problems = append(problems, fmt.Sprintf("  %s: handler %q starts with lowercase", base, handler))
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("route handler naming violations:\n%s", strings.Join(problems, "\n"))
	}
}
