-- Company accounts in the customer portal. A company's portal_default_role
-- turns on company features for its contacts: NULL (the default) keeps the
-- portal personal, 'requester' lets contacts see only their own requests, and
-- 'member' lets them see and reply to every request from the company.
ALTER TABLE crm_companies ADD COLUMN IF NOT EXISTS portal_default_role TEXT;
ALTER TABLE crm_companies DROP CONSTRAINT IF EXISTS chk_crm_companies_portal_default_role;
ALTER TABLE crm_companies ADD CONSTRAINT chk_crm_companies_portal_default_role
    CHECK (portal_default_role IS NULL OR portal_default_role IN ('requester', 'member')) NOT VALID;
ALTER TABLE crm_companies VALIDATE CONSTRAINT chk_crm_companies_portal_default_role;

-- A contact's role overrides its companies' default in either direction.
-- NULL follows the company. Signing in still needs portal_access.
ALTER TABLE crm_contacts ADD COLUMN IF NOT EXISTS portal_role TEXT;
ALTER TABLE crm_contacts DROP CONSTRAINT IF EXISTS chk_crm_contacts_portal_role;
ALTER TABLE crm_contacts ADD CONSTRAINT chk_crm_contacts_portal_role
    CHECK (portal_role IS NULL OR portal_role IN ('requester', 'member')) NOT VALID;
ALTER TABLE crm_contacts VALIDATE CONSTRAINT chk_crm_contacts_portal_role;

-- Company request lists filter conversations by company within a workspace.
CREATE INDEX IF NOT EXISTS idx_support_conversations_workspace_crm_company
    ON support_conversations (workspace_id, crm_company_id) WHERE crm_company_id IS NOT NULL;
