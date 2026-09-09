ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS delayed_team_reply_sent_for timestamptz;
CREATE INDEX IF NOT EXISTS idx_support_delayed_team_reply ON support_conversations (ai_escalated_at) WHERE ai_state = 'escalated' AND status = 'open';

-- Cancellation survives reopening and stale full-row saves cannot erase delivery.
CREATE OR REPLACE FUNCTION preserve_delayed_team_reply_marker() RETURNS trigger AS $$
BEGIN
 NEW.delayed_team_reply_sent_for := GREATEST(OLD.delayed_team_reply_sent_for, NEW.delayed_team_reply_sent_for);
 IF NEW.status IN ('resolved', 'closed', 'spam') THEN
  NEW.delayed_team_reply_sent_for := GREATEST(NEW.delayed_team_reply_sent_for, NEW.ai_escalated_at);
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS preserve_delayed_team_reply_marker ON support_conversations;
CREATE TRIGGER preserve_delayed_team_reply_marker BEFORE UPDATE ON support_conversations
FOR EACH ROW EXECUTE FUNCTION preserve_delayed_team_reply_marker();
