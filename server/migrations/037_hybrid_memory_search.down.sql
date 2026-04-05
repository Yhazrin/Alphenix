DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'agent_memory') THEN
    DROP TRIGGER IF EXISTS trg_agent_memory_tsv ON agent_memory;
    DROP INDEX IF EXISTS idx_agent_memory_tsv;
    ALTER TABLE agent_memory DROP COLUMN IF EXISTS tsv_content;
  END IF;
END $$;

DROP FUNCTION IF EXISTS agent_memory_tsv_trigger();
