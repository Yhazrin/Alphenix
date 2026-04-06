package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Worktree Isolation ---

// WorktreeProvider creates isolated working directories for parallel forks.
// Each worktree gets a copy of the project files so forks don't interfere.
type WorktreeProvider struct {
	baseDir string // parent directory for all worktrees
}

// NewWorktreeProvider creates a provider that puts worktrees under baseDir.
func NewWorktreeProvider(baseDir string) *WorktreeProvider {
	return &WorktreeProvider{baseDir: baseDir}
}

// Create allocates a new isolated worktree, copies project files from sourceDir,
// and returns the path. Caller must call Cleanup when done.
func (wp *WorktreeProvider) Create(sourceDir string) (worktreePath string, cleanup func(), err error) {
	tmpDir, err := os.MkdirTemp(wp.baseDir, "alphenix-fork-*")
	if err != nil {
		return "", nil, err
	}

	if err := copyDir(sourceDir, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, err
	}

	cleanup = func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup, nil
}

// copyDir recursively copies src to dst (shallow — for test purposes).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Skip .git and .alphenix directories.
			if entry.Name() == ".git" || entry.Name() == ".alphenix" {
				continue
			}
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Tests ---

func TestWorktree_CreateAndCleanup(t *testing.T) {
	// Create a fake project directory with some files.
	srcDir := t.TempDir()
	writeFile(t, srcDir, "main.go", "package main\nfunc main() {}\n")
	writeFile(t, srcDir, "README.md", "# Project\n")
	os.MkdirAll(filepath.Join(srcDir, "pkg", "util"), 0o755)
	writeFile(t, filepath.Join(srcDir, "pkg", "util"), "util.go", "package util\n")

	wp := NewWorktreeProvider(t.TempDir())

	worktree, cleanup, err := wp.Create(srcDir)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify files were copied.
	if _, err := os.Stat(filepath.Join(worktree, "main.go")); err != nil {
		t.Errorf("main.go not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "pkg", "util", "util.go")); err != nil {
		t.Errorf("pkg/util/util.go not copied: %v", err)
	}

	// Cleanup removes the worktree.
	cleanup()
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Errorf("worktree should be removed after cleanup")
	}
}

func TestWorktree_Isolation(t *testing.T) {
	// Two forks writing to the same relative path should not conflict.
	srcDir := t.TempDir()
	writeFile(t, srcDir, "shared.txt", "original\n")

	wp := NewWorktreeProvider(t.TempDir())

	wt1, cleanup1, err := wp.Create(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup1()

	wt2, cleanup2, err := wp.Create(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup2()

	// Fork 1 modifies shared.txt.
	if err := os.WriteFile(filepath.Join(wt1, "shared.txt"), []byte("fork 1 edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Fork 2 modifies shared.txt differently.
	if err := os.WriteFile(filepath.Join(wt2, "shared.txt"), []byte("fork 2 edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify isolation — each worktree has its own version.
	data1, _ := os.ReadFile(filepath.Join(wt1, "shared.txt"))
	data2, _ := os.ReadFile(filepath.Join(wt2, "shared.txt"))

	if string(data1) != "fork 1 edit\n" {
		t.Errorf("worktree 1 expected 'fork 1 edit', got %q", data1)
	}
	if string(data2) != "fork 2 edit\n" {
		t.Errorf("worktree 2 expected 'fork 2 edit', got %q", data2)
	}
}

func TestWorktree_GitDirExcluded(t *testing.T) {
	srcDir := t.TempDir()
	writeFile(t, srcDir, "main.go", "package main\n")
	os.MkdirAll(filepath.Join(srcDir, ".git"), 0o755)
	writeFile(t, srcDir, filepath.Join(".git", "config"), "[core]\n")

	wp := NewWorktreeProvider(t.TempDir())
	worktree, cleanup, err := wp.Create(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// .git should NOT be copied.
	if _, err := os.Stat(filepath.Join(worktree, ".git")); !os.IsNotExist(err) {
		t.Error(".git directory should be excluded from worktree copy")
	}

	// But main.go should be there.
	if _, err := os.Stat(filepath.Join(worktree, "main.go")); err != nil {
		t.Errorf("main.go should be copied: %v", err)
	}
}

func TestWorktree_ParallelCreation(t *testing.T) {
	srcDir := t.TempDir()
	writeFile(t, srcDir, "code.go", "package code\n")

	wp := NewWorktreeProvider(t.TempDir())

	// Create 5 worktrees in parallel — simulates 5 concurrent forks.
	const N = 5
	results := make(chan string, N)
	errs := make(chan error, N)

	for i := 0; i < N; i++ {
		go func() {
			wt, cleanup, err := wp.Create(srcDir)
			if err != nil {
				errs <- err
				return
			}
			defer cleanup()

			// Verify the file exists in this worktree.
			if _, statErr := os.Stat(filepath.Join(wt, "code.go")); statErr != nil {
				errs <- statErr
				return
			}
			results <- wt
		}()
	}

	for i := 0; i < N; i++ {
		select {
		case wt := <-results:
			if wt == "" {
				t.Error("got empty worktree path")
			}
		case err := <-errs:
			t.Errorf("parallel worktree creation failed: %v", err)
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}
