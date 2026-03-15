-- Widget sessions: add visitor identity, anonymous flag, metadata, revocation
ALTER TABLE support_widget_sessions
  ADD COLUMN IF NOT EXISTS anonymous_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS is_anonymous BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS user_agent TEXT,
  ADD COLUMN IF NOT EXISTS last_page_url TEXT,
  ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- Index for session cleanup cron
CREATE INDEX IF NOT EXISTS idx_widget_sessions_revoked
  ON support_widget_sessions(revoked_at)
  WHERE revoked_at IS NOT NULL;

-- Index for looking up sessions by anonymous_id
CREATE INDEX IF NOT EXISTS idx_widget_sessions_anonymous
  ON support_widget_sessions(workspace_id, anonymous_id);

-- Conversations: add visitor ownership for direct history queries
ALTER TABLE support_conversations
  ADD COLUMN IF NOT EXISTS anonymous_id TEXT;

CREATE INDEX IF NOT EXISTS idx_conversations_visitor_workspace
  ON support_conversations(workspace_id, anonymous_id)
  WHERE anonymous_id IS NOT NULL;
