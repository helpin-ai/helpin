ALTER TABLE support_email_routes
  ADD COLUMN IF NOT EXISTS confirmation_conversation_id UUID REFERENCES support_conversations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_support_email_routes_confirmation_conversation_id
  ON support_email_routes (confirmation_conversation_id);
