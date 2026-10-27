-- The help assistant answers help center and portal visitors from published
-- help articles in a conversation. Sessions never create support
-- conversations; each answered turn is one agent run.
CREATE TABLE IF NOT EXISTS help_assistant_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    surface TEXT NOT NULL CHECK (surface IN ('helpcenter', 'portal')),
    audience TEXT NOT NULL CHECK (audience IN ('public', 'customers')),
    portal_identity_id UUID,
    visitor_id TEXT NOT NULL,
    locale TEXT NOT NULL DEFAULT '',
    turn_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_help_assistant_sessions_workspace_created
    ON help_assistant_sessions (workspace_id, created_at);

CREATE TABLE IF NOT EXISTS help_assistant_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES help_assistant_sessions (id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('pending', 'answered', 'handed_off', 'failed')),
    progress_label TEXT,
    citations JSONB,
    page_article_id TEXT,
    run_id UUID,
    helpful BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_help_assistant_messages_session
    ON help_assistant_messages (session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_help_assistant_messages_workspace_created
    ON help_assistant_messages (workspace_id, created_at);
CREATE INDEX IF NOT EXISTS idx_help_assistant_messages_pending
    ON help_assistant_messages (workspace_id, created_at) WHERE status = 'pending';
