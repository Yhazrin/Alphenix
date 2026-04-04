package protocol

// Event types for WebSocket communication between server, web clients, and daemon.
const (
	// Issue events
	EventIssueCreated = "issue:created"
	EventIssueUpdated = "issue:updated"
	EventIssueDeleted = "issue:deleted"

	// Comment events
	EventCommentCreated       = "comment:created"
	EventCommentUpdated       = "comment:updated"
	EventCommentDeleted       = "comment:deleted"
	EventReactionAdded          = "reaction:added"
	EventReactionRemoved        = "reaction:removed"
	EventIssueReactionAdded     = "issue_reaction:added"
	EventIssueReactionRemoved   = "issue_reaction:removed"

	// Agent events
	EventAgentStatus   = "agent:status"
	EventAgentCreated  = "agent:created"
	EventAgentArchived = "agent:archived"
	EventAgentRestored = "agent:restored"

	// Task events (server <-> daemon)
	EventTaskDispatch  = "task:dispatch"
	EventTaskProgress  = "task:progress"
	EventTaskCompleted = "task:completed"
	EventTaskFailed    = "task:failed"
	EventTaskMessage   = "task:message"
	EventTaskCancelled = "task:cancelled"

	// Inbox events
	EventInboxNew           = "inbox:new"
	EventInboxRead          = "inbox:read"
	EventInboxArchived      = "inbox:archived"
	EventInboxBatchRead     = "inbox:batch-read"
	EventInboxBatchArchived = "inbox:batch-archived"

	// Workspace events
	EventWorkspaceUpdated = "workspace:updated"
	EventWorkspaceDeleted = "workspace:deleted"

	// Member events
	EventMemberAdded   = "member:added"
	EventMemberUpdated = "member:updated"
	EventMemberRemoved = "member:removed"

	// Subscriber events
	EventSubscriberAdded   = "subscriber:added"
	EventSubscriberRemoved = "subscriber:removed"

	// Activity events
	EventActivityCreated = "activity:created"

	// Skill events
	EventSkillCreated = "skill:created"
	EventSkillUpdated = "skill:updated"
	EventSkillDeleted = "skill:deleted"

	// Agent communication events
	EventAgentMessage = "agent:message"

	// Task chain events
	EventTaskChained  = "task:chained"
	EventTaskInReview = "task:in_review"
	EventTaskReviewed = "task:reviewed"

	// Collaboration events
	EventTaskDependencyCreated = "task_dep:created"
	EventTaskDependencyDeleted = "task_dep:deleted"
	EventTaskCheckpointCreated = "task:checkpoint"
	EventMemoryStored          = "memory:stored"
	EventMemoryRecalled        = "memory:recalled"

	// Daemon events
	EventDaemonHeartbeat = "daemon:heartbeat"
	EventDaemonRegister  = "daemon:register"

	// Agent lifecycle events (hook-driven, published by daemon)
	EventAgentToolUse    = "agent:tool_use"
	EventAgentToolResult = "agent:tool_result"
	EventAgentStarted    = "agent:started"
	EventAgentCompleted  = "agent:completed"
	EventAgentFailed     = "agent:failed"
	EventAgentStop       = "agent:stop"
	EventAgentSessionStart = "agent:session_start"
)
