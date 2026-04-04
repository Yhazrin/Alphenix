-- name: CreateTeam :one
INSERT INTO team (workspace_id, name, description, avatar_url, lead_agent_id, created_by)
VALUES ($1, $2, sqlc.narg('description'), sqlc.narg('avatar_url'), sqlc.narg('lead_agent_id'), $3)
RETURNING *;

-- name: GetTeam :one
SELECT * FROM team WHERE id = $1;

-- name: ListTeams :many
SELECT * FROM team
WHERE workspace_id = $1 AND archived_at IS NULL
ORDER BY created_at DESC;

-- name: ListArchivedTeams :many
SELECT * FROM team
WHERE workspace_id = $1 AND archived_at IS NOT NULL
ORDER BY archived_at DESC;

-- name: UpdateTeam :one
UPDATE team
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    lead_agent_id = COALESCE(sqlc.narg('lead_agent_id'), lead_agent_id),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveTeam :one
UPDATE team
SET archived_at = now(), archived_by = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RestoreTeam :one
UPDATE team
SET archived_at = NULL, archived_by = NULL, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTeam :exec
DELETE FROM team WHERE id = $1;

-- name: AddTeamMember :one
INSERT INTO team_member (team_id, agent_id, role)
VALUES ($1, $2, sqlc.narg('role'))
ON CONFLICT (team_id, agent_id) DO UPDATE SET role = EXCLUDED.role
RETURNING *;

-- name: RemoveTeamMember :exec
DELETE FROM team_member WHERE team_id = $1 AND agent_id = $2;

-- name: ListTeamMembers :many
SELECT * FROM team_member WHERE team_id = $1 ORDER BY joined_at ASC;

-- name: ListTeamMembersByWorkspace :many
SELECT tm.* FROM team_member tm
JOIN team t ON tm.team_id = t.id
WHERE t.workspace_id = $1 AND t.archived_at IS NULL
ORDER BY t.name, tm.joined_at;

-- name: UpdateTeamMemberRole :one
UPDATE team_member
SET role = $3
WHERE team_id = $1 AND agent_id = $2
RETURNING *;

-- name: GetTeamMember :one
SELECT * FROM team_member WHERE team_id = $1 AND agent_id = $2;

-- name: UpdateTeamLead :one
UPDATE team
SET lead_agent_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateTeamTask :one
INSERT INTO team_task_queue (team_id, issue_id, assigned_by, priority)
VALUES ($1, $2, $3, sqlc.narg('priority'))
RETURNING *;

-- name: ListPendingTeamTasks :many
SELECT * FROM team_task_queue
WHERE team_id = $1 AND status = 'pending'
ORDER BY priority DESC, created_at ASC;

-- name: GetTeamTask :one
SELECT * FROM team_task_queue WHERE id = $1;

-- name: UpdateTeamTaskStatus :one
UPDATE team_task_queue
SET status = $2, delegated_to_agent_id = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListTeamTasksByIssue :many
SELECT * FROM team_task_queue WHERE issue_id = $1 ORDER BY created_at DESC;
