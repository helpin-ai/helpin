-- Simplify support conversation lifecycle statuses to:
-- open, waiting_on_customer, resolved, spam

UPDATE support_conversations
SET
  status = CASE
    WHEN status = 'in_progress' THEN 'open'
    WHEN status = 'waiting' THEN 'waiting_on_customer'
    WHEN status = 'closed' THEN 'resolved'
    ELSE status
  END,
  resolved_at = CASE
    WHEN status = 'closed' THEN COALESCE(resolved_at, closed_at, updated_at, created_at)
    ELSE resolved_at
  END
WHERE status IN ('in_progress', 'waiting', 'closed');
