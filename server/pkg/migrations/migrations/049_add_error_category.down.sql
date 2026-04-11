-- Rollback Migration 049: Remove error classification fields

-- Drop indexes first
DROP INDEX IF EXISTS idx_run_steps_error_category;
DROP INDEX IF EXISTS idx_runs_error_category;

-- Remove columns from run_steps
ALTER TABLE run_steps DROP COLUMN IF EXISTS error_category;
ALTER TABLE run_steps DROP COLUMN IF EXISTS error_subcategory;
ALTER TABLE run_steps DROP COLUMN IF EXISTS error_severity;
ALTER TABLE run_steps DROP COLUMN IF EXISTS exclusion_reason;

-- Remove columns from runs
ALTER TABLE runs DROP COLUMN IF EXISTS error_category;
ALTER TABLE runs DROP COLUMN IF EXISTS error_severity;
