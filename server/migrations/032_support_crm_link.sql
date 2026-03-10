-- 032: Link support tickets to CRM contacts
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS crm_contact_id UUID;
CREATE INDEX IF NOT EXISTS idx_support_tickets_crm_contact ON support_tickets(crm_contact_id) WHERE crm_contact_id IS NOT NULL;
