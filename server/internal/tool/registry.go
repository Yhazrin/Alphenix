package tool

import (
	"fmt"
	"sort"
	"sync"
)

// PermissionLevel classifies tool risk.
type PermissionLevel int

const (
	PermissionRead     PermissionLevel = iota // safe reads
	PermissionWrite                            // file writes, edits
	PermissionDangerous                        // shell, delete, destructive
	PermissionNetwork                          // outbound HTTP
)

func (p PermissionLevel) String() string {
	switch p {
	case PermissionRead:
		return "read"
	case PermissionWrite:
		return "write"
	case PermissionDangerous:
		return "dangerous"
	case PermissionNetwork:
		return "network"
	default:
		return "unknown"
	}
}

// ToolSource identifies where a tool originates from.
type ToolSource string

const (
	SourceBuiltin ToolSource = "builtin" // built-in tool (read_file, shell_exec, etc.)
	SourceMCP     ToolSource = "mcp"     // tool from an external MCP server
	SourceSkill   ToolSource = "skill"   // tool provided by a Skill
)

// ToolDef describes a tool available to the agent runtime.
type ToolDef struct {
	Name        string
	Description string
	Permission  PermissionLevel
	// Schema is an optional JSON Schema describing the tool's input format.
	Schema []byte

	// IsConcurrencySafe indicates whether this tool can run concurrently with
	// other tools. Read-only tools (grep, list_files) are safe; write tools
	// (write_file, shell_exec) are not. Defaults to false (fail-closed).
	IsConcurrencySafe bool

	// IsReadOnly indicates whether this tool never modifies state.
	// Used for prompt annotations and concurrency partitioning.
	// Defaults to false (fail-closed).
	IsReadOnly bool

	// Source identifies where this tool comes from.
	// Defaults to SourceBuiltin for tools registered via DefaultRegistry.
	Source ToolSource

	// SourceConfig holds source-specific metadata.
	// For MCP tools: {"server_name": "...", "server_id": "...", "original_tool_name": "..."}
	// For Skill tools: {"skill_id": "...", "skill_name": "..."}
	SourceConfig map[string]any
}

// NamespacedName returns the fully-qualified tool name.
// MCP tools: "mcp.{server}.{tool}", Skill tools: "skill.{name}.{tool}", Builtin: plain name.
func (td ToolDef) NamespacedName() string {
	switch td.Source {
	case SourceMCP:
		server, _ := td.SourceConfig["server_name"].(string)
		if server != "" {
			return fmt.Sprintf("mcp.%s.%s", server, td.Name)
		}
	case SourceSkill:
		skill, _ := td.SourceConfig["skill_name"].(string)
		if skill != "" {
			return fmt.Sprintf("skill.%s.%s", skill, td.Name)
		}
	}
	return td.Name
}

// MCPSourceConfig constructs a SourceConfig map for MCP tools.
func MCPSourceConfig(serverName, toolName, serverID string) map[string]any {
	return map[string]any{
		"server_name":       serverName,
		"original_tool_name": toolName,
		"server_id":         serverID,
	}
}

// SkillSourceConfig constructs a SourceConfig map for Skill tools.
func SkillSourceConfig(skillName, skillID string) map[string]any {
	return map[string]any{
		"skill_name": skillName,
		"skill_id":   skillID,
	}
}

// ChangeEvent describes a tool registry mutation.
type ChangeEvent struct {
	Action string   // "register" or "unregister"
	Names  []string // affected tool names (namespaced)
	Source ToolSource
}

// Registry is a thread-safe tool catalog.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]ToolDef

	// OnChange is called after tools are registered or unregistered.
	// Implementations should be fast and non-blocking (e.g. publish to event bus).
	OnChange func(ChangeEvent)
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]ToolDef),
	}
}

// Register adds or replaces a tool definition.
func (r *Registry) Register(def ToolDef) error {
	if def.Name == "" {
		return fmt.Errorf("tool name must not be empty")
	}
	r.mu.Lock()
	r.tools[def.Name] = def
	r.mu.Unlock()
	r.notifyChange(ChangeEvent{
		Action: "register",
		Names:  []string{def.Name},
		Source: def.Source,
	})
	return nil
}

// Get retrieves a tool definition by name.
func (r *Registry) Get(name string) (ToolDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.tools[name]
	return def, ok
}

