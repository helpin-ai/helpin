-- Production runs explicit migrations with auto-migration disabled.
ALTER TABLE crm_email_accounts ADD COLUMN IF NOT EXISTS signature text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS crm_email_templates (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    name text, subject text, body_html text,
    shared boolean, version bigint,
    created_at timestamptz, updated_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_crm_email_templates_workspace_id ON crm_email_templates (workspace_id);

CREATE TABLE IF NOT EXISTS crm_email_sequences (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    name text, status text, version bigint,
    steps jsonb, timezone text,
    start_hour bigint, end_hour bigint,
    weekdays boolean, include_signature boolean,
    entry_stage_id text, entry_account_id text,
    entry_after timestamptz, entry_cursor_id text, entry_error text,
    created_at timestamptz, updated_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_crm_email_sequences_workspace_id ON crm_email_sequences (workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_sequences_status ON crm_email_sequences (status);

CREATE TABLE IF NOT EXISTS crm_sequence_enrollments (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    sequence_id uuid NOT NULL,
    sequence_version bigint, sequence_name text,
    contact_id uuid, contact_name text,
    email text NOT NULL,
    deal_id text, account_id uuid, owner_id uuid,
    status text, step_index bigint, step_count bigint,
    steps jsonb, timezone text, start_hour bigint, end_hour bigint,
    weekdays boolean, next_at timestamptz,
    lease_until timestamptz, lease_token text,
    unsubscribe_token text, error text,
    created_at timestamptz, updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS crm_sequence_recipient ON crm_sequence_enrollments (sequence_id, email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_sequence_enrollments_unsubscribe_token ON crm_sequence_enrollments (unsubscribe_token);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_enrollments_workspace_id ON crm_sequence_enrollments (workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_enrollments_contact_id ON crm_sequence_enrollments (contact_id);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_enrollments_account_id ON crm_sequence_enrollments (account_id);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_enrollments_owner_id ON crm_sequence_enrollments (owner_id);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_enrollments_status ON crm_sequence_enrollments (status);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_enrollments_next_at ON crm_sequence_enrollments (next_at);

CREATE TABLE IF NOT EXISTS crm_sequence_deliveries (
    id uuid PRIMARY KEY,
    enrollment_id uuid NOT NULL,
    step_index bigint NOT NULL,
    workspace_id uuid, account_id uuid,
    kind text, status text, subject text, body_html text,
    result_id text, error text,
    created_at timestamptz, updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS crm_sequence_delivery_step ON crm_sequence_deliveries (enrollment_id, step_index);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_deliveries_workspace_id ON crm_sequence_deliveries (workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_sequence_deliveries_account_id ON crm_sequence_deliveries (account_id);

CREATE TABLE IF NOT EXISTS crm_email_suppressions (
    workspace_id uuid,
    email text,
    created_at timestamptz,
    PRIMARY KEY (workspace_id, email)
);
