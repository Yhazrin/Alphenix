-- name: CreateAgentMemory :one
INSERT INTO agent_memory (workspace_id, agent_id, content, embedding, metadata, expires_at)
VALUES ($1, $2, $3, $4, $5, sqlc.narg(expires_at))
RETURNING *;

-- name: GetAgentMemory :one
SELECT * FROM agent_memory
WHERE id = $1;

-- name: SearchAgentMemory :many
-- Semantic search: find memories closest to the given embedding for a specific agent.
SELECT *, 1 - (embedding <=> $1) AS similarity
FROM agent_memory
WHERE agent_id = $2
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY embedding <=> $1
LIMIT $3;

-- name: SearchWorkspaceMemory :many
-- Semantic search across all agents in a workspace.
SELECT *, 1 - (embedding <=> $1) AS similarity
FROM agent_memory
WHERE workspace_id = $2
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY embedding <=> $1
LIMIT $3;

-- name: ListAgentMemory :many
SELECT * FROM agent_memory
WHERE agent_id = $1
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC
LIMIT $2;

-- name: DeleteAgentMemory :exec
DELETE FROM agent_memory
WHERE id = $1;

-- name: DeleteExpiredMemory :exec
DELETE FROM agent_memory
WHERE expires_at IS NOT NULL AND expires_at < now();
