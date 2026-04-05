-- Add step_type and call_id to run_steps for structured agent observability.
-- step_type: thinking | text | tool_use | tool_result | error
-- call_id: pairs tool_use with tool_result events (nullable for thinking/text).

ALTER TABLE run_steps ADD COLUMN step_type TEXT NOT NULL DEFAULT 'tool_use';
ALTER TABLE run_steps ADD COLUMN call_id TEXT;
