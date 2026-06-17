ALTER TABLE support_widget_sessions
  ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMPTZ;

UPDATE support_widget_sessions
SET last_active_at = COALESCE(updated_at, created_at, now())
WHERE last_active_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_support_widget_sessions_workspace_anon_active
  ON support_widget_sessions (workspace_id, anonymous_id, last_active_at DESC)
  WHERE last_active_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_widget_sessions_workspace_active
  ON support_widget_sessions (workspace_id, last_active_at DESC)
  WHERE last_active_at IS NOT NULL;
