package tool

import (
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	err := r.Register(ToolDef{
		Name:        "read_file",
		Description: "Read a file",
		Permission:  PermissionRead,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	def, ok := r.Get("read_file")
	if !ok {
		t.Fatal("Get() returned false")
	}
	if def.Description != "Read a file" {
		t.Errorf("Description = %q, want %q", def.Description, "Read a file")
	}
}

func TestRegistry_RegisterEmptyName(t *testing.T) {
	r := NewRegistry()
	err := r.Register(ToolDef{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegistry_ListByPermission(t *testing.T) {
	r := DefaultRegistry()

	// Read-only should get only read tools
	readTools := r.ListByPermission(PermissionRead)
	for _, def := range readTools {
		if def.Permission > PermissionRead {
			t.Errorf("tool %s has permission %v, expected <= read", def.Name, def.Permission)
		}
	}

	// Dangerous should get read + write + dangerous
	dangerous := r.ListByPermission(PermissionDangerous)
	if len(dangerous) < len(readTools) {
		t.Error("dangerous permission should include more tools than read-only")
	}
}

func TestRegistry_IsAllowed(t *testing.T) {
	r := DefaultRegistry()

	if !r.IsAllowed("read_file", PermissionRead) {
		t.Error("read_file should be allowed at read level")
	}
	if r.IsAllowed("shell_exec", PermissionRead) {
		t.Error("shell_exec should NOT be allowed at read level")
	}
	if !r.IsAllowed("shell_exec", PermissionDangerous) {
		t.Error("shell_exec should be allowed at dangerous level")
	}
	if r.IsAllowed("nonexistent", PermissionDangerous) {
		t.Error("nonexistent tool should not be allowed")
	}
}

func TestRegistry_Count(t *testing.T) {
	r := DefaultRegistry()
	if r.Count() != 8 {
		t.Errorf("Count() = %d, want 8", r.Count())
	}
}

func TestRegistry_Names(t *testing.T) {
	r := DefaultRegistry()
	names := r.Names()
	if len(names) != 8 {
		t.Errorf("Names() returned %d names, want 8", len(names))
	}
}

func TestDefaultRegistry_SourceIsBuiltin(t *testing.T) {
	r := DefaultRegistry()
	for _, def := range r.List() {
		if def.Source != SourceBuiltin {
			t.Errorf("tool %s has source %q, want %q", def.Name, def.Source, SourceBuiltin)
		}
	}
}

func TestRegistryOnChange(t *testing.T) {
	t.Run("register triggers OnChange", func(t *testing.T) {
		r := NewRegistry()
		var events []ChangeEvent
		r.OnChange = func(e ChangeEvent) {
			events = append(events, e)
		}

		_ = r.Register(ToolDef{Name: "tool1", Source: SourceBuiltin})
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Action != "register" {
			t.Errorf("expected action 'register', got %q", events[0].Action)
		}
		if len(events[0].Names) != 1 || events[0].Names[0] != "tool1" {
			t.Errorf("expected names [tool1], got %v", events[0].Names)
		}
	})

	t.Run("unregister triggers OnChange", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(ToolDef{Name: "tool1"})

		var events []ChangeEvent
		r.OnChange = func(e ChangeEvent) {
			events = append(events, e)
		}

		r.Unregister("tool1")
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Action != "unregister" {
			t.Errorf("expected action 'unregister', got %q", events[0].Action)
		}
	})

	t.Run("unregister nonexistent does not trigger OnChange", func(t *testing.T) {
		r := NewRegistry()
		called := false
		r.OnChange = func(e ChangeEvent) {
			called = true
		}

		r.Unregister("nonexistent")
		if called {
			t.Error("OnChange should not be called for nonexistent tool")
		}
	})

	t.Run("register dynamic triggers OnChange with namespaced name", func(t *testing.T) {
		r := NewRegistry()
		var events []ChangeEvent
		r.OnChange = func(e ChangeEvent) {
			events = append(events, e)
		}

		_ = r.RegisterDynamic(ToolDef{
			Name:   "calc",
			Source: SourceMCP,
			SourceConfig: MCPSourceConfig("math-server", "calc", "srv-1"),
		})
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Names[0] != "mcp.math-server.calc" {
			t.Errorf("expected namespaced name 'mcp.math-server.calc', got %q", events[0].Names[0])
		}
	})

	t.Run("nil OnChange does not panic", func(t *testing.T) {
		r := NewRegistry()
		// OnChange is nil — should not panic.
		_ = r.Register(ToolDef{Name: "tool1"})
		r.Unregister("tool1")
	})
}

func TestRegistry_RegisterDynamic_MCPTool(t *testing.T) {
	r := DefaultRegistry()
	err := r.RegisterDynamic(ToolDef{
		Name:        "query",
		Description: "Run a SQL query",
		Permission:  PermissionNetwork,
		Source:      SourceMCP,
		SourceConfig: MCPSourceConfig("postgres", "query", "uuid-1"),
	})
	if err != nil {
		t.Fatalf("RegisterDynamic() error = %v", err)
	}

	// Should be stored under namespaced key
	def, ok := r.Get("mcp.postgres.query")
	if !ok {
		t.Fatal("Get(\"mcp.postgres.query\") returned false")
	}
	if def.Source != SourceMCP {
		t.Errorf("Source = %q, want %q", def.Source, SourceMCP)
	}

	// Count should increase
	if r.Count() != 9 {
		t.Errorf("Count() = %d, want 9", r.Count())
	}
}

func TestRegistry_RegisterDynamic_EmptyName(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterDynamic(ToolDef{
		Name:   "",
		Source: SourceMCP,
	})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := DefaultRegistry()

	// Remove existing tool
	if !r.Unregister("shell_exec") {
		t.Error("Unregister(\"shell_exec\") returned false")
	}
	if r.Count() != 7 {
		t.Errorf("Count() = %d, want 7", r.Count())
	}
	if r.IsAllowed("shell_exec", PermissionDangerous) {
		t.Error("shell_exec should not exist after unregister")
	}

	// Remove non-existent tool
	if r.Unregister("nonexistent") {
		t.Error("Unregister(\"nonexistent\") returned true")
	}
}

func TestRegistry_UnregisterBySource(t *testing.T) {
	r := DefaultRegistry()

	// Add MCP tools
	_ = r.RegisterDynamic(ToolDef{Name: "query", Source: SourceMCP, SourceConfig: map[string]any{"server_name": "postgres"}})
	_ = r.RegisterDynamic(ToolDef{Name: "search", Source: SourceMCP, SourceConfig: map[string]any{"server_name": "elastic"}})
	// Add a Skill tool
	_ = r.RegisterDynamic(ToolDef{Name: "lint", Source: SourceSkill, SourceConfig: map[string]any{"skill_name": "golang"}})

	before := r.Count() // 8 builtin + 3 dynamic = 11
	if before != 11 {
		t.Errorf("Count() before unregister = %d, want 11", before)
	}

	// Remove all MCP tools
	n := r.UnregisterBySource(SourceMCP)
	if n != 2 {
		t.Errorf("UnregisterBySource(mcp) removed %d, want 2", n)
	}
	if r.Count() != 9 {
		t.Errorf("Count() after mcp unregister = %d, want 9", r.Count())
	}

	// Builtin and Skill tools should still be there
	if _, ok := r.Get("read_file"); !ok {
		t.Error("builtin tool read_file should still exist")
	}
	if _, ok := r.Get("skill.golang.lint"); !ok {
		t.Error("skill tool should still exist")
	}
}

func TestRegistry_ListBySource(t *testing.T) {
	r := DefaultRegistry()
	_ = r.RegisterDynamic(ToolDef{Name: "query", Source: SourceMCP, SourceConfig: map[string]any{"server_name": "postgres"}})
	_ = r.RegisterDynamic(ToolDef{Name: "lint", Source: SourceSkill, SourceConfig: map[string]any{"skill_name": "golang"}})

	mcpTools := r.ListBySource(SourceMCP)
	if len(mcpTools) != 1 {
		t.Errorf("ListBySource(mcp) returned %d tools, want 1", len(mcpTools))
	}

	skillTools := r.ListBySource(SourceSkill)
	if len(skillTools) != 1 {
		t.Errorf("ListBySource(skill) returned %d tools, want 1", len(skillTools))
	}

	builtinTools := r.ListBySource(SourceBuiltin)
	if len(builtinTools) != 8 {
		t.Errorf("ListBySource(builtin) returned %d tools, want 8", len(builtinTools))
	}
}

func TestToolDef_NamespacedName(t *testing.T) {
	tests := []struct {
		name string
		def  ToolDef
		want string
	}{
		{
			name: "builtin",
			def:  ToolDef{Name: "read_file", Source: SourceBuiltin},
			want: "read_file",
		},
		{
			name: "mcp with server",
			def: ToolDef{
				Name:         "query",
				Source:       SourceMCP,
				SourceConfig: map[string]any{"server_name": "postgres"},
			},
			want: "mcp.postgres.query",
		},
		{
			name: "mcp without server",
			def:  ToolDef{Name: "query", Source: SourceMCP},
			want: "query",
		},
		{
			name: "skill with name",
			def: ToolDef{
				Name:         "lint",
				Source:       SourceSkill,
				SourceConfig: map[string]any{"skill_name": "golang"},
			},
			want: "skill.golang.lint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.NamespacedName()
			if got != tt.want {
				t.Errorf("NamespacedName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- PermissionLevel.String ---

func TestPermissionLevel_String(t *testing.T) {
	tests := []struct {
		level PermissionLevel
		want  string
	}{
		{PermissionRead, "read"},
		{PermissionWrite, "write"},
		{PermissionDangerous, "dangerous"},
		{PermissionNetwork, "network"},
		{PermissionLevel(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("PermissionLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// --- SortedTools ---

func TestSortedTools_ReturnsSortedByName(t *testing.T) {
	r := NewRegistry()
	r.Register(ToolDef{Name: "zebra"})
	r.Register(ToolDef{Name: "alpha"})
	r.Register(ToolDef{Name: "middle"})

	sorted := r.SortedTools()
	if len(sorted) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(sorted))
	}
	if sorted[0].Name != "alpha" || sorted[1].Name != "middle" || sorted[2].Name != "zebra" {
		t.Errorf("tools not sorted: %v", []string{sorted[0].Name, sorted[1].Name, sorted[2].Name})
	}
}

func TestSortedTools_EmptyRegistry(t *testing.T) {
	r := NewRegistry()
	sorted := r.SortedTools()
	if len(sorted) != 0 {
		t.Errorf("empty registry should return empty slice, got %d", len(sorted))
	}
}

func TestSortedTools_StableOrder(t *testing.T) {
	r := NewRegistry()
	r.Register(ToolDef{Name: "read", Source: SourceMCP, SourceConfig: map[string]any{"server_name": "srv1"}})
	r.Register(ToolDef{Name: "write", Source: SourceBuiltin})
	r.Register(ToolDef{Name: "exec", Source: SourceMCP, SourceConfig: map[string]any{"server_name": "aaa"}})

	sorted := r.SortedTools()
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].NamespacedName() > sorted[i].NamespacedName() {
			t.Errorf("tools not sorted at index %d: %q > %q",
				i, sorted[i-1].NamespacedName(), sorted[i].NamespacedName())
		}
	}
}

// ---------------------------------------------------------------------------
// MCPSourceConfig / SkillSourceConfig
// ---------------------------------------------------------------------------

func TestMCPSourceConfig_ReturnsCorrectMap(t *testing.T) {
	cfg := MCPSourceConfig("postgres", "query", "uuid-1")
	if cfg["server_name"] != "postgres" {
		t.Errorf("server_name = %q, want %q", cfg["server_name"], "postgres")
	}
	if cfg["original_tool_name"] != "query" {
		t.Errorf("original_tool_name = %q, want %q", cfg["original_tool_name"], "query")
	}
	if cfg["server_id"] != "uuid-1" {
		t.Errorf("server_id = %q, want %q", cfg["server_id"], "uuid-1")
	}
}

func TestSkillSourceConfig_ReturnsCorrectMap(t *testing.T) {
	cfg := SkillSourceConfig("golang", "lint")
	if cfg["skill_name"] != "golang" {
		t.Errorf("skill_name = %q, want %q", cfg["skill_name"], "golang")
	}
	if cfg["skill_id"] != "lint" {
		t.Errorf("skill_id = %q, want %q", cfg["skill_id"], "lint")
	}
}

// ---------------------------------------------------------------------------
// Registry.List
// ---------------------------------------------------------------------------

func TestRegistry_List_ReturnsAllTools(t *testing.T) {
	r := NewRegistry()
	r.Register(ToolDef{Name: "a"})
	r.Register(ToolDef{Name: "b"})
	r.Register(ToolDef{Name: "c"})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("List() returned %d tools, want 3", len(list))
	}
}

func TestRegistry_List_EmptyRegistry(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Errorf("empty registry List() returned %d, want 0", len(list))
	}
}

// ---------------------------------------------------------------------------
// Register overwrite
// ---------------------------------------------------------------------------

func TestRegistry_RegisterOverwrite(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(ToolDef{Name: "tool1", Description: "v1"})
	_ = r.Register(ToolDef{Name: "tool1", Description: "v2"})

	def, ok := r.Get("tool1")
	if !ok {
		t.Fatal("Get(\"tool1\") returned false")
	}
	if def.Description != "v2" {
		t.Errorf("Description = %q, want %q", def.Description, "v2")
	}
	if r.Count() != 1 {
		t.Errorf("Count() = %d, want 1 after overwrite", r.Count())
	}
}

// ---------------------------------------------------------------------------
// Get missing
// ---------------------------------------------------------------------------

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") should return false")
	}
}

// ---------------------------------------------------------------------------
// UnregisterBySource triggers OnChange
// ---------------------------------------------------------------------------

func TestRegistry_UnregisterBySource_TriggersOnChange(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterDynamic(ToolDef{Name: "q1", Source: SourceMCP, SourceConfig: map[string]any{"server_name": "srv1"}})
	_ = r.RegisterDynamic(ToolDef{Name: "q2", Source: SourceMCP, SourceConfig: map[string]any{"server_name": "srv2"}})

	var events []ChangeEvent
	r.OnChange = func(e ChangeEvent) {
		events = append(events, e)
	}

	n := r.UnregisterBySource(SourceMCP)
	if n != 2 {
		t.Fatalf("UnregisterBySource removed %d, want 2", n)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 OnChange event, got %d", len(events))
	}
	if events[0].Action != "unregister" {
		t.Errorf("expected action 'unregister', got %q", events[0].Action)
	}
	if len(events[0].Names) != 2 {
		t.Errorf("expected 2 names in event, got %d", len(events[0].Names))
	}
}

// ---------------------------------------------------------------------------
// ListBySource no match
// ---------------------------------------------------------------------------

func TestRegistry_ListBySource_NoMatch(t *testing.T) {
	r := DefaultRegistry()
	tools := r.ListBySource("nonexistent_source")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools for nonexistent source, got %d", len(tools))
	}
}
