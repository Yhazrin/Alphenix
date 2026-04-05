CREATE TABLE IF NOT EXISTS mcp_servers (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    transport       TEXT        NOT NULL DEFAULT 'stdio',
    url             TEXT        NOT NULL DEFAULT '',
    command         TEXT        NOT NULL DEFAULT '',
    args            JSONB       NOT NULL DEFAULT '[]'::jsonb,
    env             JSONB       NOT NULL DEFAULT '{}'::jsonb,
    status          TEXT        NOT NULL DEFAULT 'disconnected',
    last_connected_at TIMESTAMPTZ,
    last_error      TEXT        NOT NULL DEFAULT '',
    config          JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT mcp_servers_workspace_name_unique UNIQUE (workspace_id, name)
);

CREATE INDEX idx_mcp_servers_workspace_id ON mcp_servers (workspace_id);
CREATE INDEX idx_mcp_servers_status ON mcp_servers (status);
