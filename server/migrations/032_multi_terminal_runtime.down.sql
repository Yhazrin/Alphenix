DROP INDEX IF EXISTS agent_runtime_unique_idx;
ALTER TABLE agent_runtime DROP COLUMN IF EXISTS instance_id;
ALTER TABLE agent_runtime ADD CONSTRAINT agent_runtime_workspace_id_daemon_id_provider_key
    UNIQUE (workspace_id, daemon_id, provider);
