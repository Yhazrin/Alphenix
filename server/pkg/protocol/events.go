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
	EventAgentUpdated  = "agent:updated"
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
	EventAgentToolUse      = "agent:tool_use"
	EventAgentToolResult   = "agent:tool_result"
	EventAgentStarted      = "agent:started"
	EventAgentCompleted    = "agent:completed"
	EventAgentFailed       = "agent:failed"
	EventAgentStop         = "agent:stop"
	EventAgentSessionStart = "agent:session_start"

	// Fork lifecycle events (multi-agent parallel orchestration)
	EventForkStarted   = "agent:fork_started"
	EventForkCompleted = "agent:fork_completed"
	EventForkFailed    = "agent:fork_failed"

	// Run events (RunOrchestrator lifecycle)
	EventRunCreated       = "run:created"
	EventRunStarted       = "run:started"
	EventRunPhaseChanged  = "run:phase_changed"
	EventRunCompleted     = "run:completed"
	EventRunFailed        = "run:failed"
	EventRunCancelled     = "run:cancelled"
	EventRunStepStarted   = "run:step_started"
	EventRunStepCompleted = "run:step_completed"
	EventRunTodoCreated   = "run:todo_created"
	EventRunTodoUpdated   = "run:todo_updated"

	// Team events
	EventTeamCreated       = "team:created"
	EventTeamUpdated       = "team:updated"
	EventTeamDeleted       = "team:deleted"
	EventTeamMemberAdded   = "team:member_added"
	EventTeamMemberRemoved = "team:member_removed"
)
