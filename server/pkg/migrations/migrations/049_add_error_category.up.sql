-- Migration 049: Add error classification fields to runs and run_steps
-- Adds error_category, error_severity, error_subcategory, and exclusion_reason fields
-- to support Failure Taxonomy v0.2 specification

-- Add columns to run_steps
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS error_category VARCHAR(50);
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS error_subcategory VARCHAR(50);
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS error_severity VARCHAR(20);
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS exclusion_reason VARCHAR(255);

-- Add columns to runs
ALTER TABLE runs ADD COLUMN IF NOT EXISTS error_category VARCHAR(50);
ALTER TABLE runs ADD COLUMN IF NOT EXISTS error_severity VARCHAR(20);

-- Create indexes for common error queries
CREATE INDEX IF NOT EXISTS idx_run_steps_error_category ON run_steps(error_category) WHERE error_category IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_runs_error_category ON runs(error_category) WHERE error_category IS NOT NULL;

-- Fill existing data in run_steps based on existing error indicators
UPDATE run_steps
SET error_category = CASE
    WHEN step_type = 'error' AND tool_name = '' THEN 'AGENT_ERROR'
    WHEN is_error = true AND tool_name = '' AND step_type != 'error' THEN 'CODE_ERROR'
    WHEN is_error = true AND tool_name != '' THEN 'TOOL_ERROR'
    WHEN exclusion_reason ILIKE '%timeout%' THEN 'TIMEOUT'
    WHEN exclusion_reason ILIKE '%policy%' OR exclusion_reason ILIKE '%deny%' OR exclusion_reason ILIKE '%SQL Guard%' THEN 'POLICY_VIOLATION'
    WHEN exclusion_reason ILIKE '%human%' OR exclusion_reason ILIKE '%user%' OR exclusion_reason ILIKE '%confirm%' THEN 'HUMAN_INTERVENTION'
    ELSE NULL
END,
error_severity = CASE
    WHEN step_type = 'error' AND tool_name = '' THEN 'FATAL'
    WHEN is_error = true AND tool_name = '' AND step_type != 'error' THEN 'PERMANENT'
    WHEN is_error = true AND tool_name != '' THEN 'RECOVERABLE'
    WHEN exclusion_reason ILIKE '%timeout%' THEN 'TRANSIENT'
    WHEN exclusion_reason ILIKE '%policy%' OR exclusion_reason ILIKE '%deny%' OR exclusion_reason ILIKE '%SQL Guard%' THEN 'PERMANENT'
    WHEN exclusion_reason ILIKE '%human%' OR exclusion_reason ILIKE '%user%' OR exclusion_reason ILIKE '%confirm%' THEN 'TRANSIENT'
    ELSE NULL
END
WHERE is_error = true OR step_type = 'error';

-- Fill existing data in runs (from last error step)
UPDATE runs
SET error_category = (
    SELECT error_category
    FROM run_steps
    WHERE run_id = runs.id AND is_error = true
    ORDER BY seq DESC
    LIMIT 1
),
error_severity = (
    SELECT error_severity
    FROM run_steps
    WHERE run_id = runs.id AND is_error = true
    ORDER BY seq DESC
    LIMIT 1
)
WHERE phase IN ('failed', 'cancelled') AND error_category IS NULL;
