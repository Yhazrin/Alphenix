ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_channel_id_fkey;
ALTER TABLE issue DROP COLUMN IF EXISTS channel_id;

DROP TABLE IF EXISTS channel_participant;
DROP TABLE IF EXISTS channel;
