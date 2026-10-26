-- Conversation imports from other help desks, resumable and idempotent on
-- the source's conversation ID.

CREATE TABLE IF NOT EXISTS support_import_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    source text NOT NULL,
    status text NOT NULL,
    since timestamptz NOT NULL,
    mailbox_id uuid,
    total integer NOT NULL DEFAULT 0,
    imported integer NOT NULL DEFAULT 0,
    skipped integer NOT NULL DEFAULT 0,
    failed integer NOT NULL DEFAULT 0,
    failures jsonb NOT NULL DEFAULT '[]',
    cursor text NOT NULL DEFAULT '',
    error text NOT NULL DEFAULT '',
    payload_encrypted text NOT NULL,
    locked_until timestamptz,
    started_by uuid NOT NULL,
    completed_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_support_import_jobs_workspace_id ON support_import_jobs (workspace_id);
CREATE INDEX IF NOT EXISTS idx_support_import_jobs_status ON support_import_jobs (status);

CREATE TABLE IF NOT EXISTS support_import_records (
    workspace_id uuid NOT NULL,
    source text NOT NULL,
    external_id text NOT NULL,
    conversation_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, source, external_id)
);

-- Imported history is not news: no webhook events for imported
-- conversations or the messages copied with them. Later live activity on an
-- imported conversation still sends events.
CREATE OR REPLACE FUNCTION webhook_conversation_inserted()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.merged_into_id IS NULL AND NEW.anonymized_at IS NULL AND NEW.created_at > NOW() - INTERVAL '1 hour'
        AND NEW.source IS DISTINCT FROM 'intercom' THEN
        PERFORM webhook_enqueue_event(NEW.workspace_id, 'conversation.created', NEW.id, NULL, '{}'::jsonb, NULL);
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS webhook_message_inserted ON support_messages;
CREATE TRIGGER webhook_message_inserted
AFTER INSERT ON support_messages
FOR EACH ROW
WHEN (NEW.message_type = 'reply' AND NOT NEW.is_internal AND NEW.system_event_type IS NULL
    AND NEW.sender_type IN ('customer', 'user', 'ai', 'agent') AND NEW.deleted_at IS NULL
    AND NEW.created_at > NOW() - INTERVAL '1 hour'
    AND COALESCE(NEW.metadata->>'imported_from', '') = '')
EXECUTE FUNCTION webhook_message_inserted();
