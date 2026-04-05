-- Run Orchestrator tables: execution kernel for structured agent runs.

-- 0. Base tables (may already exist in production)
CREATE TABLE IF NOT EXISTS outbox_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    workspace_id UUID REFERENCES workspace(id) ON DELETE CASCADE,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 1. runs: top-level execution lifecycle
CREATE TABLE IF NOT EXISTS runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    parent_run_id UUID REFERENCES runs(id) ON DELETE SET NULL,
    team_id UUID,

    -- Lifecycle
    phase TEXT NOT NULL DEFAULT 'pending',   -- pending | planning | executing | reviewing | completed | failed | cancelled
    status TEXT NOT NULL DEFAULT 'active',   -- active | completed | failed | cancelled

    -- Prompt
    system_prompt TEXT NOT NULL DEFAULT '',
    model_name TEXT NOT NULL DEFAULT '',

    -- Permissions
    permission_mode TEXT NOT NULL DEFAULT 'default', -- default | bypass | plan

    -- Token / cost tracking
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    estimated_cost_usd NUMERIC(10, 6) NOT NULL DEFAULT 0,

    -- Timing
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_runs_workspace ON runs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_runs_issue ON runs(issue_id);
CREATE INDEX IF NOT EXISTS idx_runs_task ON runs(task_id) WHERE task_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_runs_agent ON runs(agent_id);
CREATE INDEX IF NOT EXISTS idx_runs_parent ON runs(parent_run_id) WHERE parent_run_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_runs_phase ON runs(phase) WHERE status = 'active';

-- 2. run_steps: individual tool calls within a run
CREATE TABLE IF NOT EXISTS run_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,

    seq INT NOT NULL,                       -- ordering within the run
    tool_name TEXT NOT NULL,
    tool_input JSONB,
    tool_output TEXT,
    is_error BOOLEAN NOT NULL DEFAULT false,

    -- Timing
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,

    UNIQUE(run_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_run_steps_run ON run_steps(run_id, seq);

-- 3. run_todos: structured execution plan
CREATE TABLE IF NOT EXISTS run_todos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,

    seq INT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending', -- pending | in_progress | completed | blocked
    blocker TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(run_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_run_todos_run ON run_todos(run_id, seq);

-- 4. run_continuations: structured continuation packets for resume
CREATE TABLE IF NOT EXISTS run_continuations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,

    compact_summary TEXT NOT NULL DEFAULT '',
    pending_todos JSONB NOT NULL DEFAULT '[]',
    key_decisions JSONB NOT NULL DEFAULT '[]',
    changed_files JSONB NOT NULL DEFAULT '[]',
    blockers JSONB NOT NULL DEFAULT '[]',
    open_questions JSONB NOT NULL DEFAULT '[]',
    token_budget_used BIGINT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_run_continuations_run ON run_continuations(run_id);

-- 5. run_artifacts: outputs produced during a run
CREATE TABLE IF NOT EXISTS run_artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    step_id UUID REFERENCES run_steps(id) ON DELETE SET NULL,

    artifact_type TEXT NOT NULL DEFAULT 'text', -- text | file | diff | image
    name TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL DEFAULT 'text/plain',

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_run_artifacts_run ON run_artifacts(run_id);

-- 6. run_handoffs: delegation records between runs
CREATE TABLE IF NOT EXISTS run_handoffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    target_run_id UUID REFERENCES runs(id) ON DELETE SET NULL,

    handoff_type TEXT NOT NULL DEFAULT 'delegate', -- delegate | escalate | chain
    target_team_id UUID,
    target_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    context_packet JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_run_handoffs_source ON run_handoffs(source_run_id);
CREATE INDEX IF NOT EXISTS idx_run_handoffs_target ON run_handoffs(target_run_id) WHERE target_run_id IS NOT NULL;

-- 7. Upgrade outbox_messages for retry + dead-letter
ALTER TABLE outbox_messages ADD COLUMN IF NOT EXISTS retry_count INT DEFAULT 0;
ALTER TABLE outbox_messages ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE outbox_messages ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE outbox_messages ADD COLUMN IF NOT EXISTS dead_lettered_at TIMESTAMPTZ;
ALTER TABLE outbox_messages ADD COLUMN IF NOT EXISTS dead_letter_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_outbox_next_attempt
    ON outbox_messages(next_attempt_at)
    WHERE dead_lettered_at IS NULL AND processed_at IS NULL;

-- 8. Team columns deferred to team migration (team table created later)
