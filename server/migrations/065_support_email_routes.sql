CREATE TABLE IF NOT EXISTS support_email_routes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    mailbox_id uuid NULL REFERENCES support_mailboxes(id) ON DELETE CASCADE,
    route_key text NOT NULL,
    inbound_address text NOT NULL,
    source_address text NULL,
    provider_type text NOT NULL DEFAULT 'forwarding',
    active boolean NOT NULL DEFAULT true,
    last_inbound_at timestamptz NULL,
    created_by_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_email_routes_route_key
    ON support_email_routes(route_key);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_email_routes_inbound_address
    ON support_email_routes(inbound_address);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ser_active_shared_route
    ON support_email_routes(workspace_id)
    WHERE mailbox_id IS NULL AND active = true;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ser_active_mailbox_route
    ON support_email_routes(mailbox_id)
    WHERE mailbox_id IS NOT NULL AND active = true;

ALTER TABLE support_email_logs
    ADD COLUMN IF NOT EXISTS email_route_id uuid NULL REFERENCES support_email_routes(id) ON DELETE SET NULL;

ALTER TABLE support_email_logs
    ADD COLUMN IF NOT EXISTS recipient_address text NULL;

ALTER TABLE support_email_logs
    ADD COLUMN IF NOT EXISTS references_header text NULL;

CREATE INDEX IF NOT EXISTS idx_sel_rfc_message_id
    ON support_email_logs(workspace_id, rfc_message_id)
    WHERE rfc_message_id IS NOT NULL AND rfc_message_id <> '';
