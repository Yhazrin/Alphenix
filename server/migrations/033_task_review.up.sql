-- Task review table — stores review results for completed tasks.
CREATE TABLE task_review (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    reviewer_type TEXT NOT NULL CHECK (reviewer_type IN ('automated', 'agent', 'member')),
    reviewer_id UUID,
    verdict TEXT NOT NULL CHECK (verdict IN ('pass', 'fail', 'retry')),
    score INTEGER CHECK (score BETWEEN 0 AND 100),
    feedback TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_review_task ON task_review(task_id);

-- Add 'in_review' to the task status constraint.
ALTER TABLE agent_task_queue
    DROP CONSTRAINT IF EXISTS agent_task_queue_status_check;

ALTER TABLE agent_task_queue
    ADD CONSTRAINT agent_task_queue_status_check
    CHECK (status IN ('queued', 'dispatched', 'running', 'in_review', 'completed', 'failed', 'cancelled'));

-- Add review tracking columns.
ALTER TABLE agent_task_queue
    ADD COLUMN review_status TEXT NOT NULL DEFAULT 'none'
        CHECK (review_status IN ('none', 'pending', 'passed', 'failed')),
    ADD COLUMN review_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN max_reviews INTEGER NOT NULL DEFAULT 1;
