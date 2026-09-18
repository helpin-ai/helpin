CREATE UNIQUE INDEX IF NOT EXISTS idx_support_conversation_translation_scope ON support_conversations(workspace_id,id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_message_translation_scope ON support_messages(workspace_id,conversation_id,id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_jev_translation_scope ON jev_decision_attempts(workspace_id,id);
ALTER TABLE jev_decision_attempts DROP CONSTRAINT IF EXISTS jev_decision_attempts_feature_check;
ALTER TABLE jev_decision_attempts ADD CONSTRAINT jev_decision_attempts_feature_check CHECK (feature IN ('meeting_routing','coverage_classification','coverage_topic_matching','automation_condition','answer_evidence','translation_review'));

CREATE TABLE IF NOT EXISTS support_translation_preferences (
 workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 reading_language text NOT NULL DEFAULT 'en', auto_translate_incoming boolean NOT NULL DEFAULT true, auto_translate_outgoing boolean NOT NULL DEFAULT true,
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(workspace_id,user_id)
);
CREATE TABLE IF NOT EXISTS support_translation_conversations (
 workspace_id uuid NOT NULL, conversation_id uuid NOT NULL,
 customer_language text NOT NULL DEFAULT '', translation_mode text NOT NULL DEFAULT 'inherit' CHECK (translation_mode IN ('inherit','on','off')),
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(workspace_id,conversation_id),
 FOREIGN KEY(workspace_id,conversation_id) REFERENCES support_conversations(workspace_id,id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS support_translations (
 id uuid PRIMARY KEY, workspace_id uuid NOT NULL, conversation_id uuid NOT NULL,
 purpose text NOT NULL CHECK (purpose IN ('message_display','outgoing_reply')),
 source_message_id uuid, created_by_user_id uuid REFERENCES users(id) ON DELETE CASCADE, sent_message_id uuid,
 source_text text NOT NULL, source_hash text NOT NULL, source_language text NOT NULL DEFAULT '', target_language text NOT NULL,
 send_key text NOT NULL DEFAULT '',
 translated_text text NOT NULL DEFAULT '', cache_key text NOT NULL,
 provider text NOT NULL DEFAULT '', model text NOT NULL DEFAULT '', pipeline_version text NOT NULL,
 attempts integer NOT NULL DEFAULT 1,
 status text NOT NULL CHECK (status IN ('pending','ready','failed','expired')),
 review_status text NOT NULL DEFAULT 'not_requested' CHECK (review_status IN ('not_requested','pending','accepted','needs_review','unavailable','shadow_accepted','shadow_rejected')),
 jev_assessment_id uuid, sent_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL, sent_at timestamptz,
 error_code text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz,
 UNIQUE(workspace_id,cache_key), UNIQUE(workspace_id,sent_message_id),
 FOREIGN KEY(workspace_id,conversation_id) REFERENCES support_conversations(workspace_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,conversation_id,source_message_id) REFERENCES support_messages(workspace_id,conversation_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,conversation_id,sent_message_id) REFERENCES support_messages(workspace_id,conversation_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,jev_assessment_id) REFERENCES jev_decision_attempts(workspace_id,id),
 CHECK ((purpose='message_display' AND source_message_id IS NOT NULL AND sent_message_id IS NULL) OR
        (purpose='outgoing_reply' AND source_message_id IS NULL AND created_by_user_id IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_support_translations_expiry ON support_translations(expires_at) WHERE sent_message_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_support_translations_conversation ON support_translations(workspace_id,conversation_id);

-- Privacy invalidation applies to SQL writers as well as application writers.
CREATE OR REPLACE FUNCTION invalidate_support_message_translations() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.deleted_at IS NOT NULL OR NEW.content IS DISTINCT FROM OLD.content THEN
  DELETE FROM support_translations WHERE workspace_id=NEW.workspace_id AND (source_message_id=NEW.id OR sent_message_id=NEW.id);
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS support_message_translation_invalidation ON support_messages;
CREATE TRIGGER support_message_translation_invalidation AFTER UPDATE OF content,deleted_at ON support_messages FOR EACH ROW EXECUTE FUNCTION invalidate_support_message_translations();
CREATE OR REPLACE FUNCTION invalidate_support_conversation_translations() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.anonymized_at IS NOT NULL THEN
  DELETE FROM support_translations WHERE workspace_id=NEW.workspace_id AND conversation_id=NEW.id;
  DELETE FROM support_translation_conversations WHERE workspace_id=NEW.workspace_id AND conversation_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS support_conversation_translation_invalidation ON support_conversations;
CREATE TRIGGER support_conversation_translation_invalidation AFTER UPDATE OF anonymized_at ON support_conversations FOR EACH ROW EXECUTE FUNCTION invalidate_support_conversation_translations();

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_translation_send_key ON support_translations(workspace_id,conversation_id,created_by_user_id,send_key) WHERE purpose='outgoing_reply';
