-- Migration: support_inbox_state_foundation

ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS list_last_message_id UUID,
    ADD COLUMN IF NOT EXISTS list_last_message_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS list_last_message_preview TEXT,
    ADD COLUMN IF NOT EXISTS list_last_message_is_internal BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS last_public_message_id UUID,
    ADD COLUMN IF NOT EXISTS last_public_message_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_public_sender_type TEXT,
    ADD COLUMN IF NOT EXISTS last_public_sender_display_name TEXT,
    ADD COLUMN IF NOT EXISTS last_customer_message_id UUID,
    ADD COLUMN IF NOT EXISTS last_customer_message_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS unanswered_customer_message_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS customer_awaiting_response BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS needs_human_reply BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS support_state_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS visitor_country_code TEXT,
    ADD COLUMN IF NOT EXISTS visitor_country_name TEXT,
    ADD COLUMN IF NOT EXISTS view_search_document TEXT;

CREATE TABLE IF NOT EXISTS support_conversation_user_states (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_customer_message_id UUID,
    last_read_customer_message_at TIMESTAMPTZ,
    unread_customer_message_count INTEGER NOT NULL DEFAULT 0 CHECK (unread_customer_message_count >= 0),
    manually_unread BOOLEAN NOT NULL DEFAULT FALSE,
    mentioned_at TIMESTAMPTZ,
    relevance_mask INTEGER NOT NULL DEFAULT 0 CHECK (relevance_mask >= 0),
    version BIGINT NOT NULL DEFAULT 0 CHECK (version >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_counter_buckets (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    audience_type TEXT NOT NULL CHECK (audience_type IN ('shared', 'user')),
    audience_id TEXT NOT NULL,
    bucket_type TEXT NOT NULL,
    bucket_id TEXT NOT NULL,
    total_count BIGINT NOT NULL DEFAULT 0 CHECK (total_count >= 0),
    needs_human_reply_count BIGINT NOT NULL DEFAULT 0 CHECK (needs_human_reply_count >= 0),
    unread_count BIGINT NOT NULL DEFAULT 0 CHECK (unread_count >= 0),
    version BIGINT NOT NULL DEFAULT 0 CHECK (version >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, mailbox_scope_id, audience_type, audience_id, bucket_type, bucket_id)
);

-- One normalized row per conversation/audience/bucket is the idempotency
-- boundary for counter maintenance. It lets a live write safely initialize an
-- unbackfilled conversation and lets mailbox moves subtract from the exact old
-- scope before adding to the new one.
CREATE TABLE IF NOT EXISTS support_inbox_counter_contributions (
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    audience_type TEXT NOT NULL CHECK (audience_type IN ('shared', 'user')),
    audience_id TEXT NOT NULL,
    bucket_id TEXT NOT NULL,
    total_count SMALLINT NOT NULL CHECK (total_count IN (0, 1)),
    needs_human_reply_count SMALLINT NOT NULL CHECK (needs_human_reply_count IN (0, 1)),
    unread_count SMALLINT NOT NULL CHECK (unread_count IN (0, 1)),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, audience_type, audience_id, bucket_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_scope_heads (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    shared_version BIGINT NOT NULL DEFAULT 0 CHECK (shared_version >= 0),
    applied_view_watermark BIGINT NOT NULL DEFAULT 0 CHECK (applied_view_watermark >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, mailbox_scope_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_user_scope_heads (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    personal_version BIGINT NOT NULL DEFAULT 0 CHECK (personal_version >= 0),
    applied_view_watermark BIGINT NOT NULL DEFAULT 0 CHECK (applied_view_watermark >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id, mailbox_scope_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_access_versions (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version BIGINT NOT NULL DEFAULT 0 CHECK (version >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_conversation_projection_states (
    conversation_id UUID PRIMARY KEY REFERENCES support_conversations(id) ON DELETE CASCADE,
    generation BIGINT NOT NULL DEFAULT 1 CHECK (generation > 0),
    contribution_hash TEXT NOT NULL,
    initialized_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (conversation_id, generation)
);

CREATE TABLE IF NOT EXISTS support_inbox_conversation_contributions (
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    generation BIGINT NOT NULL CHECK (generation > 0),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    shared_contribution JSONB NOT NULL DEFAULT '{}'::jsonb,
    personal_contributions JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, generation)
);

CREATE TABLE IF NOT EXISTS support_inbox_conversation_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mutation_id UUID NOT NULL,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    audience_type TEXT NOT NULL CHECK (audience_type IN ('shared', 'user')),
    audience_id TEXT NOT NULL,
    source_version BIGINT NOT NULL CHECK (source_version > 0),
    old_values JSONB NOT NULL DEFAULT '{}'::jsonb,
    new_values JSONB NOT NULL DEFAULT '{}'::jsonb,
    affected_user_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    applied_at TIMESTAMPTZ,
    UNIQUE (workspace_id, mailbox_scope_id, audience_type, audience_id, source_version),
    UNIQUE (mutation_id, mailbox_scope_id, audience_type, audience_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_view_scope_counts (
    view_id UUID NOT NULL REFERENCES support_inbox_views(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    total_count BIGINT NOT NULL DEFAULT 0 CHECK (total_count >= 0),
    needs_human_reply_count BIGINT NOT NULL DEFAULT 0 CHECK (needs_human_reply_count >= 0),
    source_version BIGINT NOT NULL DEFAULT 0 CHECK (source_version >= 0),
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (view_id, mailbox_scope_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_view_user_counts (
    view_id UUID NOT NULL REFERENCES support_inbox_views(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mailbox_scope_id TEXT NOT NULL,
    total_adjustment BIGINT NOT NULL DEFAULT 0,
    needs_human_reply_adjustment BIGINT NOT NULL DEFAULT 0,
    unread_count BIGINT NOT NULL DEFAULT 0 CHECK (unread_count >= 0),
    shared_source_version BIGINT NOT NULL DEFAULT 0 CHECK (shared_source_version >= 0),
    personal_source_version BIGINT NOT NULL DEFAULT 0 CHECK (personal_source_version >= 0),
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (view_id, user_id, mailbox_scope_id)
);

CREATE TABLE IF NOT EXISTS support_inbox_view_projection_states (
    view_id UUID NOT NULL REFERENCES support_inbox_views(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    mailbox_scope_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('building', 'ready', 'failed')),
    shared_watermark BIGINT NOT NULL DEFAULT 0 CHECK (shared_watermark >= 0),
    personal_watermark BIGINT NOT NULL DEFAULT 0 CHECK (personal_watermark >= 0),
    error TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (view_id, user_id, mailbox_scope_id)
);

CREATE TABLE IF NOT EXISTS support_realtime_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    target_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    state_version BIGINT NOT NULL DEFAULT 0,
    personal_state_version BIGINT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'delivered', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_expires_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS support_inbox_state_rollouts (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    mode TEXT NOT NULL DEFAULT 'legacy' CHECK (mode IN ('legacy', 'shadow', 'v2')),
    generation BIGINT NOT NULL DEFAULT 1 CHECK (generation > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
