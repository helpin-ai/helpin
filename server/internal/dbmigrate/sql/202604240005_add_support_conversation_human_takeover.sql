ALTER TABLE support_conversations
  ADD COLUMN IF NOT EXISTS human_takeover boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_support_conversations_human_takeover
  ON support_conversations (human_takeover);

UPDATE support_conversations
SET human_takeover = true
WHERE opened_by_user_id IS NOT NULL
  AND human_takeover = false;
