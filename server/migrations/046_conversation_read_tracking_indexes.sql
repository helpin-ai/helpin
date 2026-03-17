-- Message query indexes for unread count computation
CREATE INDEX IF NOT EXISTS idx_support_messages_conversation_created_at
  ON support_messages(conversation_id, created_at);

CREATE INDEX IF NOT EXISTS idx_support_messages_conversation_public_replies
  ON support_messages(conversation_id, created_at)
  WHERE is_internal = false AND message_type = 'reply';
