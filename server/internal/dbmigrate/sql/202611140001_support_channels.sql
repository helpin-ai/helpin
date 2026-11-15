-- Provider channels (combined inbox phase 1). A connected account (a Telegram
-- bot first) feeds a mailbox; identities are a person per account; the outbox
-- carries replies out. Columns added to existing tables are nullable or have a
-- constant default, so the ALTERs are metadata-only.

CREATE TABLE IF NOT EXISTS support_channel_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    external_account_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    handle TEXT NOT NULL DEFAULT '',
    mailbox_id UUID NULL REFERENCES support_mailboxes(id) ON DELETE SET NULL,
    ai_replies_disabled BOOLEAN NOT NULL DEFAULT FALSE,
    token_encrypted TEXT NOT NULL,
    webhook_secret_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    auth_error_count INTEGER NOT NULL DEFAULT 0,
    last_error_code TEXT NULL,
    last_error_message TEXT NULL,
    last_event_at TIMESTAMPTZ NULL,
    connected_by UUID NULL,
    disabled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_channel_accounts_provider_check CHECK (provider IN ('telegram')),
    CONSTRAINT support_channel_accounts_status_check
        CHECK (status IN ('active', 'reauthorization_required', 'disabled')),
    CONSTRAINT support_channel_accounts_auth_error_count_check CHECK (auth_error_count >= 0)
);

-- A bot can only have one webhook, so it is live in at most one workspace.
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_channel_accounts_live
    ON support_channel_accounts (provider, external_account_id)
    WHERE disabled_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_support_channel_accounts_workspace
    ON support_channel_accounts (workspace_id, status);

CREATE TABLE IF NOT EXISTS support_channel_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    channel_account_id UUID NOT NULL REFERENCES support_channel_accounts(id),
    external_user_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    crm_contact_id UUID NULL REFERENCES crm_contacts(id) ON DELETE SET NULL,
    last_inbound_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_channel_identities_account_user_key UNIQUE (channel_account_id, external_user_id)
);

CREATE INDEX IF NOT EXISTS idx_support_channel_identities_contact
    ON support_channel_identities (workspace_id, crm_contact_id)
    WHERE crm_contact_id IS NOT NULL;

ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS channel_account_id UUID NULL;
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS channel_identity_id UUID NULL;
CREATE INDEX IF NOT EXISTS idx_support_conversations_channel_identity
    ON support_conversations (channel_identity_id, status)
    WHERE channel_identity_id IS NOT NULL;

ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS channel_account_id UUID NULL;
ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS external_message_key TEXT NULL;
ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS external_echo BOOLEAN NOT NULL DEFAULT FALSE;
-- The single dedupe point for provider retries and repeated webhooks.
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_messages_external_key
    ON support_messages (channel_account_id, external_message_key)
    WHERE channel_account_id IS NOT NULL AND external_message_key IS NOT NULL;

ALTER TABLE support_inbound_jobs ADD COLUMN IF NOT EXISTS channel_account_id UUID NULL;
-- sha256 of "<account>:<external user>", so privacy cleanup can find events
-- whose conversation does not exist yet.
ALTER TABLE support_inbound_jobs ADD COLUMN IF NOT EXISTS subject_key TEXT NULL;
CREATE INDEX IF NOT EXISTS idx_support_inbound_jobs_subject
    ON support_inbound_jobs (subject_key)
    WHERE subject_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS support_channel_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    channel_account_id UUID NOT NULL REFERENCES support_channel_accounts(id),
    channel_identity_id UUID NOT NULL REFERENCES support_channel_identities(id),
    conversation_id UUID NOT NULL,
    message_id UUID NOT NULL,
    semantic_key TEXT NOT NULL,
    dispatch_status TEXT NOT NULL DEFAULT 'pending',
    receipt_status TEXT NOT NULL DEFAULT 'none',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claim_token TEXT NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    external_message_key TEXT NULL,
    last_error_code TEXT NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_channel_outbox_semantic_key_key UNIQUE (semantic_key),
    CONSTRAINT support_channel_outbox_dispatch_check
        CHECK (dispatch_status IN ('pending', 'dispatching', 'accepted', 'rejected', 'uncertain', 'cancelled')),
    CONSTRAINT support_channel_outbox_receipt_check
        CHECK (receipt_status IN ('none', 'sent', 'delivered', 'read', 'failed')),
    CONSTRAINT support_channel_outbox_attempts_check CHECK (attempts >= 0)
);

CREATE INDEX IF NOT EXISTS idx_support_channel_outbox_due_work
    ON support_channel_outbox (dispatch_status, next_attempt_at, lease_expires_at)
    WHERE dispatch_status IN ('pending', 'dispatching');
-- Per-chat ordering: the claim looks for earlier undelivered rows per identity.
CREATE INDEX IF NOT EXISTS idx_support_channel_outbox_identity_order
    ON support_channel_outbox (channel_identity_id, created_at)
    WHERE dispatch_status IN ('pending', 'dispatching');
CREATE INDEX IF NOT EXISTS idx_support_channel_outbox_message
    ON support_channel_outbox (message_id);
