-- Stored generated columns are computed after BEFORE triggers. NEW.search_vector
-- is null here even when no identity changed; comparing it with OLD rejects valid
-- projection updates and read receipts. Its input columns remain guarded.
CREATE OR REPLACE FUNCTION helpin_guard_anonymized_support() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE archived_at timestamptz;
BEGIN
  IF TG_TABLE_NAME = 'support_conversations' THEN
    -- Read receipts may advance without restoring identity or reopening replies.
    IF OLD.anonymized_at IS NOT NULL AND
       (to_jsonb(NEW) - ARRAY['updated_at', 'team_last_seen_at', 'contact_last_seen_at', 'support_state_version', 'search_vector'])
       IS DISTINCT FROM
       (to_jsonb(OLD) - ARRAY['updated_at', 'team_last_seen_at', 'contact_last_seen_at', 'support_state_version', 'search_vector']) THEN
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
