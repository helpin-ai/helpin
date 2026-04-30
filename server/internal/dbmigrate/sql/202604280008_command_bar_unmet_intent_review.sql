ALTER TABLE command_bar_unmet_intents
  ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'open',
  ADD COLUMN IF NOT EXISTS review_notes text,
  ADD COLUMN IF NOT EXISTS reviewed_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_command_bar_unmet_intents_workspace_status_created
  ON command_bar_unmet_intents (workspace_id, status, created_at DESC);
