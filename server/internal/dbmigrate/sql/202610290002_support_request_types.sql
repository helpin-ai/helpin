-- Request types: what a customer asks for on the portal, with the
-- conversation fields that kind of request carries.

CREATE TABLE IF NOT EXISTS support_request_types (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    -- [{"key","required","customer_visible","help_text"}] in form order.
    fields jsonb NOT NULL DEFAULT '[]',
    default_priority text,
    mailbox_id uuid,
    enabled boolean NOT NULL,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_request_types_workspace_name
    ON support_request_types (workspace_id, lower(name));

-- No foreign key: types are disabled, not deleted, and a deleted type
-- leaves its conversations untyped by the application.
ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS request_type_id uuid;

CREATE INDEX IF NOT EXISTS idx_support_conversations_request_type
    ON support_conversations (workspace_id, request_type_id)
    WHERE request_type_id IS NOT NULL;

ALTER TABLE support_sla_policies
    ADD COLUMN IF NOT EXISTS request_type_ids text[] NOT NULL DEFAULT '{}';

-- A conversation's request type can select a different policy.
DROP TRIGGER IF EXISTS support_conversation_sla_updated ON support_conversations;
CREATE TRIGGER support_conversation_sla_updated
AFTER UPDATE OF status, priority, mailbox_id, channel, snoozed_until, resolved_at, merged_into_id,
    flow_state, ai_state, human_takeover, ai_escalated_at, customer_requested_human_at, ai_paused_at,
    crm_company_id, request_type_id
ON support_conversations
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status
    OR OLD.priority IS DISTINCT FROM NEW.priority
    OR OLD.mailbox_id IS DISTINCT FROM NEW.mailbox_id
    OR OLD.channel IS DISTINCT FROM NEW.channel
    OR OLD.snoozed_until IS DISTINCT FROM NEW.snoozed_until
    OR OLD.resolved_at IS DISTINCT FROM NEW.resolved_at
    OR OLD.merged_into_id IS DISTINCT FROM NEW.merged_into_id
    OR OLD.flow_state IS DISTINCT FROM NEW.flow_state
    OR OLD.ai_state IS DISTINCT FROM NEW.ai_state
    OR OLD.human_takeover IS DISTINCT FROM NEW.human_takeover
    OR OLD.ai_escalated_at IS DISTINCT FROM NEW.ai_escalated_at
    OR OLD.customer_requested_human_at IS DISTINCT FROM NEW.customer_requested_human_at
    OR OLD.ai_paused_at IS DISTINCT FROM NEW.ai_paused_at
    OR OLD.crm_company_id IS DISTINCT FROM NEW.crm_company_id
    OR OLD.request_type_id IS DISTINCT FROM NEW.request_type_id)
EXECUTE FUNCTION support_conversation_sla_changed();
