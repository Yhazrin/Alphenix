-- Task Report queries for execution replay and reporting.

-- name: GetTaskReport :one
SELECT
    atq.id,
    atq.agent_id,
    a.name AS agent_name,
    atq.issue_id,
    i.title AS issue_title,
    atq.runtime_id,
    ar.name AS runtime_name,
    atq.status,
    atq.priority,
    atq.dispatched_at,
    atq.started_at,
    atq.completed_at,
    atq.result,
    atq.error,
    atq.created_at,
    atq.review_status,
    (SELECT count(*) FROM task_message tm WHERE tm.task_id = atq.id) AS message_count,
    (SELECT count(*) FROM task_checkpoint tc WHERE tc.task_id = atq.id) AS checkpoint_count
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
JOIN issue i ON i.id = atq.issue_id
LEFT JOIN agent_runtime ar ON ar.id = atq.runtime_id
WHERE atq.id = $1;

-- name: GetTaskTimelineMessages :many
SELECT
    'message' AS event_type,
    tm.id::text,
    tm.created_at AS timestamp,
    COALESCE(tm.type, '') AS title,
    COALESCE(tm.content, '') AS detail,
    NULL::jsonb AS meta
FROM task_message tm
WHERE tm.task_id = $1

UNION ALL

SELECT
    'checkpoint' AS event_type,
    tc.id::text,
    tc.created_at AS timestamp,
    'Checkpoint: ' || tc.label AS title,
    '' AS detail,
    tc.state AS meta
FROM task_checkpoint tc
WHERE tc.task_id = $1

UNION ALL

SELECT
    'review' AS event_type,
    tr.id::text,
    tr.created_at AS timestamp,
    'Review: ' || tr.verdict AS title,
    COALESCE(tr.feedback, '') AS detail,
    jsonb_build_object('score', tr.score, 'reviewer_type', tr.reviewer_type) AS meta
FROM task_review tr
WHERE tr.task_id = $1

ORDER BY timestamp ASC;
