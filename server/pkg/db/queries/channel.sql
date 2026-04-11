-- name: CreateChannel :one
INSERT INTO channel (workspace_id, name, slug, description, is_default, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetChannel :one
SELECT * FROM channel WHERE id = $1;

-- name: GetChannelInWorkspace :one
SELECT * FROM channel
WHERE id = $1 AND workspace_id = $2 AND archived_at IS NULL;

-- name: GetDefaultChannelByWorkspace :one
SELECT * FROM channel
WHERE workspace_id = $1 AND is_default = true AND archived_at IS NULL
LIMIT 1;

-- name: GetChannelBySlugInWorkspace :one
SELECT * FROM channel
WHERE workspace_id = $1 AND slug = $2 AND archived_at IS NULL;

-- name: ListChannelsByWorkspace :many
SELECT * FROM channel
WHERE workspace_id = $1 AND archived_at IS NULL
ORDER BY is_default DESC, name ASC;

-- name: UpdateChannel :one
UPDATE channel SET
    name = COALESCE(sqlc.narg('name'), name),
    description = sqlc.narg('description'),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ArchiveChannel :one
UPDATE channel SET archived_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND is_default = false
RETURNING *;

-- name: AddChannelParticipant :exec
INSERT INTO channel_participant (channel_id, participant_type, participant_id)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RemoveChannelParticipant :exec
DELETE FROM channel_participant
WHERE channel_id = $1 AND participant_type = $2 AND participant_id = $3;

-- name: ListChannelParticipants :many
SELECT participant_type, participant_id, created_at
FROM channel_participant
WHERE channel_id = $1
ORDER BY participant_type, created_at;

-- name: SeedChannelParticipantsFromWorkspace :exec
INSERT INTO channel_participant (channel_id, participant_type, participant_id)
SELECT sqlc.arg('channel_id')::uuid, 'member', m.user_id FROM member m WHERE m.workspace_id = sqlc.arg('workspace_id')::uuid
UNION ALL
SELECT sqlc.arg('channel_id')::uuid, 'agent', a.id FROM agent a WHERE a.workspace_id = sqlc.arg('workspace_id')::uuid AND a.archived_at IS NULL
UNION ALL
SELECT sqlc.arg('channel_id')::uuid, 'team', t.id FROM team t WHERE t.workspace_id = sqlc.arg('workspace_id')::uuid AND t.archived_at IS NULL
ON CONFLICT DO NOTHING;
