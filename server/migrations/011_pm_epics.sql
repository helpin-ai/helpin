-- 011_pm_epics.sql
-- PM epics and epic label associations

CREATE TABLE pm_epics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    epic_state_id UUID REFERENCES pm_epic_workflow_states(id) ON DELETE SET NULL,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    planned_start_date DATE,
    deadline DATE,
    started BOOLEAN NOT NULL DEFAULT false,
    started_at TIMESTAMPTZ,
    completed BOOLEAN NOT NULL DEFAULT false,
    completed_at TIMESTAMPTZ,
    position INT NOT NULL DEFAULT 0,
    color TEXT,
    health TEXT NOT NULL DEFAULT 'on_track' CHECK (health IN ('on_track', 'at_risk', 'off_track')),
    health_comment TEXT,
    archived BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Placeholder table for later objective hierarchy support.
CREATE TABLE pm_epic_objectives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    epic_id UUID NOT NULL REFERENCES pm_epics(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pm_epic_labels (
    epic_id UUID NOT NULL REFERENCES pm_epics(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES pm_labels(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (epic_id, label_id)
);

CREATE INDEX idx_pm_epics_workspace ON pm_epics(workspace_id);
CREATE INDEX idx_pm_epics_state ON pm_epics(epic_state_id);
CREATE INDEX idx_pm_epics_team ON pm_epics(team_id);
CREATE INDEX idx_pm_epics_owner ON pm_epics(owner_id);
CREATE INDEX idx_pm_epics_archived ON pm_epics(workspace_id, archived);
CREATE INDEX idx_pm_epics_position ON pm_epics(workspace_id, position);
CREATE INDEX idx_pm_epic_objectives_epic ON pm_epic_objectives(epic_id);
CREATE INDEX idx_pm_epic_labels_label ON pm_epic_labels(label_id);

CREATE TRIGGER trg_pm_epics_updated_at
    BEFORE UPDATE ON pm_epics
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_pm_epic_objectives_updated_at
    BEFORE UPDATE ON pm_epic_objectives
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
