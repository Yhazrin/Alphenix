-- Task chaining: allow a task to be spawned as a follow-up from another task.

ALTER TABLE agent_task_queue
  ADD COLUMN chain_source_task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
  ADD COLUMN chain_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_task_queue_chain_source
  ON agent_task_queue(chain_source_task_id)
  WHERE chain_source_task_id IS NOT NULL;
