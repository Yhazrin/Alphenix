-- Add instance_id to support multiple terminal instances per daemon.
-- Each terminal running `multica daemon` gets its own runtime record.
ALTER TABLE agent_runtime ADD COLUMN instance_id TEXT NOT NULL DEFAULT '';

-- Drop the old unique constraint and replace with one that includes instance_id.
ALTER TABLE agent_runtime DROP CONSTRAINT IF EXISTS agent_runtime_workspace_id_daemon_id_provider_key;
CREATE UNIQUE INDEX agent_runtime_unique_idx
    ON agent_runtime(workspace_id, daemon_id, instance_id, provider);
