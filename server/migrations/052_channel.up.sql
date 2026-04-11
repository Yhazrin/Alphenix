-- Channels: workspace-scoped projects; issues belong to exactly one channel.
-- Participants (members, agents, teams) can be linked per channel for isolation and UI.

CREATE TABLE channel (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    UNIQUE (workspace_id, slug)
);

CREATE INDEX idx_channel_workspace ON channel (workspace_id) WHERE archived_at IS NULL;

CREATE TABLE channel_participant (
    channel_id UUID NOT NULL REFERENCES channel(id) ON DELETE CASCADE,
    participant_type TEXT NOT NULL CHECK (participant_type IN ('member', 'agent', 'team')),
    participant_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, participant_type, participant_id)
);

CREATE INDEX idx_channel_participant_participant ON channel_participant (participant_type, participant_id);

ALTER TABLE issue ADD COLUMN channel_id UUID REFERENCES channel(id) ON DELETE RESTRICT;

-- One default channel per workspace (General).
INSERT INTO channel (workspace_id, name, slug, description, is_default)
SELECT w.id, 'General', 'general', 'Default channel for this workspace', true
FROM workspace w;

UPDATE issue i
SET channel_id = c.id
FROM channel c
WHERE c.workspace_id = i.workspace_id AND c.slug = 'general';

ALTER TABLE issue ALTER COLUMN channel_id SET NOT NULL;

-- Backfill participants for General: all workspace members, agents, and teams.
INSERT INTO channel_participant (channel_id, participant_type, participant_id)
SELECT c.id, 'member', m.user_id
FROM channel c
JOIN member m ON m.workspace_id = c.workspace_id
WHERE c.slug = 'general'
ON CONFLICT DO NOTHING;

INSERT INTO channel_participant (channel_id, participant_type, participant_id)
SELECT c.id, 'agent', a.id
FROM channel c
JOIN agent a ON a.workspace_id = c.workspace_id
WHERE c.slug = 'general' AND a.archived_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO channel_participant (channel_id, participant_type, participant_id)
SELECT c.id, 'team', t.id
FROM channel c
JOIN team t ON t.workspace_id = c.workspace_id
WHERE c.slug = 'general' AND t.archived_at IS NULL
ON CONFLICT DO NOTHING;
