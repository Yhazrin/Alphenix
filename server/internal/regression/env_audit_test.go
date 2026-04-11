// Package regression: environment variable usage audit tests.
// Cross-references os.Getenv/os.LookupEnv calls in Go code.
package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var envGetCall = regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\(\s*"([A-Z_][A-Z0-9_]*)"`)
var envSetCall = regexp.MustCompile(`os\.Setenv\(\s*"([A-Z_][A-Z0-9_]*)"`)

func TestEnvVarUsageAudit(t *testing.T) {
	rootDir := "../.."
	type envInfo struct {
		name   string
		files  []string
		isRead bool
		isWrite bool
	}
	envs := make(map[string]*envInfo)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := string(content)
		relPath, _ := filepath.Rel(rootDir, path)

		for _, m := range envGetCall.FindAllStringSubmatch(src, -1) {
			name := m[1]
			if envs[name] == nil {
				envs[name] = &envInfo{name: name}
			}
			envs[name].files = append(envs[name].files, relPath)
			envs[name].isRead = true
		}

		for _, m := range envSetCall.FindAllStringSubmatch(src, -1) {
			name := m[1]
			if envs[name] == nil {
				envs[name] = &envInfo{name: name}
			}
			envs[name].files = append(envs[name].files, relPath)
			envs[name].isWrite = true
		}

		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk root directory: %v", err)
	}

	var names []string
	for name := range envs {
		names = append(names, name)
	}
	sort.Strings(names)

	t.Logf("found %d unique env vars in Go code:", len(names))
	for _, name := range names {
		ev := envs[name]
		usageType := "read"
		if ev.isWrite && !ev.isRead {
			usageType = "write-only"
		} else if ev.isWrite {
			usageType = "read+write"
		}

		dedup := make(map[string]bool)
		var fileList []string
		for _, f := range ev.files {
			if !dedup[f] {
				dedup[f] = true
				fileList = append(fileList, f)
			}
		}
		sort.Strings(fileList)
		t.Logf("  %-40s [%s] in %s", name, usageType, strings.Join(fileList, ", "))
	}
}

func TestNoHardcodedSecretPatterns(t *testing.T) {
	rootDir := "../.."

	secretPatterns := []*regexp.Regexp{
		regexp.MustCompile(`"(?:sk|pk|ak|rk)_[a-zA-Z0-9]{20,}"`),
		regexp.MustCompile(`"ghp_[a-zA-Z0-9]{36}"`),
		regexp.MustCompile(`"gho_[a-zA-Z0-9]{36}"`),
		regexp.MustCompile(`"xox[bpsa]-[a-zA-Z0-9-]+"`),
		regexp.MustCompile(`"AKIA[0-9A-Z]{16}"`),
	}

	var findings []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
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
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := string(content)
		relPath, _ := filepath.Rel(rootDir, path)

		for _, pat := range secretPatterns {
			for _, m := range pat.FindAllString(src, -1) {
				findings = append(findings, relPath+": potential hardcoded secret: "+m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk root directory: %v", err)
	}

	if len(findings) > 0 {
		sort.Strings(findings)
		t.Errorf("potential hardcoded secrets found:\n  %s",
			strings.Join(findings, "\n  "))
	}
}

func TestDockerComposeEnvVarsConsistent(t *testing.T) {
	composeFiles, _ := filepath.Glob(filepath.Join("../..", "docker-compose*"))
	composeFiles2, _ := filepath.Glob(filepath.Join("../..", "compose*"))
	composeFiles = append(composeFiles, composeFiles2...)

	if len(composeFiles) == 0 {
		t.Skip("no docker-compose files found, skipping")
		return
	}

	dockerEnvPattern := regexp.MustCompile(`^\s*([A-Z_][A-Z0-9_]*):`)
	dockerEnvVars := make(map[string]bool)
	for _, f := range composeFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			if m := dockerEnvPattern.FindStringSubmatch(line); m != nil {
				dockerEnvVars[m[1]] = true
			}
		}
	}

	if len(dockerEnvVars) == 0 {
		t.Log("no env vars found in docker-compose files")
		return
	}

	t.Logf("found %d env vars in docker-compose files", len(dockerEnvVars))

	rootDir := "../.."
	goEnvVars := make(map[string]bool)
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		content, _ := os.ReadFile(path)
		if content == nil {
			return nil
		}
		for _, m := range envGetCall.FindAllStringSubmatch(string(content), -1) {
			goEnvVars[m[1]] = true
		}
		return nil
	})

	var dockerOnly []string
	skip := map[string]bool{"POSTGRES_PASSWORD": true, "POSTGRES_USER": true, "POSTGRES_DB": true}
	for v := range dockerEnvVars {
		if !goEnvVars[v] && !skip[v] {
			dockerOnly = append(dockerOnly, v)
		}
	}

	if len(dockerOnly) > 0 {
		sort.Strings(dockerOnly)
		t.Logf("env vars in docker-compose but not used in Go code: %v", dockerOnly)
	}
}
