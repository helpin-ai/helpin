-- Company admins in the customer portal: a contact role that manages the
-- company's people, per-company invite switches, and an audit log of
-- portal-side changes to a company's people.

ALTER TABLE crm_contacts DROP CONSTRAINT IF EXISTS chk_crm_contacts_portal_role;
ALTER TABLE crm_contacts ADD CONSTRAINT chk_crm_contacts_portal_role
    CHECK (portal_role IS NULL OR portal_role IN ('requester', 'member', 'admin')) NOT VALID;
ALTER TABLE crm_contacts VALIDATE CONSTRAINT chk_crm_contacts_portal_role;

-- Company admins may invite colleagues only when support turns it on, and
-- only addresses on the company's domain unless widened.
ALTER TABLE crm_companies
    ADD COLUMN IF NOT EXISTS portal_admin_invites boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS portal_invite_any_domain boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS support_portal_company_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    crm_company_id uuid NOT NULL,
    -- Who acted: a company admin in the portal, or a teammate.
    actor_identity_id uuid,
    actor_user_id uuid,
    subject_contact_id uuid,
    subject_email text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    before_value text,
    after_value text,
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_portal_company_events_company
    ON support_portal_company_events (workspace_id, crm_company_id, occurred_at DESC);
