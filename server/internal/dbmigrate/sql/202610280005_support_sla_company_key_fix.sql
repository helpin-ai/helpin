-- A conversation whose company is not in its workspace has no company for
-- SLA matching; give such records the same key evaluation computes, so the
-- next evaluation does not mistake it for a company change.
UPDATE support_conversation_slas AS sla
SET company_key = 'company:none'
FROM support_conversations AS c
WHERE c.id = sla.conversation_id
  AND c.crm_company_id IS NOT NULL
  AND sla.company_key = 'company:' || c.crm_company_id::text || '|tier:none'
  AND NOT EXISTS (
      SELECT 1 FROM crm_companies AS co
      WHERE co.id = c.crm_company_id AND co.workspace_id = c.workspace_id
  );
