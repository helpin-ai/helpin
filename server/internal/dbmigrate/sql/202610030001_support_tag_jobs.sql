-- Identifier-only work; no historical replay and no message content in the queue.
CREATE TABLE IF NOT EXISTS support_tag_jobs (
    message_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    available_at timestamptz NOT NULL DEFAULT now(),
    lease_token text NOT NULL DEFAULT '',
    lease_until timestamptz,
    result_code text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, conversation_id, message_id)
        REFERENCES support_messages(workspace_id, conversation_id, id) ON DELETE CASCADE,
    CHECK ((status = 'processing' AND lease_token <> '' AND lease_until IS NOT NULL)
        OR (status <> 'processing' AND lease_token = '' AND lease_until IS NULL))
);
CREATE INDEX IF NOT EXISTS support_tag_jobs_pending ON support_tag_jobs(available_at, message_id)
    WHERE status IN ('pending', 'processing');

CREATE OR REPLACE FUNCTION support_enqueue_tagging() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.sender_type = 'customer' AND NOT NEW.is_internal AND NEW.message_type = 'reply'
        AND NEW.deleted_at IS NULL AND btrim(NEW.content) <> '' THEN
        INSERT INTO support_tag_jobs(workspace_id, conversation_id, message_id)
        VALUES (NEW.workspace_id, NEW.conversation_id, NEW.id) ON CONFLICT DO NOTHING;
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS support_enqueue_tagging ON support_messages;
CREATE TRIGGER support_enqueue_tagging AFTER INSERT ON support_messages
    FOR EACH ROW EXECUTE FUNCTION support_enqueue_tagging();

CREATE OR REPLACE FUNCTION support_clear_tagging() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.anonymized_at IS NOT NULL THEN
        DELETE FROM support_tag_jobs WHERE workspace_id = NEW.workspace_id AND conversation_id = NEW.id;
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS support_clear_tagging ON support_conversations;
CREATE TRIGGER support_clear_tagging AFTER UPDATE OF anonymized_at ON support_conversations
    FOR EACH ROW EXECUTE FUNCTION support_clear_tagging();
