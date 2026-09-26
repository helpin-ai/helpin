ALTER TABLE support_translations ADD COLUMN IF NOT EXISTS policy_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE support_translation_conversations ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1;
-- Freeze existing policies; do not turn disabled/asymmetric workspaces on.
INSERT INTO support_translation_conversations(workspace_id,conversation_id,translation_mode,customer_language)
SELECT c.workspace_id,c.id,
 CASE WHEN COALESCE((i.settings::jsonb->>'translation_enabled')::boolean,true)
 AND COALESCE((i.settings::jsonb->>'translation_incoming_enabled')::boolean,true)
 AND COALESCE((i.settings::jsonb->>'translation_outgoing_enabled')::boolean,true) THEN 'on' ELSE 'off' END,
 COALESCE(i.settings::jsonb->>'translation_customer_language','')
FROM support_conversations c LEFT JOIN support_widget_installations i ON i.workspace_id=c.workspace_id
ON CONFLICT DO NOTHING;
UPDATE support_translation_conversations p SET translation_mode = CASE WHEN
 COALESCE((i.settings::jsonb->>'translation_enabled')::boolean,true)
 AND COALESCE((i.settings::jsonb->>'translation_incoming_enabled')::boolean,true)
 AND COALESCE((i.settings::jsonb->>'translation_outgoing_enabled')::boolean,true) THEN 'on' ELSE 'off' END
FROM support_conversations c LEFT JOIN support_widget_installations i ON i.workspace_id=c.workspace_id
WHERE p.workspace_id=c.workspace_id AND p.conversation_id=c.id AND p.translation_mode='inherit';
CREATE TABLE IF NOT EXISTS support_live_messages (
 workspace_id uuid NOT NULL, conversation_id uuid NOT NULL, message_id uuid PRIMARY KEY,
 enabled boolean NOT NULL, revision bigint NOT NULL, target_language text NOT NULL, source_hash text NOT NULL,
 status text NOT NULL DEFAULT 'queued', attempts integer NOT NULL DEFAULT 0,
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,conversation_id,message_id) REFERENCES support_messages(workspace_id,conversation_id,id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS support_live_messages_pending ON support_live_messages(status,updated_at);
CREATE OR REPLACE FUNCTION support_initialize_live_translate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO support_translation_conversations(workspace_id,conversation_id,translation_mode)
 SELECT NEW.workspace_id,NEW.id, CASE WHEN COALESCE((settings::jsonb->>'translation_enabled')::boolean,true) THEN 'on' ELSE 'off' END
 FROM support_widget_installations WHERE workspace_id=NEW.workspace_id ON CONFLICT DO NOTHING;
 INSERT INTO support_translation_conversations(workspace_id,conversation_id,translation_mode) VALUES(NEW.workspace_id,NEW.id,'on') ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS support_initialize_live_translate ON support_conversations;
CREATE TRIGGER support_initialize_live_translate AFTER INSERT ON support_conversations FOR EACH ROW EXECUTE FUNCTION support_initialize_live_translate();
CREATE OR REPLACE FUNCTION support_enqueue_live_translate() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE policy support_translation_conversations; target text;
BEGIN
 IF NEW.sender_type <> 'customer' OR NEW.is_internal OR NEW.message_type <> 'reply' OR btrim(NEW.content)='' THEN RETURN NEW; END IF;
 SELECT * INTO policy FROM support_translation_conversations WHERE workspace_id=NEW.workspace_id AND conversation_id=NEW.conversation_id FOR UPDATE;
 SELECT COALESCE(settings::jsonb->>'default_agent_language','en') INTO target FROM support_widget_installations WHERE workspace_id=NEW.workspace_id;
 INSERT INTO support_live_messages(workspace_id,conversation_id,message_id,enabled,revision,target_language,source_hash)
 VALUES(NEW.workspace_id,NEW.conversation_id,NEW.id,policy.translation_mode='on',policy.revision,COALESCE(NULLIF(target,''),'en'),encode(digest(NEW.content,'sha256'),'hex')) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS support_enqueue_live_translate ON support_messages;
CREATE TRIGGER support_enqueue_live_translate AFTER INSERT ON support_messages FOR EACH ROW EXECUTE FUNCTION support_enqueue_live_translate();
ALTER TABLE support_translations DROP CONSTRAINT IF EXISTS support_translations_purpose_check;
ALTER TABLE support_translations ADD CONSTRAINT support_translations_purpose_check CHECK(purpose IN ('message_display','manual_display','outgoing_reply','language_detection'));
CREATE TABLE IF NOT EXISTS support_pending_sends (
 id uuid PRIMARY KEY, workspace_id uuid NOT NULL, conversation_id uuid NOT NULL,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, request text NOT NULL, request_hash text NOT NULL, recipient_email text NOT NULL DEFAULT '', target_language text NOT NULL DEFAULT '',
 revision bigint NOT NULL, attempts integer NOT NULL DEFAULT 0, status text NOT NULL, failure text NOT NULL DEFAULT '', message_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,conversation_id) REFERENCES support_conversations(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS support_pending_sends_work ON support_pending_sends(status,updated_at);
-- Queued drafts contain customer data too; privacy erasure must cover them.
CREATE OR REPLACE FUNCTION support_clear_live_translate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.anonymized_at IS NOT NULL THEN
  DELETE FROM support_pending_sends WHERE workspace_id=NEW.workspace_id AND conversation_id=NEW.id;
  DELETE FROM support_live_messages WHERE workspace_id=NEW.workspace_id AND conversation_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS support_clear_live_translate ON support_conversations;
CREATE TRIGGER support_clear_live_translate AFTER UPDATE OF anonymized_at ON support_conversations FOR EACH ROW EXECUTE FUNCTION support_clear_live_translate();

ALTER TABLE support_translations DROP CONSTRAINT IF EXISTS support_translations_check;
ALTER TABLE support_translations ADD CONSTRAINT support_translations_check CHECK ((purpose IN ('message_display','manual_display','language_detection') AND source_message_id IS NOT NULL AND sent_message_id IS NULL) OR (purpose='outgoing_reply' AND source_message_id IS NULL AND created_by_user_id IS NOT NULL));