// List returns all registered tool definitions.
func (r *Registry) List() []ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolDef, 0, len(r.tools))
	for _, def := range r.tools {
		result = append(result, def)
	}
	return result
}

// SortedTools returns all tool definitions sorted by namespaced name.
// Use this for prompt assembly to ensure stable cache-friendly ordering.
func (r *Registry) SortedTools() []ToolDef {
	result := r.List()
	sort.Slice(result, func(i, j int) bool {
		return result[i].NamespacedName() < result[j].NamespacedName()
	})
	return result
}

// ListByPermission returns tools matching the given permission level.
func (r *Registry) ListByPermission(level PermissionLevel) []ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ToolDef
	for _, def := range r.tools {
		if def.Permission <= level {
			result = append(result, def)
		}
	}
	return result
}

// IsAllowed checks if a tool is available at the given permission ceiling.
func (r *Registry) IsAllowed(name string, maxLevel PermissionLevel) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.tools[name]
	if !ok {
		return false
	}
	return def.Permission <= maxLevel
}

// Names returns all registered tool names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Unregister removes a tool by name. Returns true if the tool existed.
func (r *Registry) Unregister(name string) bool {
	r.mu.Lock()
	def, ok := r.tools[name]
	if ok {
		delete(r.tools, name)
	}
	r.mu.Unlock()
	if ok {
		r.notifyChange(ChangeEvent{
			Action: "unregister",
			Names:  []string{name},
			Source: def.Source,
		})
	}
	return ok
}

// RegisterDynamic registers tools from a dynamic source (MCP, Skill).
// It uses the tool's NamespacedName() as the registry key to avoid collisions.
func (r *Registry) RegisterDynamic(def ToolDef) error {
	if def.Name == "" {
		return fmt.Errorf("tool name must not be empty")
	}
	key := def.NamespacedName()
	r.mu.Lock()
	r.tools[key] = def
	r.mu.Unlock()
	r.notifyChange(ChangeEvent{
		Action: "register",
		Names:  []string{key},
		Source: def.Source,
	})
	return nil
}

// UnregisterBySource removes all tools matching the given source.
// For MCP tools, sourceConfigKey "server_name" with sourceConfigVal removes all tools from that server.
func (r *Registry) UnregisterBySource(source ToolSource) int {
	r.mu.Lock()
	var names []string
	for name, def := range r.tools {
		if def.Source == source {
			delete(r.tools, name)
			names = append(names, name)
		}
	}
	r.mu.Unlock()
	if len(names) > 0 {
		r.notifyChange(ChangeEvent{
			Action: "unregister",
			Names:  names,
			Source: source,
		})
	}
	return len(names)
}

// ListBySource returns all tools from a specific source.
func (r *Registry) ListBySource(source ToolSource) []ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ToolDef
	for _, def := range r.tools {
		if def.Source == source {
			result = append(result, def)
		}
	}
	return result
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// DefaultRegistry returns a registry pre-populated with standard tools.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	tools := []ToolDef{
		{Name: "read_file", Description: "Read the contents of a file", Permission: PermissionRead, IsConcurrencySafe: true, IsReadOnly: true, Source: SourceBuiltin},
		{Name: "list_files", Description: "List files in a directory", Permission: PermissionRead, IsConcurrencySafe: true, IsReadOnly: true, Source: SourceBuiltin},
		{Name: "grep", Description: "Search file contents with regex", Permission: PermissionRead, IsConcurrencySafe: true, IsReadOnly: true, Source: SourceBuiltin},
		{Name: "write_file", Description: "Write content to a file", Permission: PermissionWrite, Source: SourceBuiltin},
		{Name: "edit_file", Description: "Edit a file with search-and-replace", Permission: PermissionWrite, Source: SourceBuiltin},
		{Name: "shell_exec", Description: "Execute a shell command", Permission: PermissionDangerous, Source: SourceBuiltin},
		{Name: "delete_file", Description: "Delete a file", Permission: PermissionDangerous, Source: SourceBuiltin},
		{Name: "http_request", Description: "Make an HTTP request", Permission: PermissionNetwork, Source: SourceBuiltin},
	}
	for _, def := range tools {
		_ = r.Register(def) // all pre-populated names are non-empty
	}
	return r
}

// notifyChange calls the OnChange callback if set. Non-blocking by convention.
func (r *Registry) notifyChange(event ChangeEvent) {
	if r.OnChange != nil {
		r.OnChange(event)
	}
}
