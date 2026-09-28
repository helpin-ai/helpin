-- dbmigrate:no-transaction

-- Portal eligibility matches contacts by lower(trim(email)), and queries must use
-- this exact expression to use the index.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_crm_contacts_workspace_normalized_email
    ON crm_contacts (workspace_id, lower(trim(email)));

-- Portal settings summarize the few contacts with an access decision.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_crm_contacts_workspace_portal_access
    ON crm_contacts (workspace_id, portal_access)
    WHERE portal_access IS NOT NULL;
