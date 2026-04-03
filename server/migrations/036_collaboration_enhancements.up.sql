-- Collaboration enhancements: DAG task dependencies, agent messaging improvements,
-- long-term agent memory (pgvector), and task checkpoints.

-- Enable pgvector extension for embeddings.
CREATE EXTENSION IF NOT EXISTS vector;

-- 1. Task dependency DAG — fine-grained task-level ordering within issue workflows.
-- Unlike issue_dependency (issue-level blocking), this tracks individual task execution order.
CREATE TABLE task_dependency (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    depends_on_task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(task_id, depends_on_task_id),
    CHECK (task_id != depends_on_task_id)
);

CREATE INDEX idx_task_dependency_task ON task_dependency(task_id);
CREATE INDEX idx_task_dependency_depends ON task_dependency(depends_on_task_id);

-- 2. Enhance agent_message table for structured inter-agent communication.
ALTER TABLE agent_message
    ADD COLUMN message_type TEXT NOT NULL DEFAULT 'direct'
        CHECK (message_type IN ('direct', 'question', 'answer', 'status_update', 'handoff', 'conflict_alert')),
    ADD COLUMN read_at TIMESTAMPTZ,
    ADD COLUMN reply_to_id UUID REFERENCES agent_message(id) ON DELETE SET NULL;

CREATE INDEX idx_agent_message_unread ON agent_message(to_agent_id, read_at)
    WHERE read_at IS NULL;

-- 3. Agent long-term memory — cross-task knowledge using pgvector embeddings.
-- Agents store observations, patterns, and learnings that persist across tasks.
CREATE TABLE agent_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    embedding vector(1536) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_agent_memory_agent ON agent_memory(agent_id);
CREATE INDEX idx_agent_memory_workspace ON agent_memory(workspace_id);
CREATE INDEX idx_agent_memory_embedding ON agent_memory
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- 4. Task checkpoints — persist intermediate agent execution state for resume capability.
CREATE TABLE task_checkpoint (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    files_changed JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_checkpoint_task ON task_checkpoint(task_id, created_at DESC);
