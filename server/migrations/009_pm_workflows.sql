-- 009_pm_workflows.sql
-- PM workflows, workflow states, and epic workflow states

CREATE TABLE pm_workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    default_state_id UUID,
    auto_assign_owner BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

CREATE TABLE pm_workflow_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES pm_workflows(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    state_type TEXT NOT NULL CHECK (state_type IN ('backlog', 'unstarted', 'started', 'done')),
    position INT NOT NULL DEFAULT 0,
    color TEXT,
    description TEXT,
    wip_limit INT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workflow_id, name)
);

CREATE TABLE pm_epic_workflow_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    state_type TEXT NOT NULL CHECK (state_type IN ('unstarted', 'started', 'done')),
    position INT NOT NULL DEFAULT 0,
    color TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

ALTER TABLE pm_workflows
    ADD CONSTRAINT fk_pm_workflows_default_state
    FOREIGN KEY (default_state_id) REFERENCES pm_workflow_states(id) ON DELETE SET NULL;

CREATE INDEX idx_pm_workflows_workspace ON pm_workflows(workspace_id);
CREATE INDEX idx_pm_workflows_team ON pm_workflows(team_id);
CREATE INDEX idx_pm_workflow_states_workflow ON pm_workflow_states(workflow_id);
CREATE INDEX idx_pm_workflow_states_type ON pm_workflow_states(workflow_id, state_type);
CREATE INDEX idx_pm_workflow_states_position ON pm_workflow_states(workflow_id, position);
CREATE INDEX idx_pm_epic_workflow_states_workspace ON pm_epic_workflow_states(workspace_id);
CREATE INDEX idx_pm_epic_workflow_states_position ON pm_epic_workflow_states(workspace_id, position);

CREATE TRIGGER trg_pm_workflows_updated_at
    BEFORE UPDATE ON pm_workflows
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_pm_workflow_states_updated_at
    BEFORE UPDATE ON pm_workflow_states
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_pm_epic_workflow_states_updated_at
    BEFORE UPDATE ON pm_epic_workflow_states
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
