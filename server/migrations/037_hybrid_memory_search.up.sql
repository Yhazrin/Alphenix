-- Add tsvector column for BM25 full-text search on agent memory.
ALTER TABLE agent_memory ADD COLUMN tsv_content tsvector;

-- GIN index for fast tsvector lookups.
CREATE INDEX idx_agent_memory_tsv ON agent_memory USING GIN(tsv_content);

-- Backfill existing rows.
UPDATE agent_memory SET tsv_content = to_tsvector('english', content);

-- Auto-populate trigger: keeps tsv_content in sync on INSERT/UPDATE of content.
CREATE OR REPLACE FUNCTION agent_memory_tsv_trigger() RETURNS trigger AS $$
BEGIN
    NEW.tsv_content := to_tsvector('english', NEW.content);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_agent_memory_tsv
    BEFORE INSERT OR UPDATE OF content ON agent_memory
    FOR EACH ROW EXECUTE FUNCTION agent_memory_tsv_trigger();
