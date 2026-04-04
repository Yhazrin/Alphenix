DROP TRIGGER IF EXISTS trg_agent_memory_tsv ON agent_memory;
DROP FUNCTION IF EXISTS agent_memory_tsv_trigger();
DROP INDEX IF EXISTS idx_agent_memory_tsv;
ALTER TABLE agent_memory DROP COLUMN IF EXISTS tsv_content;
