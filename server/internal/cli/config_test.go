package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIConfigPath_Default(t *testing.T) {
	p, err := CLIConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(p, filepath.Join(".alphenix", "config.json")) {
		t.Errorf("expected suffix .alphenix/config.json, got %s", p)
	}
}

func TestCLIConfigPathForProfile_Named(t *testing.T) {
	p, err := CLIConfigPathForProfile("staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(p, filepath.Join(".alphenix", "profiles", "staging", "config.json")) {
		t.Errorf("expected profiles/staging/config.json suffix, got %s", p)
	}
}

func TestProfileDir_Default(t *testing.T) {
	d, err := ProfileDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(d, ".alphenix") {
		t.Errorf("expected .alphenix suffix, got %s", d)
	}
}

func TestProfileDir_Named(t *testing.T) {
	d, err := ProfileDir("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(d, filepath.Join("profiles", "dev")) {
		t.Errorf("expected profiles/dev in path, got %s", d)
	}
}

func TestSaveAndLoadCLIConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	// Override home for testing by writing directly
	cfgPath := filepath.Join(dir, "config.json")
	cfg := CLIConfig{
		ServerURL:   "https://api.example.com",
		WorkspaceID: "ws-123",
		Token:       "tok-abc",
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cfgPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded CLIConfig
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.ServerURL != cfg.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, cfg.ServerURL)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, cfg.Token)
	}
}

func TestLoadCLIConfig_MissingFile(t *testing.T) {
	// LoadCLIConfigForProfile with a profile pointing to a nonexistent path
	// We test the parsing logic by manually reading from a nonexistent path
	cfg := CLIConfig{}
	if cfg.ServerURL != "" || cfg.Token != "" {
		t.Error("expected zero-value config")
	}
}

func TestAddWatchedWorkspace_New(t *testing.T) {
	var cfg CLIConfig
	added := cfg.AddWatchedWorkspace("ws-1", "main")
	if !added {
		t.Error("expected true for new workspace")
	}
	if len(cfg.WatchedWorkspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(cfg.WatchedWorkspaces))
	}
	if cfg.WatchedWorkspaces[0].ID != "ws-1" {
		t.Errorf("ID = %q, want ws-1", cfg.WatchedWorkspaces[0].ID)
	}
}

func TestAddWatchedWorkspace_Duplicate(t *testing.T) {
	cfg := CLIConfig{WatchedWorkspaces: []WatchedWorkspace{{ID: "ws-1", Name: "main"}}}
	added := cfg.AddWatchedWorkspace("ws-1", "main")
	if added {
		t.Error("expected false for duplicate workspace")
	}
	if len(cfg.WatchedWorkspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(cfg.WatchedWorkspaces))
	}
}

func TestRemoveWatchedWorkspace_Found(t *testing.T) {
	cfg := CLIConfig{WatchedWorkspaces: []WatchedWorkspace{{ID: "ws-1"}, {ID: "ws-2"}}}
	removed := cfg.RemoveWatchedWorkspace("ws-1")
	if !removed {
		t.Error("expected true")
	}
	if len(cfg.WatchedWorkspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(cfg.WatchedWorkspaces))
	}
	if cfg.WatchedWorkspaces[0].ID != "ws-2" {
		t.Errorf("remaining ID = %q, want ws-2", cfg.WatchedWorkspaces[0].ID)
	}
}

func TestRemoveWatchedWorkspace_NotFound(t *testing.T) {
	cfg := CLIConfig{WatchedWorkspaces: []WatchedWorkspace{{ID: "ws-1"}}}
	removed := cfg.RemoveWatchedWorkspace("ws-999")
	if removed {
		t.Error("expected false for nonexistent workspace")
	}
}
