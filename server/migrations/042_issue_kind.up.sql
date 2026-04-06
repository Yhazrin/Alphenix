ALTER TABLE issue ADD COLUMN issue_kind TEXT NOT NULL DEFAULT 'task'
  CHECK (issue_kind IN ('goal', 'task'));
