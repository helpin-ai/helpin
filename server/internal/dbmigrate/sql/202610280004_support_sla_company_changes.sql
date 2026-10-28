-- When a conversation's company changes, so a policy that starts matching
-- through the new company counts its targets from the change.
ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS crm_company_changed_at timestamptz;

CREATE OR REPLACE FUNCTION support_conversation_company_changed()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.crm_company_changed_at := NOW();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_conversation_company_changed ON support_conversations;
CREATE TRIGGER support_conversation_company_changed
BEFORE UPDATE OF crm_company_id ON support_conversations
FOR EACH ROW
WHEN (OLD.crm_company_id IS DISTINCT FROM NEW.crm_company_id)
EXECUTE FUNCTION support_conversation_company_changed();

-- Records written before SLA tiers take the company they have now, so a
-- later company or tier change counts from the change.
UPDATE support_conversation_slas AS sla
SET company_key = CASE
        WHEN c.crm_company_id IS NULL THEN 'company:none'
        ELSE 'company:' || c.crm_company_id::text || '|tier:' || COALESCE(co.support_sla_tier_id::text, 'none')
    END
FROM support_conversations AS c
LEFT JOIN crm_companies AS co ON co.id = c.crm_company_id AND co.workspace_id = c.workspace_id
WHERE c.id = sla.conversation_id AND sla.company_key = '';
