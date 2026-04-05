-- Revert: delete all workspace_repo rows that came from the JSONB migration.
-- We can't perfectly distinguish migrated vs manually-created rows, so this
-- clears all repos. The original JSONB data is still in workspace.repos.
DELETE FROM workspace_repo;
