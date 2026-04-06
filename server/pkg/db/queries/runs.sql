-- name: CreateRun :one
INSERT INTO runs (
    workspace_id, issue_id, task_id, agent_id, parent_run_id, team_id,
    phase, status, system_prompt, model_name, permission_mode
) VALUES (
    $1, $2, sqlc.narg(task_id), $3, sqlc.narg(parent_run_id), sqlc.narg(team_id),
    $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetRun :one
SELECT * FROM runs
WHERE id = $1;

-- name: GetRunForUpdate :one
SELECT * FROM runs
WHERE id = $1
FOR UPDATE;

-- name: ListRunsByIssue :many
SELECT * FROM runs
WHERE issue_id = $1
ORDER BY created_at DESC;

-- name: ListRunsByWorkspace :many
SELECT * FROM runs
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActiveRunsByAgent :many
SELECT * FROM runs
WHERE agent_id = $1 AND status = 'active'
ORDER BY created_at ASC;

-- name: ListActiveRunsByWorkspace :many
SELECT * FROM runs
WHERE workspace_id = $1 AND status = 'active'
ORDER BY created_at ASC;

-- name: UpdateRunPhase :one
UPDATE runs SET
    phase = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRunStatus :one
UPDATE runs SET
    status = $2,
    phase = COALESCE(sqlc.narg(phase), phase),
    updated_at = now(),
    completed_at = CASE WHEN $2 IN ('completed', 'failed', 'cancelled') THEN now() ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: UpdateRunTokens :one
UPDATE runs SET
    input_tokens = input_tokens + $2,
    output_tokens = output_tokens + $3,
    estimated_cost_usd = estimated_cost_usd + $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: StartRun :one
UPDATE runs SET
    status = 'active',
    phase = 'executing',
    started_at = now(),
    updated_at = now()
WHERE id = $1 AND phase = 'pending'
RETURNING *;

-- name: CompleteRun :one
UPDATE runs SET
    status = 'completed',
    phase = 'completed',
    completed_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: FailRun :one
UPDATE runs SET
    status = 'failed',
    phase = 'failed',
    completed_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: CancelRun :one
UPDATE runs SET
    status = 'cancelled',
    phase = 'cancelled',
    completed_at = now(),
    updated_at = now()
WHERE id = $1 AND status NOT IN ('completed','failed','cancelled')
RETURNING *;

-- name: DeleteRun :exec
DELETE FROM runs
WHERE id = $1;
