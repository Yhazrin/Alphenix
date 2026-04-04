-- Runtime pool: extend agent_runtime with ownership, approval, and tagging
-- for shared runtime network in a small team.

-- 1. Add runtime pool fields to agent_runtime
ALTER TABLE agent_runtime ADD COLUMN owner_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL;
ALTER TABLE agent_runtime ADD COLUMN approval_status TEXT NOT NULL DEFAULT 'pending' CHECK (approval_status IN ('pending', 'approved', 'rejected', 'revoked'));
ALTER TABLE agent_runtime ADD COLUMN visibility TEXT NOT NULL DEFAULT 'workspace' CHECK (visibility IN ('private', 'workspace', 'team'));
ALTER TABLE agent_runtime ADD COLUMN trust_level TEXT NOT NULL DEFAULT 'trusted_member' CHECK (trust_level IN ('self', 'trusted_member', 'restricted'));
ALTER TABLE agent_runtime ADD COLUMN drain_mode BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE agent_runtime ADD COLUMN paused BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE agent_runtime ADD COLUMN tags JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE agent_runtime ADD COLUMN max_concurrent_tasks_override INT;
ALTER TABLE agent_runtime ADD COLUMN last_claimed_at TIMESTAMPTZ;
ALTER TABLE agent_runtime ADD COLUMN success_count_24h INT NOT NULL DEFAULT 0;
ALTER TABLE agent_runtime ADD COLUMN failure_count_24h INT NOT NULL DEFAULT 0;
ALTER TABLE agent_runtime ADD COLUMN avg_task_duration_ms BIGINT NOT NULL DEFAULT 0;

-- Existing runtimes (before this migration) are considered self-owned and approved.
UPDATE agent_runtime SET owner_user_id = NULL, approval_status = 'approved' WHERE approval_status = 'pending';

-- Indexes for pool queries
CREATE INDEX idx_agent_runtime_owner_user_id ON agent_runtime(owner_user_id);
CREATE INDEX idx_agent_runtime_approval_status ON agent_runtime(approval_status);
CREATE INDEX idx_agent_runtime_paused ON agent_runtime(paused);
CREATE INDEX idx_agent_runtime_drain_mode ON agent_runtime(drain_mode);

-- 2. runtime_join_token table for the join flow
CREATE TABLE runtime_join_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_runtime_join_token_workspace_id ON runtime_join_token(workspace_id);
CREATE INDEX idx_runtime_join_token_token_hash ON runtime_join_token(token_hash);
CREATE INDEX idx_runtime_join_token_expires_at ON runtime_join_token(expires_at);

-- 3. runtime_audit_log for audit trail
CREATE TABLE runtime_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    runtime_id UUID NOT NULL REFERENCES agent_runtime(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_runtime_audit_log_workspace_id ON runtime_audit_log(workspace_id);
CREATE INDEX idx_runtime_audit_log_runtime_id ON runtime_audit_log(runtime_id);
CREATE INDEX idx_runtime_audit_log_created_at ON runtime_audit_log(created_at);