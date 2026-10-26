-- Imported history stays out of live AI work: copying old conversations
-- must not classify them for tags or detect and translate their languages.
-- Later live messages on an imported conversation are handled as usual.

CREATE OR REPLACE FUNCTION support_enqueue_tagging() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.sender_type = 'customer' AND NOT NEW.is_internal AND NEW.message_type = 'reply'
        AND NEW.deleted_at IS NULL AND btrim(NEW.content) <> ''
        AND COALESCE(NEW.metadata->>'imported_from', '') = '' THEN
        INSERT INTO support_tag_jobs(workspace_id, conversation_id, message_id)
        VALUES (NEW.workspace_id, NEW.conversation_id, NEW.id) ON CONFLICT DO NOTHING;
    END IF;
    RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION support_enqueue_live_translate() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE policy support_translation_conversations; target text;
BEGIN
 IF NEW.sender_type <> 'customer' OR NEW.is_internal OR NEW.message_type <> 'reply' OR btrim(NEW.content)=''
    OR COALESCE(NEW.metadata->>'imported_from','') <> '' THEN RETURN NEW; END IF;
 SELECT * INTO policy FROM support_translation_conversations WHERE workspace_id=NEW.workspace_id AND conversation_id=NEW.conversation_id FOR UPDATE;
 SELECT COALESCE(settings::jsonb->>'default_agent_language','en') INTO target FROM support_widget_installations WHERE workspace_id=NEW.workspace_id;
 INSERT INTO support_live_messages(workspace_id,conversation_id,message_id,enabled,revision,target_language,source_hash)
 VALUES(NEW.workspace_id,NEW.conversation_id,NEW.id,policy.translation_mode='on',policy.revision,COALESCE(NULLIF(target,''),'en'),encode(digest(NEW.content,'sha256'),'hex')) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;

-- Drop work already queued for imported messages.
DELETE FROM support_tag_jobs j
USING support_messages m
WHERE j.message_id = m.id AND j.status = 'pending'
    AND COALESCE(m.metadata->>'imported_from', '') <> '';

DELETE FROM support_live_messages j
USING support_messages m
WHERE j.message_id = m.id AND j.status = 'queued'
    AND COALESCE(m.metadata->>'imported_from', '') <> '';

-- Import workers write progress only while they hold the job's lease, and
-- back off when the source keeps failing.
ALTER TABLE support_import_jobs
    ADD COLUMN IF NOT EXISTS lease_token text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS attempts integer NOT NULL DEFAULT 0;

-- Finished imports no longer need the source's access token.
UPDATE support_import_jobs SET payload_encrypted = ''
WHERE status IN ('done', 'failed', 'cancelled') AND payload_encrypted <> '';

UPDATE docs_import_jobs SET payload_encrypted = NULL
WHERE source = 'intercom' AND status IN ('done', 'failed', 'interrupted') AND payload_encrypted IS NOT NULL;

-- Deliveries logged before conversation_id existed are purged with the
-- conversation's other data too.
UPDATE webhook_deliveries
SET conversation_id = (payload->'data'->'conversation'->>'id')::uuid
WHERE conversation_id IS NULL
    AND payload->'data'->'conversation'->>'id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
