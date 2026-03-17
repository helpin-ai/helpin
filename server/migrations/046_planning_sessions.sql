-- Planning sessions for interactive epic planning
CREATE TABLE IF NOT EXISTS planning_sessions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         UUID NOT NULL REFERENCES workspaces(id),
    epic_id              UUID NOT NULL REFERENCES pm_epics(id),
    agent_id             UUID NOT NULL REFERENCES pm_agents(id),
    status               TEXT NOT NULL DEFAULT 'active',
    planning_methodology TEXT NOT NULL DEFAULT 'structured_v1',
    spec_document_id     UUID,
    spec_sections        JSONB NOT NULL DEFAULT '[]',
    context_snapshot     JSONB NOT NULL DEFAULT '{}',
    token_usage          JSONB NOT NULL DEFAULT '{"input": 0, "output": 0}',
    started_by           UUID,
    started_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_planning_sessions_epic ON planning_sessions(epic_id);
CREATE INDEX IF NOT EXISTS idx_planning_sessions_workspace ON planning_sessions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_planning_sessions_status ON planning_sessions(workspace_id, status);

CREATE TABLE IF NOT EXISTS planning_session_messages (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id       UUID NOT NULL REFERENCES planning_sessions(id) ON DELETE CASCADE,
    role             TEXT NOT NULL,
    content          TEXT NOT NULL,
    message_type     TEXT NOT NULL DEFAULT 'message',
    section_metadata JSONB,
    tool_invocations JSONB,
    content_blocks   JSONB,
    token_usage      JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_planning_session_messages_session ON planning_session_messages(session_id, created_at);

-- Add active session reference to epics
ALTER TABLE pm_epics ADD COLUMN IF NOT EXISTS active_planning_session_id UUID;
CREATE INDEX IF NOT EXISTS idx_pm_epics_active_session ON pm_epics(active_planning_session_id) WHERE active_planning_session_id IS NOT NULL;
