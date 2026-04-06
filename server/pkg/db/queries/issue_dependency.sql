-- name: CreateIssueDependency :one
INSERT INTO issue_dependency (issue_id, depends_on_issue_id, type)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetIssueDependencies :many
SELECT * FROM issue_dependency
WHERE issue_id = $1;

-- name: GetIssueDependents :many
SELECT * FROM issue_dependency
WHERE depends_on_issue_id = $1;

-- name: DeleteIssueDependency :exec
DELETE FROM issue_dependency
WHERE issue_id = $1 AND depends_on_issue_id = $2;

-- name: DeleteAllDependenciesForIssue :exec
DELETE FROM issue_dependency
WHERE issue_id = $1 OR depends_on_issue_id = $1;

-- name: ListReadySubIssues :many
-- Returns sub-issues whose hard dependencies are all satisfied (done or cancelled).
SELECT i.* FROM issue i
WHERE i.parent_issue_id = $1
  AND i.status IN ('backlog', 'todo')
  AND NOT EXISTS (
    SELECT 1 FROM issue_dependency d
    JOIN issue dep ON dep.id = d.depends_on_issue_id
    WHERE d.issue_id = i.id
      AND d.type = 'blocked_by'
      AND dep.status NOT IN ('done', 'cancelled')
  )
ORDER BY i.position ASC, i.created_at ASC;
