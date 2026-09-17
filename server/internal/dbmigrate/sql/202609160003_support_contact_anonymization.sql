-- Retain support text and reporting history while irreversibly unlinking identity.
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS anonymized_at timestamptz;

-- Serialize late message/identity writes with anonymization. Application checks
-- improve errors, but the DB boundary also covers in-flight workers and old saves.
CREATE OR REPLACE FUNCTION helpin_guard_anonymized_support() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE archived_at timestamptz;
BEGIN
  IF TG_TABLE_NAME = 'support_conversations' THEN
    -- Read receipts may advance without restoring identity or reopening replies.
    IF OLD.anonymized_at IS NOT NULL AND
       (to_jsonb(NEW) - ARRAY['updated_at', 'team_last_seen_at', 'contact_last_seen_at', 'support_state_version'])
       IS DISTINCT FROM
       (to_jsonb(OLD) - ARRAY['updated_at', 'team_last_seen_at', 'contact_last_seen_at', 'support_state_version']) THEN
      RAISE EXCEPTION 'anonymized conversation is read-only' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
  END IF;
  IF TG_TABLE_NAME = 'support_widget_sessions' AND TG_OP = 'UPDATE' THEN
    IF OLD.revoked_at IS NOT NULL AND OLD.session_token LIKE 'revoked:%' THEN
      RAISE EXCEPTION 'anonymized visitor session is revoked' USING ERRCODE = 'P0001';
    END IF;
  END IF;
  SELECT anonymized_at INTO archived_at FROM support_conversations
    WHERE id = NEW.conversation_id AND workspace_id = NEW.workspace_id FOR UPDATE;
  IF archived_at IS NOT NULL THEN
    RAISE EXCEPTION 'anonymized conversation is read-only' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS guard_anonymized_support ON support_conversations;
CREATE TRIGGER guard_anonymized_support BEFORE UPDATE ON support_conversations
FOR EACH ROW EXECUTE FUNCTION helpin_guard_anonymized_support();

DO $$
DECLARE tbl text;
BEGIN
  FOREACH tbl IN ARRAY ARRAY['support_messages', 'support_email_logs', 'support_email_webhook_events', 'support_widget_sessions', 'support_attachments', 'support_events', 'support_conversation_triage', 'support_conversation_triage_events', 'support_ai_follow_ups']
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS guard_anonymized_support ON %I', tbl);
    EXECUTE format('CREATE TRIGGER guard_anonymized_support BEFORE INSERT OR UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION helpin_guard_anonymized_support()', tbl);
  END LOOP;
END;
$$;
