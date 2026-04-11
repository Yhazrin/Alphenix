// Package regression: route-handler registration consistency tests.
// Pure-filesystem structural checks — no database connection required.
package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const routeDir = "../../cmd/server"
const handlerDir = "../../internal/handler"

// hCall matches h.SomeMethod( — handler method calls in route files.
var hCall = regexp.MustCompile(`\bh\.([A-Z]\w+)\b`)

// handlerMethodDecl matches: func (h *Handler) SomeMethod(
var handlerMethodDecl = regexp.MustCompile(`func\s+\(h\s+\*Handler\)\s+([A-Z]\w+)\s*\(`)

// routeFuncDecl matches: func registerXxxRoutes(
var routeFuncDecl = regexp.MustCompile(`func\s+register\w+Routes\s*\(`)

// httpVerb matches: r.Get("/path", h.Method) etc.
var httpVerb = regexp.MustCompile(`\br\.(Get|Post|Put|Patch|Delete|Head|Options)\s*\(\s*"([^"]+)"`)

// routeRouteCall matches: r.Route("/prefix", func(r chi.Router) { nesting
var routeRouteCall = regexp.MustCompile(`\br\.Route\s*\(\s*"([^"]+)"`)

// knownInternal are Handler methods called only from within the handler package itself (not from routes).
var knownInternal = map[string]bool{
	"ensureUserWorkspace":                true,
	"issueJWT":                           true,
	"commentMentionsOthersButNotAssignee": true,
	"isReplyToMemberThread":              true,
	"isAgentSelfTrigger":                 true,
	"enqueueMentionedAgentTasks":         true,
	"executeDecompose":                   true,
	"decomposeParallel":                  true,
	"decomposeSerial":                    true,
	"notifySubscribers":                  true,
	"executeFork":                        true,
	"sendEmailAsync":                     true,
	"ListMembers":                        true,
}

func TestRouteHandlerReferencesExist(t *testing.T) {
	// Collect all handler methods defined.
	handlerMethods := make(map[string]string) // method name -> file
	err := filepath.Walk(handlerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(handlerDir, path)
		for _, m := range handlerMethodDecl.FindAllStringSubmatch(string(content), -1) {
			handlerMethods[m[1]] = relPath
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk handler directory: %v", err)
	}

	// Collect all h.Method calls from route files.
	calledMethods := make(map[string][]string) // method -> []"file:line approx"
	routeFiles, globErr := filepath.Glob(filepath.Join(routeDir, "routes_*.go"))
	if globErr != nil {
		t.Fatalf("cannot glob route files: %v", globErr)
	}
	routerFile := filepath.Join(routeDir, "router.go")
	if _, statErr := os.Stat(routerFile); statErr == nil {
		routeFiles = append(routeFiles, routerFile)
	}

	for _, f := range routeFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		relPath, _ := filepath.Rel(routeDir, f)
		src := string(content)
		for _, m := range hCall.FindAllStringSubmatch(src, -1) {
			method := m[1]
			calledMethods[method] = append(calledMethods[method], relPath)
		}
	}

	var problems []string
	for method, callers := range calledMethods {
		if _, exists := handlerMethods[method]; !exists {
			if knownInternal[method] {
				continue
			}
			sort.Strings(callers)
			problems = append(problems, "  h."+method+" referenced in "+strings.Join(callers, ", ")+" but no handler method found")
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("route files reference non-existent handler methods:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestNoDuplicateRouteRegistrations(t *testing.T) {
	routeFiles, err := filepath.Glob(filepath.Join(routeDir, "routes_*.go"))
	if err != nil {
		t.Fatalf("cannot glob route files: %v", err)
	}
	routerFile := filepath.Join(routeDir, "router.go")
	if _, statErr := os.Stat(routerFile); statErr == nil {
		routeFiles = append(routeFiles, routerFile)
	}

	type routeKey struct {
		method string
		path   string
		file   string
	}
	routes := make(map[string][]string) // "METHOD path" -> []files

	for _, f := range routeFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		relPath, _ := filepath.Rel(routeDir, f)
		src := string(content)

		// Track r.Route nesting to build full path prefix.
		var pathPrefix []string
		lines := strings.Split(src, "\n")
		braceDepth := 0

		for _, line := range lines {
			// Track Route nesting.
			if rm := routeRouteCall.FindStringSubmatch(line); rm != nil {
				pathPrefix = append(pathPrefix, rm[1])
			}
			braceDepth += strings.Count(line, "{")
			braceDepth -= strings.Count(line, "}")

			// When brace depth drops, pop path prefix.
			for braceDepth < len(pathPrefix) && len(pathPrefix) > 0 {
				pathPrefix = pathPrefix[:len(pathPrefix)-1]
			}

			// Match HTTP verb registrations.
			if vm := httpVerb.FindStringSubmatch(line); vm != nil {
				method := strings.ToUpper(vm[1])
				path := vm[2]
				// Build full path from prefix.
				fullPath := strings.Join(pathPrefix, "") + path
				key := method + " " + fullPath
				routes[key] = append(routes[key], relPath)
			}
		}
	}

	var problems []string
	for key, files := range routes {
		if len(files) > 1 {
			sort.Strings(files)
			problems = append(problems, "  "+key+" registered in: "+strings.Join(files, ", "))
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("duplicate route registrations detected:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestHandlerMethodsReachableFromRoutes(t *testing.T) {
	// Find exported handler methods that are never called from any route file.
	handlerMethods := make(map[string]string)
	err := filepath.Walk(handlerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(handlerDir, path)
		for _, m := range handlerMethodDecl.FindAllStringSubmatch(string(content), -1) {
			handlerMethods[m[1]] = relPath
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk handler directory: %v", err)
	}

	// Collect all h.Method references from route files + handler files themselves.
	called := make(map[string]bool)
	sources := []string{routeDir, handlerDir}
	for _, dir := range sources {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, _ := os.ReadFile(path)
			if content == nil {
				return nil
			}
			for _, m := range hCall.FindAllStringSubmatch(string(content), -1) {
				called[m[1]] = true
			}
			return nil
		})
	}

	var unreachable []string
	for method, file := range handlerMethods {
		if !called[method] && !knownInternal[method] {
			unreachable = append(unreachable, "  "+method+" (in "+file+") — never referenced in route or handler files")
		}
	}
	sort.Strings(unreachable)

	if len(unreachable) > 0 {
		t.Logf("handler methods potentially unreachable from routes:\n%s",
			strings.Join(unreachable, "\n"))
	}
}
