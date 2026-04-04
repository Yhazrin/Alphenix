package protocol

import "encoding/json"

// Message is the envelope for all WebSocket messages.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// TaskDispatchPayload is sent from server to daemon when a task is assigned.
type TaskDispatchPayload struct {
	TaskID      string `json:"task_id"`
	IssueID     string `json:"issue_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// TaskProgressPayload is sent from daemon to server during task execution.
type TaskProgressPayload struct {
	TaskID  string `json:"task_id"`
	Summary string `json:"summary"`
	Step    int    `json:"step,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// TaskCompletedPayload is sent from daemon to server when a task finishes.
type TaskCompletedPayload struct {
	TaskID string `json:"task_id"`
	PRURL  string `json:"pr_url,omitempty"`
	Output string `json:"output,omitempty"`
}

// TaskMessagePayload represents a single agent execution message (tool call, text, etc.)
type TaskMessagePayload struct {
	TaskID  string         `json:"task_id"`
	IssueID string         `json:"issue_id,omitempty"`
	Seq     int            `json:"seq"`
	Type    string         `json:"type"`              // "text", "tool_use", "tool_result", "error"
	Tool    string         `json:"tool,omitempty"`     // tool name for tool_use/tool_result
	Content string         `json:"content,omitempty"`  // text content
	Input   map[string]any `json:"input,omitempty"`    // tool input (tool_use only)
	Output  string         `json:"output,omitempty"`   // tool output (tool_result only)
}

// DaemonRegisterPayload is sent from daemon to server on connection.
type DaemonRegisterPayload struct {
	DaemonID string        `json:"daemon_id"`
	AgentID  string        `json:"agent_id"`
	Runtimes []RuntimeInfo `json:"runtimes"`
}

// RuntimeInfo describes an available agent runtime on the daemon's machine.
type RuntimeInfo struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

// HeartbeatPayload is sent periodically from daemon to server.
type HeartbeatPayload struct {
	DaemonID     string `json:"daemon_id"`
	AgentID      string `json:"agent_id"`
	CurrentTasks int    `json:"current_tasks"`
}

// AgentMessagePayload is sent when an agent sends a message to another agent.
type AgentMessagePayload struct {
	MessageID   string `json:"message_id"`
	FromAgentID string `json:"from_agent_id"`
	ToAgentID   string `json:"to_agent_id"`
	TaskID      string `json:"task_id,omitempty"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
	ReplyToID   string `json:"reply_to_id,omitempty"`
}

// TaskDependencyPayload is sent when a task dependency is created or removed.
type TaskDependencyPayload struct {
	TaskID          string `json:"task_id"`
	DependsOnTaskID string `json:"depends_on_task_id"`
}

// TaskCheckpointPayload is sent when an agent saves a checkpoint.
type TaskCheckpointPayload struct {
	CheckpointID string `json:"checkpoint_id"`
	TaskID       string `json:"task_id"`
	Label        string `json:"label"`
}

// ColleagueInfo provides agent context for collaborative prompts.
type ColleagueInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Role        string `json:"role,omitempty"`
	Status      string `json:"status,omitempty"`
}

// SharedContext holds workspace-level context injected into collaborative agent prompts.
type SharedContext struct {
	Colleagues       []ColleagueInfo        `json:"colleagues,omitempty"`
	PendingMessages  []AgentMessagePayload  `json:"pending_messages,omitempty"`
	Dependencies     []TaskDependencyInfo   `json:"dependencies,omitempty"`
	WorkspaceMemory  []MemoryRecall         `json:"workspace_memory,omitempty"`
	LastCheckpoint   *CheckpointInfo        `json:"last_checkpoint,omitempty"`
}

// TaskDependencyInfo describes a dependency relationship for prompt injection.
type TaskDependencyInfo struct {
	TaskID         string `json:"task_id"`
	DependsOnID    string `json:"depends_on_id"`
	DependencyStatus string `json:"dependency_status"`
}

// MemoryRecall represents a recalled memory entry for prompt injection.
type MemoryRecall struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Similarity float64 `json:"similarity"`
	AgentName  string  `json:"agent_name,omitempty"`
	BM25Score  float64 `json:"bm25_score,omitempty"`
	FusedScore float64 `json:"fused_score,omitempty"`
	SearchType string  `json:"search_type,omitempty"` // "hybrid", "vector", "bm25", "recent"
}

// CheckpointInfo describes the latest checkpoint for resume context.
type CheckpointInfo struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	State        any    `json:"state,omitempty"`
	FilesChanged any    `json:"files_changed,omitempty"`
	CreatedAt    string `json:"created_at"`
}
