-- name: ListMCPServersByWorkspace :many
SELECT * FROM mcp_servers
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetMCPServer :one
SELECT * FROM mcp_servers
WHERE id = $1;

-- name: GetMCPServerInWorkspace :one
SELECT * FROM mcp_servers
WHERE id = $1 AND workspace_id = $2;

-- name: CreateMCPServer :one
INSERT INTO mcp_servers (
    workspace_id, name, description, transport, url, command, args, env, config
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateMCPServer :one
UPDATE mcp_servers SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    transport = COALESCE(sqlc.narg('transport'), transport),
    url = COALESCE(sqlc.narg('url'), url),
    command = COALESCE(sqlc.narg('command'), command),
    args = COALESCE(sqlc.narg('args'), args),
    env = COALESCE(sqlc.narg('env'), env),
    status = COALESCE(sqlc.narg('status'), status),
    config = COALESCE(sqlc.narg('config'), config),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMCPServer :exec
DELETE FROM mcp_servers
WHERE id = $1;

-- name: UpdateMCPServerStatus :one
UPDATE mcp_servers SET
    status = $2,
    last_connected_at = COALESCE(sqlc.narg('last_connected_at'), last_connected_at),
    last_error = COALESCE(sqlc.narg('last_error'), last_error),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListActiveMCPServers :many
SELECT * FROM mcp_servers
WHERE status = 'connected'
ORDER BY created_at ASC;
