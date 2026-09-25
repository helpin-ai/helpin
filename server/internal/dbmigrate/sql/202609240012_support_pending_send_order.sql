-- Limit conversation ordering checks to unfinished work, excluding sent history.
CREATE INDEX IF NOT EXISTS support_pending_sends_conversation_work
ON support_pending_sends (workspace_id, conversation_id, created_at, id)
WHERE status IN ('queued', 'preparing', 'translating', 'sending');
