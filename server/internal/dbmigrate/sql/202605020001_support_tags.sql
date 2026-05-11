-- Support inbox tags. User tags are stored here; AI handoff/resolved tags are
-- computed from conversation state and are not inserted into these tables.

CREATE TABLE IF NOT EXISTS support_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_tags_workspace_lower_name
    ON support_tags (workspace_id, lower(name));

CREATE INDEX IF NOT EXISTS idx_support_tags_workspace_id
    ON support_tags (workspace_id);

CREATE TABLE IF NOT EXISTS support_conversation_tags (
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES support_tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_support_conversation_tags_tag_id
    ON support_conversation_tags (tag_id);
