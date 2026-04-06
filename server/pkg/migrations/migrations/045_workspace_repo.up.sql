-- Workspace Repo: structured repository table replacing workspace.repos JSONB.
-- Each workspace can have multiple repos, each with a stable ID for FK references.

CREATE TABLE workspace_repo (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,                    -- Display name, e.g. "alphenix"
    url TEXT NOT NULL,                     -- Git URL
    default_branch TEXT NOT NULL DEFAULT 'main',
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,  -- At most one default per workspace
    -- config: extensible JSON object for repo-specific settings.
    -- Expected schema: { "clone_depth"?: int, "env"?: object, "pre_build"?: string }
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

-- Ensure at most one default repo per workspace
CREATE UNIQUE INDEX idx_workspace_repo_default
    ON workspace_repo(workspace_id)
    WHERE is_default = true;

-- Issue can optionally belong to a repo
ALTER TABLE issue ADD COLUMN repo_id UUID REFERENCES workspace_repo(id) ON DELETE SET NULL;

CREATE INDEX idx_workspace_repo_workspace ON workspace_repo(workspace_id);
CREATE INDEX idx_issue_repo ON issue(repo_id);
