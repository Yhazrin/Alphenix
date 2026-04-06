// Package execenv manages isolated per-task execution environments for the daemon.
// Each task gets its own directory with injected context files. Repositories are
// checked out on demand by the agent via `alphenix repo checkout`.
package execenv

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// RepoContextForEnv describes a workspace repo available for checkout.
type RepoContextForEnv struct {
	URL         string // remote URL
	Description string // human-readable description
}

// PrepareParams holds all inputs needed to set up an execution environment.
type PrepareParams struct {
	WorkspacesRoot    string             // base path for all envs (e.g., ~/alphenix_workspaces)
	WorkspaceID       string             // workspace UUID — tasks are grouped under this
	TaskID            string             // task UUID — used for directory name
	AgentName         string             // for git branch naming only
	Provider          string             // agent provider ("claude", "codex") — determines skill injection paths
	Task              TaskContextForEnv  // context data for writing files
	RepoCLAUDEProvider RepoCLAUDEProvider // optional: reads repo-level CLAUDE.md from bare cache
}

// TaskContextForEnv is the subset of task context used for writing context files.
type TaskContextForEnv struct {
	IssueID           string
	TriggerCommentID  string // comment that triggered this task (empty for on_assign)
	AgentName         string
	AgentInstructions string // agent identity/persona instructions, injected into CLAUDE.md
	AgentSkills       []SkillContextForEnv
	Repos             []RepoContextForEnv // workspace repos available for checkout
}

// SkillContextForEnv represents a skill to be written into the execution environment.
type SkillContextForEnv struct {
	Name    string
	Content string
	Files   []SkillFileContextForEnv
}

// SkillFileContextForEnv represents a supporting file within a skill.
type SkillFileContextForEnv struct {
	Path    string
	Content string
}

// Environment represents a prepared, isolated execution environment.
type Environment struct {
	// RootDir is the top-level env directory ({workspacesRoot}/{task_id_short}/).
	RootDir string
	// WorkDir is the directory to pass as Cwd to the agent ({RootDir}/workdir/).
	WorkDir string
	// CodexHome is the path to the per-task CODEX_HOME directory (set only for codex provider).
	CodexHome string

	logger *slog.Logger // for cleanup logging
}

// RepoCLAUDEProvider is an optional interface for reading repo-level CLAUDE.md
// from a bare cache. When provided, Prepare copies discovered CLAUDE.md files
// into the workDir so Claude Code sees project-specific instructions immediately.
type RepoCLAUDEProvider interface {
	// LookupRepoFile returns the content of a file from a repo's bare cache,
	// or empty string if not found. The file is read from the default branch.
	LookupRepoFile(workspaceID, repoURL, filePath string) (string, error)
}

// Prepare creates an isolated execution environment for a task.
// The workdir starts empty (no repo checkouts). The agent checks out repos
// on demand via `alphenix repo checkout <url>`.
func Prepare(params PrepareParams, logger *slog.Logger) (*Environment, error) {
	if params.WorkspacesRoot == "" {
		return nil, fmt.Errorf("execenv: workspaces root is required")
	}
	if params.WorkspaceID == "" {
		return nil, fmt.Errorf("execenv: workspace ID is required")
	}
	if params.TaskID == "" {
		return nil, fmt.Errorf("execenv: task ID is required")
	}

	envRoot := filepath.Join(params.WorkspacesRoot, params.WorkspaceID, shortID(params.TaskID))

	// Remove existing env if present (defensive — task IDs are unique).
	if _, err := os.Stat(envRoot); err == nil {
		if err := os.RemoveAll(envRoot); err != nil {
			return nil, fmt.Errorf("execenv: remove existing env: %w", err)
		}
	}

	// Create directory tree.
	workDir := filepath.Join(envRoot, "workdir")
	for _, dir := range []string{workDir, filepath.Join(envRoot, "output"), filepath.Join(envRoot, "logs")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("execenv: create directory %s: %w", dir, err)
		}
	}

	env := &Environment{
		RootDir: envRoot,
		WorkDir: workDir,
		logger:  logger,
	}

	// Write context files into workdir (skills go to provider-native paths).
	if err := writeContextFiles(workDir, params.Provider, params.Task); err != nil {
		return nil, fmt.Errorf("execenv: write context files: %w", err)
	}

	// Copy repo-level CLAUDE.md files from bare cache into workDir so Claude Code
	// sees project-specific instructions immediately (before the agent checks out repos).
	if params.Provider == "claude" && params.RepoCLAUDEProvider != nil {
		copyRepoCLAUDEFiles(workDir, params.WorkspaceID, params.Task.Repos, params.RepoCLAUDEProvider, logger)
	}

	// For Codex, set up a per-task CODEX_HOME seeded from ~/.codex/ with skills.
	if params.Provider == "codex" {
		codexHome := filepath.Join(envRoot, "codex-home")
		if err := prepareCodexHome(codexHome, logger); err != nil {
			return nil, fmt.Errorf("execenv: prepare codex-home: %w", err)
		}
		if len(params.Task.AgentSkills) > 0 {
			if err := writeSkillFiles(filepath.Join(codexHome, "skills"), params.Task.AgentSkills); err != nil {
				return nil, fmt.Errorf("execenv: write codex skills: %w", err)
			}
		}
		env.CodexHome = codexHome
	}

	logger.Info("execenv: prepared env", "root", envRoot, "repos_available", len(params.Task.Repos))
	return env, nil
}

