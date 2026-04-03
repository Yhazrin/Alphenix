-- name: CreateTaskDependency :one
INSERT INTO task_dependency (workspace_id, task_id, depends_on_task_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTaskDependencies :many
SELECT * FROM task_dependency
WHERE task_id = $1;

-- name: GetTaskDependents :many
SELECT * FROM task_dependency
WHERE depends_on_task_id = $1;

-- name: ListReadyTasks :many
-- Returns tasks that have no unresolved dependencies (all deps are completed/cancelled/failed).
SELECT atq.* FROM agent_task_queue atq
WHERE atq.status = 'queued'
  AND atq.agent_id = $1
  AND NOT EXISTS (
    SELECT 1 FROM task_dependency td
    JOIN agent_task_queue dep ON td.depends_on_task_id = dep.id
    WHERE td.task_id = atq.id
      AND dep.status NOT IN ('completed', 'failed', 'cancelled')
  )
ORDER BY atq.priority DESC, atq.created_at ASC;

-- name: DeleteTaskDependency :exec
DELETE FROM task_dependency
WHERE task_id = $1 AND depends_on_task_id = $2;

-- name: DeleteAllDependenciesForTask :exec
DELETE FROM task_dependency
WHERE task_id = $1 OR depends_on_task_id = $1;
