DROP TABLE IF EXISTS task_review;

ALTER TABLE agent_task_queue
    DROP COLUMN IF EXISTS review_status,
    DROP COLUMN IF EXISTS review_count,
    DROP COLUMN IF EXISTS max_reviews;

ALTER TABLE agent_task_queue
    DROP CONSTRAINT IF EXISTS agent_task_queue_status_check;

ALTER TABLE agent_task_queue
    ADD CONSTRAINT agent_task_queue_status_check
    CHECK (status IN ('queued', 'dispatched', 'running', 'completed', 'failed', 'cancelled'));
