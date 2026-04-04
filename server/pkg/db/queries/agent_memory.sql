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

-- name: ListRecentWorkspaceMemory :many
-- Recent memories across all agents in a workspace (no embedding required).
SELECT *, 0.0 AS similarity FROM agent_memory
WHERE workspace_id = $1
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC
LIMIT $2;

-- name: DeleteAgentMemory :exec
DELETE FROM agent_memory
WHERE id = $1;

-- name: DeleteExpiredMemory :exec
DELETE FROM agent_memory
WHERE expires_at IS NOT NULL AND expires_at < now();

-- NOTE: BM25 queries are implemented manually in generated/agent_memory.sql.go
-- because sqlc v1.30.0 cannot parse to_tsquery syntax. Uncomment after sqlc upgrade.

-- -- name: SearchAgentMemoryBM25 :many
-- -- BM25 full-text search for a specific agent.
-- SELECT am.id, am.workspace_id, am.agent_id, am.content, am.embedding, am.metadata, am.created_at, am.expires_at,
--        ts_rank(am.tsv_content, q) AS bm25_score
-- FROM agent_memory am, to_tsquery('english', sqlc.arg(search_query)) q
-- WHERE am.agent_id = sqlc.arg(agent_id)
--   AND am.tsv_content @@ q
--   AND (am.expires_at IS NULL OR am.expires_at > now())
-- ORDER BY ts_rank(am.tsv_content, q) DESC
-- LIMIT sqlc.arg(limit_count);

-- -- name: SearchWorkspaceMemoryBM25 :many
-- -- BM25 full-text search across all agents in a workspace.
-- SELECT am.id, am.workspace_id, am.agent_id, am.content, am.embedding, am.metadata, am.created_at, am.expires_at,
--        ts_rank(am.tsv_content, q) AS bm25_score
-- FROM agent_memory am, to_tsquery('english', sqlc.arg(search_query)) q
-- WHERE am.workspace_id = sqlc.arg(workspace_id)
--   AND am.tsv_content @@ q
--   AND (am.expires_at IS NULL OR am.expires_at > now())
-- ORDER BY ts_rank(am.tsv_content, q) DESC
-- LIMIT sqlc.arg(limit_count);

