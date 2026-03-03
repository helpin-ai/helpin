-- 005_quarters_sprints.sql
-- Quarters and sprints

-- ============================================================
-- Quarters
-- ============================================================
CREATE TABLE quarters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'active', 'completed', 'archived')) DEFAULT 'draft',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

-- ============================================================
-- Sprints
-- ============================================================
CREATE TABLE sprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quarter_id UUID NOT NULL REFERENCES quarters(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    sprint_number INT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('planning', 'active', 'completed', 'locked')) DEFAULT 'planning',
    locked_at TIMESTAMPTZ,
    locked_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(quarter_id, sprint_number)
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_quarters_workspace ON quarters(workspace_id);
CREATE INDEX idx_quarters_status ON quarters(workspace_id, status);
CREATE INDEX idx_quarters_created_by ON quarters(created_by);
CREATE INDEX idx_sprints_quarter ON sprints(quarter_id);
CREATE INDEX idx_sprints_workspace ON sprints(workspace_id);
CREATE INDEX idx_sprints_status ON sprints(workspace_id, status);
CREATE INDEX idx_sprints_locked_by ON sprints(locked_by);

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_quarters_updated_at
    BEFORE UPDATE ON quarters
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_sprints_updated_at
    BEFORE UPDATE ON sprints
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
