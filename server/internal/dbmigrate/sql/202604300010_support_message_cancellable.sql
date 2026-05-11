-- Add soft-delete and email-fallback cancel-window columns to support_messages.
--
-- cancellable_until is set by EmailFallbackService.OnAgentReply to the same
-- fireAt it computes from the workspace's EmailFallbackDelaySecs setting,
-- and is NULL for messages that aren't subject to email fallback (visitor
-- inbound, AI-authored, internal notes, agent replies where fallback is off).
--
-- deleted_at is added if missing so the SupportMessage GORM model can use
-- gorm.DeletedAt for automatic soft-delete filtering on read paths
-- (ListByConversation, GetByID, GetByIDs).

ALTER TABLE support_messages
  ADD COLUMN IF NOT EXISTS deleted_at        timestamptz,
  ADD COLUMN IF NOT EXISTS cancellable_until timestamptz;

CREATE INDEX IF NOT EXISTS idx_support_messages_deleted_at
  ON support_messages(deleted_at)
  WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_messages_cancellable_until
  ON support_messages(cancellable_until)
  WHERE cancellable_until IS NOT NULL;
