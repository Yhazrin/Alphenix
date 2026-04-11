-- Add tsvector column for BM25 full-text search on agent memory.
ALTER TABLE agent_memory ADD COLUMN IF NOT EXISTS tsv_content tsvector;
CREATE INDEX IF NOT EXISTS idx_agent_memory_tsv ON agent_memory USING GIN(tsv_content);
UPDATE agent_memory SET tsv_content = to_tsvector('english', content);

CREATE OR REPLACE FUNCTION agent_memory_tsv_trigger() RETURNS trigger AS $func$
BEGIN
    NEW.tsv_content := to_tsvector('english', NEW.content);
    RETURN NEW;
END $func$ LANGUAGE plpgsql;

CREATE TRIGGER trg_agent_memory_tsv
    BEFORE INSERT OR UPDATE OF content ON agent_memory
    FOR EACH ROW EXECUTE FUNCTION agent_memory_tsv_trigger();
