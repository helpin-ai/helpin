-- Company admins in the customer portal: a role per company membership
-- (admin manages the company's people), per-company invite switches, and an
-- audit log of portal-side changes to a company's people.

-- A membership role applies in that company only and wins over the
-- contact's role and the company default. Admin exists only here.
ALTER TABLE support_portal_company_members ADD COLUMN IF NOT EXISTS role text;
ALTER TABLE support_portal_company_members DROP CONSTRAINT IF EXISTS chk_support_portal_company_members_role;
ALTER TABLE support_portal_company_members ADD CONSTRAINT chk_support_portal_company_members_role
    CHECK (role IS NULL OR role IN ('requester', 'member', 'admin')) NOT VALID;
ALTER TABLE support_portal_company_members VALIDATE CONSTRAINT chk_support_portal_company_members_role;

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
