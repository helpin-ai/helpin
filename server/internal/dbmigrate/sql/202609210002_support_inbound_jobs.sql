CREATE TABLE IF NOT EXISTS support_inbound_jobs (
 id uuid PRIMARY KEY, kind text NOT NULL, workspace_id uuid, conversation_id uuid, message_id uuid,
 payload text NOT NULL, status text NOT NULL, attempts integer NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL, lease_token text NOT NULL DEFAULT '', last_error text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_support_inbound_due ON support_inbound_jobs(kind,status,available_at);
CREATE INDEX IF NOT EXISTS idx_support_inbound_conversation ON support_inbound_jobs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_support_inbound_workspace ON support_inbound_jobs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_support_inbound_message ON support_inbound_jobs(message_id);
ALTER TABLE support_attachments ADD COLUMN IF NOT EXISTS processing_status text NOT NULL DEFAULT '';
ALTER TABLE support_attachments ADD COLUMN IF NOT EXISTS processing_error text NOT NULL DEFAULT '';

ALTER TABLE support_attachments ADD COLUMN IF NOT EXISTS content_id text NOT NULL DEFAULT '';

-- Clear pending payloads alongside explicit conversation deletion. Keep receipt
-- tombstones to prevent a late provider retry from resurrecting the same mail.
CREATE OR REPLACE FUNCTION helpin_clear_inbound_conversation_jobs() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
 UPDATE support_inbound_jobs SET status='completed',payload='',lease_token='',last_error='',updated_at=now()
 WHERE conversation_id=OLD.id AND workspace_id=OLD.workspace_id;
 RETURN OLD;
END;
$$;
DROP TRIGGER IF EXISTS clear_inbound_conversation_jobs ON support_conversations;
CREATE TRIGGER clear_inbound_conversation_jobs BEFORE DELETE ON support_conversations
FOR EACH ROW EXECUTE FUNCTION helpin_clear_inbound_conversation_jobs();

CREATE OR REPLACE FUNCTION helpin_guard_inbound_job_privacy() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE archived_at timestamptz;
BEGIN
 -- Lease/status updates do not introduce new data. Avoid taking a conversation
 -- lock while holding a job lock during normal claims and retries.
 IF TG_OP = 'UPDATE' THEN
  IF NEW.payload = OLD.payload AND NEW.conversation_id IS NOT DISTINCT FROM OLD.conversation_id
   AND NEW.workspace_id IS NOT DISTINCT FROM OLD.workspace_id THEN
   RETURN NEW;
  END IF;
 END IF;
 IF NEW.payload <> '' AND NEW.conversation_id IS NOT NULL THEN
  SELECT anonymized_at INTO archived_at FROM support_conversations
  WHERE id=NEW.conversation_id AND workspace_id=NEW.workspace_id FOR UPDATE;
  IF archived_at IS NOT NULL THEN
   NEW.payload=''; NEW.status='completed'; NEW.lease_token=''; NEW.last_error='';
  END IF;
 END IF;
 RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS guard_inbound_job_privacy ON support_inbound_jobs;
CREATE TRIGGER guard_inbound_job_privacy BEFORE INSERT OR UPDATE ON support_inbound_jobs
FOR EACH ROW EXECUTE FUNCTION helpin_guard_inbound_job_privacy();
