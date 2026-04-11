-- Rollback Run Orchestrator tables

DROP TABLE IF EXISTS run_handoffs;
DROP TABLE IF EXISTS run_artifacts;
DROP TABLE IF EXISTS run_continuations;
DROP TABLE IF EXISTS run_todos;
DROP TABLE IF EXISTS run_steps;
DROP TABLE IF EXISTS runs;

-- Revert outbox_messages columns
ALTER TABLE outbox_messages DROP COLUMN IF EXISTS retry_count;
ALTER TABLE outbox_messages DROP COLUMN IF EXISTS last_error;
ALTER TABLE outbox_messages DROP COLUMN IF EXISTS next_attempt_at;
ALTER TABLE outbox_messages DROP COLUMN IF EXISTS dead_lettered_at;
ALTER TABLE outbox_messages DROP COLUMN IF EXISTS dead_letter_reason;

DROP INDEX IF EXISTS idx_outbox_next_attempt;

-- Team columns deferred to team migration
