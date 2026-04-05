-- Migrate existing workspace.repos JSONB data into workspace_repo table.
-- This is a separate migration so schema and data failures can be rolled back independently.

INSERT INTO workspace_repo (workspace_id, name, url, description)
SELECT
    w.id,
    COALESCE(
        regexp_replace(repo->>'url', '.*[/]([^/]+?)(?:\.git)?$', '\1'),
        'repo-' || row_number() OVER (PARTITION BY w.id ORDER BY repo)
    ),
    repo->>'url',
    repo->>'description'
FROM workspace w, jsonb_array_elements(w.repos) AS repo
WHERE jsonb_array_length(w.repos) > 0;
