-- Existing episodes retain their original single-reminder contract.
ALTER TABLE support_ai_follow_ups
 ADD COLUMN IF NOT EXISTS sequence_version integer NOT NULL DEFAULT 1,
 ADD COLUMN IF NOT EXISTS second_delay_hours integer NOT NULL DEFAULT 24,
 ADD COLUMN IF NOT EXISTS second_message_id uuid,
 ADD COLUMN IF NOT EXISTS second_sent_at timestamptz,
 ADD COLUMN IF NOT EXISTS closing_notice text NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS assessment_attempts integer NOT NULL DEFAULT 1;
ALTER TABLE support_ai_follow_ups ALTER COLUMN sequence_version SET DEFAULT 2;

CREATE OR REPLACE FUNCTION cancel_support_ai_follow_up() RETURNS trigger AS $$
BEGIN
 IF NEW.flow_state IS DISTINCT FROM OLD.flow_state
 OR NEW.ai_state IS DISTINCT FROM OLD.ai_state
 OR NEW.human_takeover IS DISTINCT FROM OLD.human_takeover
 OR NEW.assigned_agent_id IS DISTINCT FROM OLD.assigned_agent_id
 OR NEW.mailbox_id IS DISTINCT FROM OLD.mailbox_id
 OR NEW.assigned_user_id IS DISTINCT FROM OLD.assigned_user_id
 OR NEW.opened_by_user_id IS DISTINCT FROM OLD.opened_by_user_id
 OR NEW.customer_requested_human_at IS DISTINCT FROM OLD.customer_requested_human_at
 OR NEW.linked_task_id IS DISTINCT FROM OLD.linked_task_id
 OR NEW.email_unsubscribed IS DISTINCT FROM OLD.email_unsubscribed
 OR NEW.channel IS DISTINCT FROM OLD.channel
 OR NEW.status IS DISTINCT FROM OLD.status THEN
  UPDATE support_ai_follow_ups SET status='cancelled', reason='conversation_changed', updated_at=now()
  WHERE conversation_id=NEW.id AND workspace_id=NEW.workspace_id AND status IN ('scheduled','assessing','waiting');
 ELSIF NEW.last_public_message_id IS DISTINCT FROM OLD.last_public_message_id THEN
  UPDATE support_ai_follow_ups SET status='cancelled', reason='conversation_changed', updated_at=now()
  WHERE conversation_id=NEW.id AND workspace_id=NEW.workspace_id AND status IN ('scheduled','assessing','waiting')
  AND sent_message_id IS DISTINCT FROM NEW.last_public_message_id
  AND second_message_id IS DISTINCT FROM NEW.last_public_message_id;
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS cancel_support_ai_follow_up ON support_conversations;
CREATE TRIGGER cancel_support_ai_follow_up AFTER UPDATE ON support_conversations
FOR EACH ROW EXECUTE FUNCTION cancel_support_ai_follow_up();

-- Timing edits affect future sequences only. Disabling or changing the AI
-- ownership policy still cancels outstanding work, including quick off/on.
CREATE OR REPLACE FUNCTION cancel_disabled_support_ai_follow_ups() RETURNS trigger AS $$
BEGIN
 IF NOT NEW.active OR NOT coalesce((NEW.settings->>'ai_follow_up_enabled')::boolean, true)
 OR jsonb_build_array(NEW.settings->'ai_enabled', NEW.settings->'ai_agent_id', NEW.settings->'ai_response_mode')
 IS DISTINCT FROM jsonb_build_array(OLD.settings->'ai_enabled', OLD.settings->'ai_agent_id', OLD.settings->'ai_response_mode') THEN
  UPDATE support_ai_follow_ups SET status='cancelled', reason='settings_changed', updated_at=now()
  WHERE workspace_id=NEW.workspace_id AND status IN ('scheduled','assessing','waiting');
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION invalidate_edited_support_ai_follow_up() RETURNS trigger AS $$
BEGIN
 PERFORM 1 FROM support_conversations WHERE id=OLD.conversation_id FOR UPDATE;
 UPDATE support_ai_follow_ups SET status='cancelled',reason='message_changed',updated_at=now()
 WHERE workspace_id=OLD.workspace_id AND conversation_id=OLD.conversation_id
 AND (source_message_id=OLD.id OR sent_message_id=OLD.id OR second_message_id=OLD.id)
 AND status IN ('scheduled','assessing','waiting');
 RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Explicit user-approved rollout: turn on for existing workspaces. Preserve
-- custom first/closure timings, replacing the previous 48h closure default.
UPDATE support_widget_installations SET settings=coalesce(settings,'{}'::jsonb) || jsonb_build_object(
 'ai_follow_up_enabled',true,
 'ai_follow_up_delay_hours',coalesce(settings->'ai_follow_up_delay_hours','24'::jsonb),
 'ai_follow_up_second_delay_hours',coalesce(settings->'ai_follow_up_second_delay_hours','24'::jsonb),
 'ai_follow_up_close_hours',CASE WHEN settings->>'ai_follow_up_close_hours' IS NULL OR settings->>'ai_follow_up_close_hours'='48' THEN '1'::jsonb ELSE settings->'ai_follow_up_close_hours' END
);
