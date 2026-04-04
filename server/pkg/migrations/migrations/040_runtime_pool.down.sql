-- Rollback runtime pool

DROP TABLE IF EXISTS runtime_audit_log;
DROP TABLE IF EXISTS runtime_join_token;

ALTER TABLE agent_runtime DROP COLUMN IF EXISTS owner_user_id;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS approval_status;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS visibility;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS trust_level;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS drain_mode;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS paused;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS tags;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS max_concurrent_tasks_override;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS last_claimed_at;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS success_count_24h;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS failure_count_24h;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS avg_task_duration_ms;