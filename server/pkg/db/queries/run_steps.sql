-- name: CreateRunStep :one
INSERT INTO run_steps (
    run_id, seq, tool_name, tool_input, tool_output, is_error, started_at, step_type, call_id
) VALUES (
    $1, $2, $3, sqlc.narg(tool_input), sqlc.narg(tool_output), $4, $5, $6, sqlc.narg(call_id)
)
RETURNING *;

-- name: GetRunStep :one
SELECT * FROM run_steps
WHERE id = $1;

-- name: CompleteRunStep :one
UPDATE run_steps SET
    tool_output = sqlc.narg(tool_output),
    is_error = $2,
    completed_at = now()
WHERE id = $1
RETURNING *;

-- name: ListRunSteps :many
SELECT * FROM run_steps
WHERE run_id = $1
ORDER BY seq ASC;

-- name: ListRunStepsLimit :many
SELECT * FROM run_steps
WHERE run_id = $1
ORDER BY seq DESC
LIMIT $2;

-- name: GetNextStepSeq :one
SELECT COALESCE(MAX(seq), 0) + 1 AS next_seq
FROM run_steps
WHERE run_id = $1;

-- name: CountRunSteps :one
SELECT COUNT(*) FROM run_steps
WHERE run_id = $1;

-- name: DeleteRunSteps :exec
DELETE FROM run_steps
WHERE run_id = $1;
