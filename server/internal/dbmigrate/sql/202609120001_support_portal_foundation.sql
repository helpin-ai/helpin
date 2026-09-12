-- Customer portal requests are projections of support_conversations. These
-- tables only provide identity, opaque-reference, and immutable audit support.

ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS portal_visible BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS portal_visibility_changed_at TIMESTAMPTZ;
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_support_conversations_portal_visible ON support_conversations (portal_visible);
CREATE INDEX IF NOT EXISTS idx_support_conversations_deleted_at ON support_conversations (deleted_at);

CREATE TABLE IF NOT EXISTS support_portal_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT,
    auth_subject TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, email)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_portal_identity_subject ON support_portal_identities (auth_subject) WHERE auth_subject IS NOT NULL;

CREATE TABLE IF NOT EXISTS support_portal_request_references (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    conversation_id UUID NOT NULL,
    portal_identity_id UUID NOT NULL,
    reference TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, reference),
    UNIQUE (workspace_id, conversation_id)
);
CREATE INDEX IF NOT EXISTS idx_support_portal_request_references_identity ON support_portal_request_references (portal_identity_id);

CREATE TABLE IF NOT EXISTS support_portal_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    conversation_id UUID NOT NULL,
    portal_identity_id UUID,
    actor_type TEXT NOT NULL,
    actor_user_id UUID,
    event_type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_support_portal_audit_request_time ON support_portal_audit_events (workspace_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_portal_audit_conversation_time ON support_portal_audit_events (conversation_id, occurred_at DESC);
