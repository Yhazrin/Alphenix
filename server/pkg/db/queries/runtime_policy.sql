-- Runtime Assignment Policy queries

-- name: ListRuntimePolicies :many
SELECT * FROM runtime_assignment_policy
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetRuntimePolicy :one
SELECT * FROM runtime_assignment_policy
WHERE id = $1 AND workspace_id = $2;

-- name: GetRuntimePolicyByAgent :one
SELECT * FROM runtime_assignment_policy
WHERE agent_id = $1 AND workspace_id = $2;

-- name: CreateRuntimePolicy :one
INSERT INTO runtime_assignment_policy (
    workspace_id, agent_id, team_id,
    required_tags, forbidden_tags,
    preferred_runtime_ids, fallback_runtime_ids,
    max_queue_depth, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateRuntimePolicy :one
UPDATE runtime_assignment_policy SET
    required_tags = COALESCE(sqlc.narg('required_tags'), required_tags),
    forbidden_tags = COALESCE(sqlc.narg('forbidden_tags'), forbidden_tags),
    preferred_runtime_ids = COALESCE(sqlc.narg('preferred_runtime_ids'), preferred_runtime_ids),
    fallback_runtime_ids = COALESCE(sqlc.narg('fallback_runtime_ids'), fallback_runtime_ids),
    max_queue_depth = COALESCE(sqlc.narg('max_queue_depth'), max_queue_depth),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteRuntimePolicy :exec
DELETE FROM runtime_assignment_policy
WHERE id = $1 AND workspace_id = $2;

-- name: ListRuntimePoliciesByTeam :many
SELECT * FROM runtime_assignment_policy
WHERE team_id = $1
ORDER BY created_at ASC;
