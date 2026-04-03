-- name: CreateTaskCheckpoint :one
INSERT INTO task_checkpoint (task_id, workspace_id, label, state, files_changed)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTaskCheckpoint :one
SELECT * FROM task_checkpoint
WHERE id = $1;

-- name: GetLatestCheckpoint :one
SELECT * FROM task_checkpoint
WHERE task_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListTaskCheckpoints :many
SELECT * FROM task_checkpoint
WHERE task_id = $1
ORDER BY created_at DESC;

-- name: DeleteTaskCheckpoints :exec
DELETE FROM task_checkpoint
WHERE task_id = $1;
