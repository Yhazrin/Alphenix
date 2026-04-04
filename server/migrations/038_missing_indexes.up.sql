-- Add missing indexes on frequently queried foreign key and filter columns.

-- agent_task_queue.issue_id: used in CancelAgentTasksByIssue, HasPendingTaskForIssue,
-- HasActiveTaskForIssue, ListTasksByIssue, ClaimAgentTask (subquery), GetLastTaskSession
CREATE INDEX IF NOT EXISTS idx_agent_task_queue_issue ON agent_task_queue(issue_id);

-- activity_log: queried by issue_id (already indexed), workspace_id, and ordered by created_at
CREATE INDEX IF NOT EXISTS idx_activity_log_workspace_created ON activity_log(workspace_id, created_at DESC);

-- comment: queried by issue_id and ordered by created_at
CREATE INDEX IF NOT EXISTS idx_comment_issue_created ON comment(issue_id, created_at);

-- issue_label: queried by workspace_id
CREATE INDEX IF NOT EXISTS idx_issue_label_workspace ON issue_label(workspace_id);

-- issue_to_label: PK covers (issue_id), missing reverse lookup by label_id
CREATE INDEX IF NOT EXISTS idx_issue_to_label_label ON issue_to_label(label_id);

-- issue_dependency: no indexes on either FK column
CREATE INDEX IF NOT EXISTS idx_issue_dependency_issue ON issue_dependency(issue_id);
CREATE INDEX IF NOT EXISTS idx_issue_dependency_depends ON issue_dependency(depends_on_issue_id);

-- daemon_connection: queried by agent_id
CREATE INDEX IF NOT EXISTS idx_daemon_connection_agent ON daemon_connection(agent_id);

-- member: queried by user_id for membership lookups
CREATE INDEX IF NOT EXISTS idx_member_user ON member(user_id);

-- agent: queried by owner_id for private agent visibility checks
CREATE INDEX IF NOT EXISTS idx_agent_owner ON agent(owner_id) WHERE owner_id IS NOT NULL;
