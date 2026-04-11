// Package regression: middleware chain consistency tests.
// Validates that routes have appropriate middleware applied (Auth, workspace membership, etc.)
// without requiring a running server -- pure-filesystem analysis of route registration code.
package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// middlewareUse matches: r.Use(middleware.SomeMiddleware(...))
var middlewareUse = regexp.MustCompile(`r\.Use\(middleware\.(\w+)`)

// middlewareWith matches: r.With(middleware.SomeMiddleware(...)).Get/Post/etc
var middlewareWith = regexp.MustCompile(`r\.With\(middleware\.(\w+)`)

func TestRouteFilesHaveMiddleware(t *testing.T) {
	routeFiles, err := filepath.Glob(filepath.Join(routeDir, "routes_*.go"))
	if err != nil {
		t.Fatalf("cannot glob route files: %v", err)
	}
	routerFile := filepath.Join(routeDir, "router.go")
	if _, statErr := os.Stat(routerFile); statErr == nil {
		routeFiles = append(routeFiles, routerFile)
	}

	for _, f := range routeFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("cannot read %s: %v", f, err)
			continue
		}
		src := string(content)
		base := filepath.Base(f)

		used := make(map[string]bool)
		for _, m := range middlewareUse.FindAllStringSubmatch(src, -1) {
			used[m[1]] = true
		}
		for _, m := range middlewareWith.FindAllStringSubmatch(src, -1) {
			used[m[1]] = true
		}

		if len(used) > 0 {
			var names []string
			for n := range used {
				names = append(names, n)
			}
			sort.Strings(names)
			t.Logf("%s uses middleware: %s", base, strings.Join(names, ", "))
		}
	}
}

func TestWorkspaceRoutesHaveMembershipMiddleware(t *testing.T) {
	routeFiles, err := filepath.Glob(filepath.Join(routeDir, "routes_*.go"))
	if err != nil {
		t.Fatalf("cannot glob route files: %v", err)
	}
	routerFile := filepath.Join(routeDir, "router.go")
	if _, statErr := os.Stat(routerFile); statErr == nil {
		routeFiles = append(routeFiles, routerFile)
	}

	allContent := ""
	for _, f := range routeFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		allContent += string(data) + "\n"
	}

	hasHeaderMembership := strings.Contains(allContent, "RequireWorkspaceMember(")
	hasURLMembership := strings.Contains(allContent, "RequireWorkspaceMemberFromURL(")

	if !hasHeaderMembership && !hasURLMembership {
		t.Error("no workspace membership middleware found in any route file")
	}

	routerContent, err := os.ReadFile(routerFile)
	if err != nil {
		t.Fatalf("cannot read router.go: %v", err)
	}
	routerSrc := string(routerContent)

	if !strings.Contains(routerSrc, "RequireWorkspaceMember(queries)") {
		t.Error("router.go: no RequireWorkspaceMember(queries) group found")
	}
}

func TestRepoRoutesHaveWorkspaceMiddleware(t *testing.T) {
	otherFile := filepath.Join(routeDir, "routes_other.go")
	content, err := os.ReadFile(otherFile)
	if err != nil {
		t.Fatalf("cannot read routes_other.go: %v", err)
	}
	src := string(content)

	repoRouteStart := strings.Index(src, `r.Route("/repos"`)
	if repoRouteStart == -1 {
		t.Log("no /repos route found in routes_other.go, skipping")
		return
	}

	idRouteStart := strings.Index(src, `r.Route("/{id}"`)
	if idRouteStart == -1 {
		return
	}

	repoSection := src[idRouteStart : repoRouteStart+len(`r.Route("/repos"`)+200]
	if len(repoSection) > len(src)-idRouteStart {
		repoSection = src[idRouteStart:]
	}

	hasMembershipMiddleware := strings.Contains(repoSection, "RequireWorkspaceMember") ||
		strings.Contains(repoSection, "RequireWorkspaceRole")

	if !hasMembershipMiddleware {
		t.Errorf("routes_other.go: /repos routes under /{id} have no workspace membership middleware — " +
			"any authenticated user can access repos in any workspace. " +
			"Repos should be inside a RequireWorkspaceMemberFromURL or RequireWorkspaceRoleFromURL group")
	}
}

func TestDaemonRoutesHaveDaemonAuth(t *testing.T) {
	daemonFile := filepath.Join(routeDir, "routes_daemon.go")
	content, err := os.ReadFile(daemonFile)
	if err != nil {
		t.Fatalf("cannot read routes_daemon.go: %v", err)
	}
	src := string(content)

	if !strings.Contains(src, "DaemonAuth") {
		t.Error("routes_daemon.go: no DaemonAuth middleware found")
	}
}

func TestAuthRoutesPublicVerify(t *testing.T) {
	authFile := filepath.Join(routeDir, "routes_auth.go")
	content, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatalf("cannot read routes_auth.go: %v", err)
	}
	src := string(content)

	if strings.Contains(src, "ws-ticket") {
		hasAuthForWsTicket := strings.Contains(src, "Auth(queries)")
		if !hasAuthForWsTicket {
			t.Error("routes_auth.go: ws-ticket route may be missing Auth middleware")
		}
	}

	publicEndpoints := []string{"send-code", "verify-code", "logout"}
	for _, ep := range publicEndpoints {
		if !strings.Contains(src, ep) {
			t.Errorf("routes_auth.go: expected public endpoint %q not found", ep)
		}
	}
}
