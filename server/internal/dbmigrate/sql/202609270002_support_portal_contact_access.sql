-- Support admins grant or block customer portal access per CRM contact. NULL
-- means no decision: denied in approved-contacts mode, allowed otherwise.
ALTER TABLE crm_contacts ADD COLUMN IF NOT EXISTS portal_access TEXT;
ALTER TABLE crm_contacts DROP CONSTRAINT IF EXISTS chk_crm_contacts_portal_access;
ALTER TABLE crm_contacts ADD CONSTRAINT chk_crm_contacts_portal_access
    CHECK (portal_access IS NULL OR portal_access IN ('allowed', 'blocked')) NOT VALID;
ALTER TABLE crm_contacts VALIDATE CONSTRAINT chk_crm_contacts_portal_access;

-- The contact a portal identity last signed in as, for attribution. Sessions
-- carry their own contact for authorization.
ALTER TABLE support_portal_identities ADD COLUMN IF NOT EXISTS crm_contact_id UUID;

-- The contact a session was issued for. Identities are shared per email, so
-- approved-contacts authorization checks the session's own contact.
ALTER TABLE support_portal_sessions ADD COLUMN IF NOT EXISTS crm_contact_id UUID;
