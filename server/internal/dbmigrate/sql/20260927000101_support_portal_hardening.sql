-- An anonymous intake request stays hidden from the portal until the address
-- owner exchanges the confirmation link issued for that conversation.
ALTER TABLE support_portal_magic_links ADD COLUMN IF NOT EXISTS conversation_id UUID;

-- Sign-in link throttling looks up recent links per address and per workspace.
CREATE INDEX IF NOT EXISTS idx_support_portal_magic_links_workspace_email_created
    ON support_portal_magic_links (workspace_id, email, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_portal_magic_links_workspace_created
    ON support_portal_magic_links (workspace_id, created_at DESC);

-- Request reconciliation runs at most once per session interval, not per read.
ALTER TABLE support_portal_sessions ADD COLUMN IF NOT EXISTS reconciled_at TIMESTAMPTZ;

-- Support conversations are permanently deleted; this column was never written.
DROP INDEX IF EXISTS idx_support_conversations_deleted_at;
ALTER TABLE support_conversations DROP COLUMN IF EXISTS deleted_at;
