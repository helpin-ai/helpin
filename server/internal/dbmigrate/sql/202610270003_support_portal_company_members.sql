-- Company portal membership is an explicit support admin decision. CRM
-- contact-company associations are also created automatically, including
-- from unverified Messenger identity claims, so they never grant access to a
-- company's portal requests. A request is shared only when both the reader
-- and the requester are members of the request's company.
CREATE TABLE IF NOT EXISTS support_portal_company_members (
    workspace_id UUID NOT NULL,
    crm_company_id UUID NOT NULL REFERENCES crm_companies (id) ON DELETE CASCADE,
    crm_contact_id UUID NOT NULL REFERENCES crm_contacts (id) ON DELETE CASCADE,
    added_by_user_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (crm_company_id, crm_contact_id)
);
CREATE INDEX IF NOT EXISTS idx_support_portal_company_members_contact
    ON support_portal_company_members (workspace_id, crm_contact_id);

-- Accept both dispatch modes again: during a rolling deploy an older API pod
-- may still queue a customer-facing turn, which must not fail the customer's
-- request. Such a turn can only ever save a private suggestion.
ALTER TABLE support_portal_ai_dispatches DROP CONSTRAINT IF EXISTS chk_support_portal_ai_dispatch_mode;
ALTER TABLE support_portal_ai_dispatches ADD CONSTRAINT chk_support_portal_ai_dispatch_mode
    CHECK (mode_at_enqueue IN ('internal_note', 'ai_first')) NOT VALID;
ALTER TABLE support_portal_ai_dispatches VALIDATE CONSTRAINT chk_support_portal_ai_dispatch_mode;
