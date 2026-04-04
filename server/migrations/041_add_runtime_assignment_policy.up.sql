-- Runtime Assignment Policy: tag-based scheduling rules for agent-to-runtime binding.

CREATE TABLE IF NOT EXISTS runtime_assignment_policy (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    team_id UUID REFERENCES team(id) ON DELETE SET NULL,

    required_tags JSONB NOT NULL DEFAULT '[]',
    forbidden_tags JSONB NOT NULL DEFAULT '[]',
    preferred_runtime_ids JSONB NOT NULL DEFAULT '[]',
    fallback_runtime_ids JSONB NOT NULL DEFAULT '[]',
    max_queue_depth INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (workspace_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_runtime_policy_workspace ON runtime_assignment_policy(workspace_id);
CREATE INDEX IF NOT EXISTS idx_runtime_policy_agent ON runtime_assignment_policy(agent_id);
CREATE INDEX IF NOT EXISTS idx_runtime_policy_team ON runtime_assignment_policy(team_id) WHERE team_id IS NOT NULL;
