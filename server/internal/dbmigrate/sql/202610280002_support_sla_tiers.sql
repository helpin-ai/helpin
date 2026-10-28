-- SLA tiers: companies carry a tier, and SLA policies can match the
-- conversation's company tier or specific companies.

CREATE TABLE IF NOT EXISTS support_sla_tiers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_sla_tiers_workspace_name
    ON support_sla_tiers (workspace_id, lower(name));

-- The tier change time lets a new tier count from the change for
-- conversations that had no SLA yet.
ALTER TABLE crm_companies
    ADD COLUMN IF NOT EXISTS support_sla_tier_id uuid REFERENCES support_sla_tiers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS support_sla_tier_changed_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_crm_companies_support_sla_tier
    ON crm_companies (workspace_id, support_sla_tier_id)
    WHERE support_sla_tier_id IS NOT NULL;

ALTER TABLE support_sla_policies
    ADD COLUMN IF NOT EXISTS company_tier_ids text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS company_ids text[] NOT NULL DEFAULT '{}';

-- The company and tier a record was matched with. A change of either counts
-- new targets from the change; empty means the record predates tiers.
ALTER TABLE support_conversation_slas
    ADD COLUMN IF NOT EXISTS company_key text NOT NULL DEFAULT '';

-- A conversation's company can select a different policy.
DROP TRIGGER IF EXISTS support_conversation_sla_updated ON support_conversations;
CREATE TRIGGER support_conversation_sla_updated
AFTER UPDATE OF status, priority, mailbox_id, channel, snoozed_until, resolved_at, merged_into_id,
    flow_state, ai_state, human_takeover, ai_escalated_at, customer_requested_human_at, ai_paused_at,
    crm_company_id
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
    OR OLD.crm_company_id IS DISTINCT FROM NEW.crm_company_id)
EXECUTE FUNCTION support_conversation_sla_changed();
