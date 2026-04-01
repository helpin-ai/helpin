-- Reference only: this repo applies support conversation triage schema changes
-- via GORM AutoMigrate in main.go. This file documents the intended PostgreSQL
-- shape for operators and reviews.

ALTER TABLE support_mailboxes
    ADD COLUMN IF NOT EXISTS routing_prompt TEXT;

ALTER TABLE support_mailboxes
    ADD COLUMN IF NOT EXISTS triage_eligible BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE IF NOT EXISTS support_conversation_triage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL UNIQUE REFERENCES support_conversations(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'suggested',
    intent TEXT,
    confidence DOUBLE PRECISION,
    reason TEXT,
    source TEXT NOT NULL,
    suggested_mailbox_id UUID REFERENCES support_mailboxes(id) ON DELETE SET NULL,
    auto_moved BOOLEAN NOT NULL DEFAULT FALSE,
    locked_at TIMESTAMPTZ,
    evaluated_at TIMESTAMPTZ,
    feedback_action TEXT,
    input_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_conversation_triage_workspace_conversation
    ON support_conversation_triage (workspace_id, conversation_id);

CREATE INDEX IF NOT EXISTS idx_support_conversation_triage_input_hash
    ON support_conversation_triage (input_hash);

CREATE TABLE IF NOT EXISTS support_conversation_triage_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    triage_id UUID REFERENCES support_conversation_triage(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    source TEXT,
    cached BOOLEAN NOT NULL DEFAULT FALSE,
    from_mailbox_id UUID REFERENCES support_mailboxes(id) ON DELETE SET NULL,
    to_mailbox_id UUID REFERENCES support_mailboxes(id) ON DELETE SET NULL,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    input_hash TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_conversation_triage_events_conversation
    ON support_conversation_triage_events (workspace_id, conversation_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_conversation_triage_events_input_hash
    ON support_conversation_triage_events (workspace_id, input_hash, created_at DESC);

CREATE TABLE IF NOT EXISTS support_triage_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    name TEXT NOT NULL,
    channels TEXT[] NOT NULL DEFAULT '{}',
    conditions JSONB NOT NULL DEFAULT '{}'::jsonb,
    target_mailbox_id UUID NOT NULL REFERENCES support_mailboxes(id) ON DELETE CASCADE,
    created_by_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_triage_rules_workspace_priority
    ON support_triage_rules (workspace_id, priority, created_at);

CREATE INDEX IF NOT EXISTS idx_support_triage_rules_workspace_active
    ON support_triage_rules (workspace_id, active);
