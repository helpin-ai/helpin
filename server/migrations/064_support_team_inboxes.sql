CREATE TABLE IF NOT EXISTS support_mailboxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    handle TEXT NOT NULL,
    icon TEXT NOT NULL DEFAULT 'inbox',
    description TEXT,
    linked_team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    visibility_mode TEXT NOT NULL DEFAULT 'members_only',
    assignment_mode TEXT NOT NULL DEFAULT 'manual',
    position INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_mailboxes_workspace_handle
    ON support_mailboxes (workspace_id, lower(handle));

CREATE INDEX IF NOT EXISTS idx_support_mailboxes_workspace_position
    ON support_mailboxes (workspace_id, position);

CREATE INDEX IF NOT EXISTS idx_support_mailboxes_workspace_active
    ON support_mailboxes (workspace_id, active);

CREATE TABLE IF NOT EXISTS support_mailbox_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mailbox_id UUID NOT NULL REFERENCES support_mailboxes(id) ON DELETE CASCADE,
    workspace_member_id UUID NOT NULL REFERENCES workspace_members(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_mailbox_memberships_unique
    ON support_mailbox_memberships (mailbox_id, workspace_member_id);

CREATE INDEX IF NOT EXISTS idx_support_mailbox_memberships_mailbox
    ON support_mailbox_memberships (mailbox_id);

CREATE INDEX IF NOT EXISTS idx_support_mailbox_memberships_workspace_member
    ON support_mailbox_memberships (workspace_member_id);

ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS mailbox_id UUID REFERENCES support_mailboxes(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_support_conversations_workspace_mailbox
    ON support_conversations (workspace_id, mailbox_id);
