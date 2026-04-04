-- Team feature: group agents into teams with shared purpose.
-- Tasks can be assigned to teams (lead delegation or broadcast mode).

-- 1. Extend issue assignee_type to include 'team'.
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_assignee_type_check;
ALTER TABLE issue ADD CONSTRAINT issue_assignee_type_check CHECK (assignee_type IN ('member', 'agent', 'team'));

-- 2. Team table - a group of agents with shared purpose.
CREATE TABLE team (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    avatar_url TEXT,
    lead_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    created_by UUID REFERENCES "user"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    archived_by UUID REFERENCES "user"(id)
);

CREATE INDEX idx_team_workspace ON team(workspace_id);
CREATE INDEX idx_team_lead_agent ON team(lead_agent_id) WHERE lead_agent_id IS NOT NULL;

-- 3. Team member junction table.
CREATE TABLE team_member (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES team(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'lead')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(team_id, agent_id)
);

CREATE INDEX idx_team_member_team ON team_member(team_id);
CREATE INDEX idx_team_member_agent ON team_member(agent_id);

-- 4. Team task queue - for team-level task assignment.
CREATE TABLE team_task_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES team(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    assigned_by UUID NOT NULL REFERENCES "user"(id),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delegated', 'completed', 'cancelled')),
    delegated_to_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_team_task_queue_team ON team_task_queue(team_id);
CREATE INDEX idx_team_task_queue_issue ON team_task_queue(issue_id);
CREATE INDEX idx_team_task_queue_status ON team_task_queue(status) WHERE status = 'pending';
