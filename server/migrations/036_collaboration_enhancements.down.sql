DROP TABLE IF EXISTS task_checkpoint;
DROP TABLE IF EXISTS agent_memory;
ALTER TABLE agent_message DROP COLUMN IF EXISTS message_type;
ALTER TABLE agent_message DROP COLUMN IF EXISTS read_at;
ALTER TABLE agent_message DROP COLUMN IF EXISTS reply_to_id;
DROP INDEX IF EXISTS idx_agent_message_unread;
DROP TABLE IF EXISTS task_dependency;
