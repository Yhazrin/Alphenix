-- name: ListWorkspaceRepos :many
SELECT * FROM workspace_repo WHERE workspace_id = $1 ORDER BY name;

-- name: GetWorkspaceRepo :one
SELECT * FROM workspace_repo WHERE id = $1;

-- name: GetDefaultWorkspaceRepo :one
SELECT * FROM workspace_repo WHERE workspace_id = $1 AND is_default = true;

-- name: CreateWorkspaceRepo :one
INSERT INTO workspace_repo (workspace_id, name, url, default_branch, description, is_default, config)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateWorkspaceRepo :one
UPDATE workspace_repo SET
    name = COALESCE(sqlc.narg('name'), name),
    url = COALESCE(sqlc.narg('url'), url),
    default_branch = COALESCE(sqlc.narg('default_branch'), default_branch),
    description = COALESCE(sqlc.narg('description'), description),
    is_default = COALESCE(sqlc.narg('is_default'), is_default),
    config = COALESCE(sqlc.narg('config'), config),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteWorkspaceRepo :exec
DELETE FROM workspace_repo WHERE id = $1;

-- name: ListIssuesByRepoID :many
SELECT * FROM issue WHERE repo_id = $1 ORDER BY position ASC, created_at DESC;
