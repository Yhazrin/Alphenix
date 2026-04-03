-- name: CreateAgentMessage :one
INSERT INTO agent_message (workspace_id, from_agent_id, to_agent_id, task_id, content, metadata, message_type, reply_to_id)
VALUES ($1, $2, $3, sqlc.narg(task_id), $4, $5, $6, sqlc.narg(reply_to_id))
RETURNING *;

-- name: GetAgentMessage :one
SELECT * FROM agent_message
WHERE id = $1;

-- name: ListAgentMessagesByTask :many
SELECT * FROM agent_message
WHERE task_id = $1
ORDER BY created_at ASC;

-- name: ListAgentMessagesForAgent :many
SELECT * FROM agent_message
WHERE to_agent_id = $1 AND created_at > $2
ORDER BY created_at ASC;

-- name: ListAgentMessagesForWorkspace :many
SELECT * FROM agent_message
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CountUnreadAgentMessages :one
SELECT count(*) FROM agent_message
WHERE to_agent_id = $1 AND read_at IS NULL;

-- name: MarkAgentMessageRead :exec
UPDATE agent_message SET read_at = now()
WHERE id = $1 AND to_agent_id = $2;

-- name: MarkAllAgentMessagesRead :exec
UPDATE agent_message SET read_at = now()
WHERE to_agent_id = $1 AND read_at IS NULL;