// Reuse wraps an existing workdir into an Environment and refreshes context files.
// Returns nil if the workdir does not exist (caller should fall back to Prepare).
func Reuse(workDir, provider string, task TaskContextForEnv, logger *slog.Logger) *Environment {
	if _, err := os.Stat(workDir); err != nil {
		return nil
	}

	env := &Environment{
		RootDir: filepath.Dir(workDir),
		WorkDir: workDir,
		logger:  logger,
	}

	// Refresh context files (issue_context.md, skills).
	if err := writeContextFiles(workDir, provider, task); err != nil {
		logger.Warn("execenv: refresh context files failed", "error", err)
	}

	logger.Info("execenv: reusing env", "workdir", workDir)
	return env
}

// copyRepoCLAUDEFiles reads CLAUDE.md from each repo's bare cache and writes it
// into the workDir under a .repo_claude/ directory. The runtime config (CLAUDE.md)
// includes instructions pointing Claude Code to these files.
func copyRepoCLAUDEFiles(workDir, workspaceID string, repos []RepoContextForEnv, provider RepoCLAUDEProvider, logger *slog.Logger) {
	if len(repos) == 0 {
		return
	}

	repoDir := filepath.Join(workDir, ".repo_claude")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		logger.Warn("execenv: create .repo_claude dir failed", "error", err)
		return
	}

	for _, repo := range repos {
		if repo.URL == "" {
			continue
		}
		content, err := provider.LookupRepoFile(workspaceID, repo.URL, "CLAUDE.md")
		if err != nil {
			logger.Debug("execenv: lookup repo CLAUDE.md failed", "url", repo.URL, "error", err)
			continue
		}
		if content == "" {
			continue
		}
		// Write to {workDir}/.repo_claude/{sanitized-repo-name}.md
		fileName := sanitizeRepoFileName(repo.URL) + ".md"
		dest := filepath.Join(repoDir, fileName)
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			logger.Warn("execenv: write repo CLAUDE.md failed", "url", repo.URL, "error", err)
			continue
		}
		logger.Info("execenv: copied repo CLAUDE.md", "url", repo.URL, "dest", dest)
	}
}

// sanitizeRepoFileName extracts a safe filename from a repo URL.
func sanitizeRepoFileName(url string) string {
	name := url
	// Strip protocol and trailing slashes.
	if i := strings.LastIndex(name, "://"); i >= 0 {
		name = name[i+3:]
	}
	name = strings.TrimRight(name, "/")
	name = strings.TrimSuffix(name, ".git")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	// Replace non-alphanumeric with hyphens.
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "repo"
	}
	return result
}

// Cleanup tears down the execution environment.
// If removeAll is true, the entire env root is deleted. Otherwise, workdir is
// removed but output/ and logs/ are preserved for debugging.
func (env *Environment) Cleanup(removeAll bool) error {
	if env == nil {
		return nil
	}

	if removeAll {
		if err := os.RemoveAll(env.RootDir); err != nil {
			env.logger.Warn("execenv: cleanup removeAll failed", "error", err)
			return err
		}
		return nil
	}

	// Partial cleanup: remove workdir, keep output/ and logs/.
	if err := os.RemoveAll(env.WorkDir); err != nil {
		env.logger.Warn("execenv: cleanup workdir failed", "error", err)
		return err
	}
	return nil
}
