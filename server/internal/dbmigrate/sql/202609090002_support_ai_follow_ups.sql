CREATE TABLE IF NOT EXISTS support_ai_follow_ups (
 id uuid PRIMARY KEY,
 workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 conversation_id uuid NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
 source_message_id uuid NOT NULL,
 run_id uuid NOT NULL UNIQUE,
 status text NOT NULL CHECK (status IN ('scheduled','assessing','waiting','resolved','skipped','cancelled','failed','handoff')),
 reason text NOT NULL DEFAULT '',
 due_at timestamptz NOT NULL,
 started_at timestamptz,
 close_hours integer NOT NULL CHECK (close_hours BETWEEN 1 AND 720),
 sent_message_id uuid,
 sent_at timestamptz,
 close_at timestamptz,
 lease_until timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(workspace_id, conversation_id, source_message_id)
);
CREATE INDEX IF NOT EXISTS idx_support_ai_follow_up_due ON support_ai_follow_ups(due_at) WHERE status IN ('scheduled','assessing','waiting');
CREATE INDEX IF NOT EXISTS idx_support_ai_follow_up_conversation ON support_ai_follow_ups(workspace_id, conversation_id, created_at DESC);

-- Lock order is always conversation -> episode. Public-message projection
-- updates the conversation in the same transaction as message insertion.
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
  AND sent_message_id IS DISTINCT FROM NEW.last_public_message_id;
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS cancel_support_ai_follow_up ON support_conversations;
CREATE TRIGGER cancel_support_ai_follow_up AFTER UPDATE ON support_conversations
FOR EACH ROW EXECUTE FUNCTION cancel_support_ai_follow_up();

-- Turning policy off cancels work permanently, including a quick off/on toggle.
CREATE OR REPLACE FUNCTION cancel_disabled_support_ai_follow_ups() RETURNS trigger AS $$
BEGIN
 IF jsonb_build_array(NEW.active, NEW.settings->'ai_enabled', NEW.settings->'ai_agent_id', NEW.settings->'ai_response_mode', NEW.settings->'ai_auto_resolve_timeout', NEW.settings->'ai_follow_up_enabled', NEW.settings->'ai_follow_up_delay_hours', NEW.settings->'ai_follow_up_close_hours', NEW.settings->'ai_follow_up_max_per_conversation')
 IS DISTINCT FROM jsonb_build_array(OLD.active, OLD.settings->'ai_enabled', OLD.settings->'ai_agent_id', OLD.settings->'ai_response_mode', OLD.settings->'ai_auto_resolve_timeout', OLD.settings->'ai_follow_up_enabled', OLD.settings->'ai_follow_up_delay_hours', OLD.settings->'ai_follow_up_close_hours', OLD.settings->'ai_follow_up_max_per_conversation') THEN
  UPDATE support_ai_follow_ups SET status = 'cancelled', reason = 'settings_changed', updated_at = now()
  WHERE workspace_id = NEW.workspace_id AND status IN ('scheduled','assessing','waiting');
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS cancel_disabled_support_ai_follow_ups ON support_widget_installations;
CREATE TRIGGER cancel_disabled_support_ai_follow_ups AFTER UPDATE OF settings, active ON support_widget_installations
FOR EACH ROW EXECUTE FUNCTION cancel_disabled_support_ai_follow_ups();

-- Editing/removing the source or sent follow-up invalidates its assessment.
CREATE OR REPLACE FUNCTION invalidate_edited_support_ai_follow_up() RETURNS trigger AS $$
BEGIN
 PERFORM 1 FROM support_conversations WHERE id = OLD.conversation_id FOR UPDATE;
 UPDATE support_ai_follow_ups SET status='cancelled', reason='message_changed', updated_at=now()
 WHERE workspace_id=OLD.workspace_id AND conversation_id=OLD.conversation_id
 AND (source_message_id=OLD.id OR sent_message_id=OLD.id)
 AND status IN ('scheduled','assessing','waiting');
 RETURN NULL;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS invalidate_edited_support_ai_follow_up ON support_messages;
CREATE TRIGGER invalidate_edited_support_ai_follow_up AFTER UPDATE OF content, is_internal, message_type, deleted_at OR DELETE ON support_messages
FOR EACH ROW EXECUTE FUNCTION invalidate_edited_support_ai_follow_up();

CREATE INDEX IF NOT EXISTS idx_support_ai_follow_up_candidates ON support_conversations(workspace_id,last_public_message_at,id)
WHERE flow_state='ai_handling' AND ai_state='pending' AND last_public_sender_type='ai' AND status IN ('open','waiting_on_customer');
