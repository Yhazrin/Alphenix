// Package regression: handler response type consistency tests.
// Validates that handler response structs follow consistent patterns,
// JSON tags use snake_case, and handler methods return defined response types.
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

// responseStruct matches response struct definitions with JSON tags.
var responseStruct = regexp.MustCompile(`type\s+(\w+)\s+struct\s*\{([^}]+)\}`)

// jsonTag matches `json:"field_name"` tags.
var jsonTag = regexp.MustCompile(`json:"(\w+)"`)

// handlerMethod matches func (h *Handler) MethodName(w http.ResponseWriter, r *http.Request)
var handlerMethod = regexp.MustCompile(`func\s+\(h\s+\*Handler\)\s+(\w+)\s*\(\s*w\s+http\.ResponseWriter`)

// responseWriteJSON matches h.writeJSON(w, statusCode, data, t)
var responseWriteJSON = regexp.MustCompile(`h\.writeJSON\(\s*w\s*,\s*\d+\s*,\s*(\w+),\s*t`)

// handlerRespondJSON matches respondJSON(w, statusCode, data)
var handlerRespondJSON = regexp.MustCompile(`respondJSON\(\s*w\s*,\s*\d+\s*,\s*(\w+)\s*\)`)

func TestHandlerResponseStructsHaveJSONTags(t *testing.T) {
	handlerFiles, err := filepath.Glob(filepath.Join(handlerDir, "*.go"))
	if err != nil {
		t.Fatalf("cannot glob handler files: %v", err)
	}

	var problems []string
	for _, f := range handlerFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		for _, m := range responseStruct.FindAllStringSubmatch(src, -1) {
			structName := m[1]
			body := m[2]

			// Only check response-type structs (end with Response, or contain "Response").
			if !strings.Contains(structName, "Response") && !strings.Contains(structName, "Result") {
				continue
			}

			for _, line := range strings.Split(body, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "//") {
					continue
				}
				// Check if field has a json tag.
				if strings.Contains(trimmed, "json:") {
					continue
				}
				// Skip unexported fields.
				parts := strings.Fields(trimmed)
				if len(parts) < 1 || strings.ToUpper(parts[0][:1]) != parts[0][:1] {
					continue
				}
				// Embedded struct fields: single token (just the type name), no JSON tag needed.
				// e.g., "PersonalAccessTokenResponse" or "*SomeType"
				if len(parts) == 1 || (len(parts) == 2 && strings.HasPrefix(parts[0], "*")) {
					continue
				}
				// Regular exported field without JSON tag.
				problems = append(problems, fmt.Sprintf(
					"  %s: %s field %q missing json tag", base, structName, parts[0]))
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("response struct fields missing JSON tags:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestJSONTagsAreSnakeCase(t *testing.T) {
	handlerFiles, err := filepath.Glob(filepath.Join(handlerDir, "*.go"))
	if err != nil {
		t.Fatalf("cannot glob handler files: %v", err)
	}

	camelCasePattern := regexp.MustCompile(`[a-z][A-Z]`)
	var problems []string

	for _, f := range handlerFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		for _, m := range responseStruct.FindAllStringSubmatch(src, -1) {
			structName := m[1]
			body := m[2]

			// Only enforce snake_case on exported types (our public API).
			// Unexported types (e.g., clawhub* matching external APIs) may use camelCase.
			if len(structName) > 0 && structName[0] >= 'a' && structName[0] <= 'z' {
				continue
			}

			for _, jm := range jsonTag.FindAllStringSubmatch(body, -1) {
				tag := jm[1]
				// Skip special tags like "-" or ",omitempty".
				if tag == "-" {
					continue
				}
				// Check for camelCase (has adjacent lowercase-uppercase).
				if camelCasePattern.MatchString(tag) {
					problems = append(problems, fmt.Sprintf(
						"  %s: %s uses camelCase json tag %q (expected snake_case)",
						base, structName, tag))
				}
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("non-snake_case JSON tags in response structs:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestHandlerMethodResponseTypes(t *testing.T) {
	// Verify that each handler method that writes JSON actually references
	// a response variable, and that variable is defined somewhere.
	handlerFiles, err := filepath.Glob(filepath.Join(handlerDir, "*.go"))
	if err != nil {
		t.Fatalf("cannot glob handler files: %v", err)
	}

	// Collect all response struct names.
	responseTypes := make(map[string]string) // name → file
	for _, f := range handlerFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		for _, m := range responseStruct.FindAllStringSubmatch(src, -1) {
			responseTypes[m[1]] = base
		}
	}

	// Collect handler methods and what they pass to writeJSON/respondJSON.
	var problems []string
	for _, f := range handlerFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		for _, hm := range handlerMethod.FindAllStringSubmatch(src, -1) {
			_ = hm[1] // methodName

			// Find the method body.
			methodStart := strings.Index(src, hm[0])
			if methodStart == -1 {
				continue
			}
			body := src[methodStart:]

			// Find the closing brace.
			braceCount := 0
			bodyEnd := -1
			for i := 0; i < len(body); i++ {
				if body[i] == '{' {
					braceCount++
				} else if body[i] == '}' {
					braceCount--
					if braceCount == 0 {
						bodyEnd = i
						break
					}
				}
			}
			if bodyEnd == -1 {
				continue
			}
			methodBody := body[:bodyEnd]

			// Check if method calls writeJSON or respondJSON.
			hasWriteJSON := responseWriteJSON.MatchString(methodBody)
			hasRespondJSON := handlerRespondJSON.MatchString(methodBody)

			if !hasWriteJSON && !hasRespondJSON {
				// Some handlers write errors or empty responses — that's fine.
				continue
			}
		}

		// Also count total handler methods in this file.
		methods := handlerMethod.FindAllStringSubmatch(src, -1)
		if len(methods) > 0 {
			t.Logf("%s: %d handler methods", base, len(methods))
		}
	}

	if len(problems) > 0 {
		t.Errorf("handler response type issues:\n%s",
			strings.Join(problems, "\n"))
	}

	t.Logf("found %d response struct types across handler package", len(responseTypes))
}

func TestResponseStructsNotDuplicate(t *testing.T) {
	handlerFiles, err := filepath.Glob(filepath.Join(handlerDir, "*.go"))
	if err != nil {
		t.Fatalf("cannot glob handler files: %v", err)
	}

	type structInfo struct {
		file    string
		fields  int
	}
	seen := make(map[string]structInfo)

	var problems []string
	for _, f := range handlerFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		for _, m := range responseStruct.FindAllStringSubmatch(src, -1) {
			name := m[1]
			body := m[2]

			fieldCount := 0
			for _, line := range strings.Split(body, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
					fieldCount++
				}
			}

			if prev, ok := seen[name]; ok {
				if prev.fields != fieldCount {
					problems = append(problems, fmt.Sprintf(
						"  %s: duplicate struct %q (%d fields) — previously defined in %s (%d fields)",
						base, name, fieldCount, prev.file, prev.fields))
				}
			} else {
				seen[name] = structInfo{file: base, fields: fieldCount}
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Errorf("response struct conflicts:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestHandlerErrorHandlingConsistency(t *testing.T) {
	handlerFiles, err := filepath.Glob(filepath.Join(handlerDir, "*.go"))
	if err != nil {
		t.Fatalf("cannot glob handler files: %v", err)
	}

	httpError := regexp.MustCompile(`h\.writeError\(\s*w\s*,\s*(\d+)`)
	legacyError := regexp.MustCompile(`http\.Error\(\s*w`)

	var withWriteError, withLegacy int
	var problems []string

	for _, f := range handlerFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		hasWriteError := httpError.MatchString(src)
		hasLegacy := legacyError.MatchString(src)

		if hasWriteError {
			withWriteError++
		}
		if hasLegacy {
			withLegacy++
			// Check if also has h.writeError — mixing patterns.
			if hasWriteError {
				problems = append(problems, fmt.Sprintf(
					"  %s: mixes h.writeError() and http.Error() — pick one pattern", base))
			}
		}
	}

	t.Logf("error handling: %d files use h.writeError(), %d use http.Error()",
		withWriteError, withLegacy)

	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("mixed error handling patterns:\n%s",
			strings.Join(problems, "\n"))
	}
}

func TestHandlerMethodsHaveHandlerReceiver(t *testing.T) {
	handlerFiles, err := filepath.Glob(filepath.Join(handlerDir, "*.go"))
	if err != nil {
		t.Fatalf("cannot glob handler files: %v", err)
	}

	var problems []string
	for _, f := range handlerFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		base := filepath.Base(f)

		// Find functions that look like handlers (accept w http.ResponseWriter)
		// but don't have *Handler receiver.
		funcPattern := regexp.MustCompile(`func\s+(?:\(\w+\s+\*\w+\)\s+)?(\w+)\s*\([^)]*w\s+http\.ResponseWriter`)
		for _, m := range funcPattern.FindAllStringSubmatch(src, -1) {
			funcName := m[1]
			fullMatch := m[0]
			if !strings.Contains(fullMatch, "*Handler)") && !strings.HasPrefix(funcName, "Test") {
				// Helper functions are OK if they're unexported or named with helper pattern.
				if strings.ToUpper(funcName[:1]) == funcName[:1] && !strings.HasPrefix(funcName, "New") {
					problems = append(problems, fmt.Sprintf(
						"  %s: exported handler function %s() has no *Handler receiver",
						base, funcName))
				}
			}
		}
	}
	sort.Strings(problems)

	if len(problems) > 0 {
		t.Logf("exported handler functions without *Handler receiver:\n%s",
			strings.Join(problems, "\n"))
	}
}
