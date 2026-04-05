-- Revert: remove step_type and call_id columns.
ALTER TABLE run_steps DROP COLUMN IF EXISTS step_type;
ALTER TABLE run_steps DROP COLUMN IF EXISTS call_id;
