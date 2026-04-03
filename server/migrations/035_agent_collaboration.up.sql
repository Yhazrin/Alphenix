-- Agent-to-agent direct messages for real-time collaboration during task execution.

CREATE TABLE agent_message (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    from_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    to_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agent_message_to ON agent_message(to_agent_id, created_at);
CREATE INDEX idx_agent_message_from ON agent_message(from_agent_id, created_at);
CREATE INDEX idx_agent_message_task ON agent_message(task_id);
