ALTER TABLE agent_task_queue
  DROP COLUMN IF EXISTS chain_source_task_id,
  DROP COLUMN IF EXISTS chain_reason;
