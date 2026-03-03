-- 003_workspace_config.sql
-- Workspace settings, teams, people, team memberships, and managers

-- ============================================================
-- Workspace settings
-- ============================================================
CREATE TABLE workspace_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    quarter_start_date DATE,
    sprint_duration_weeks INT NOT NULL DEFAULT 2 CHECK (sprint_duration_weeks > 0),
    notifications_enabled BOOLEAN NOT NULL DEFAULT true,
    auto_calculate_bonuses BOOLEAN NOT NULL DEFAULT true,
    team_weight INT NOT NULL DEFAULT 30 CHECK (team_weight >= 0 AND team_weight <= 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id)
);

-- ============================================================
-- Workspace people (employees / managers / executives)
-- Defined before workspace_teams because workspace_teams.manager_id
-- references this table.
-- ============================================================
CREATE TABLE workspace_people (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('executive', 'manager', 'employee')),
    job_role TEXT NOT NULL,
    manager_id UUID REFERENCES workspace_people(id),
    hire_date DATE NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'inactive')) DEFAULT 'active',
    base_salary NUMERIC(12, 2) NOT NULL DEFAULT 0,
    active_for_bonus BOOLEAN NOT NULL DEFAULT true,
    active_for_evaluation BOOLEAN NOT NULL DEFAULT true,
    is_account_owner BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, email)
);

-- ============================================================
-- Workspace teams
-- ============================================================
CREATE TABLE workspace_teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    manager_id UUID REFERENCES workspace_people(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Team memberships (many-to-many between teams and people)
-- ============================================================
CREATE TABLE team_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES workspace_teams(id) ON DELETE CASCADE,
    person_id UUID NOT NULL REFERENCES workspace_people(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(team_id, person_id)
);

-- ============================================================
-- Workspace managers (additional manager-specific metadata)
-- ============================================================
CREATE TABLE workspace_managers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    person_id UUID NOT NULL REFERENCES workspace_people(id),
    can_create_goals BOOLEAN NOT NULL DEFAULT false,
    can_score_performance BOOLEAN NOT NULL DEFAULT true,
    reporting_to UUID REFERENCES workspace_people(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, person_id)
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_workspace_settings_workspace ON workspace_settings(workspace_id);
CREATE INDEX idx_workspace_people_workspace ON workspace_people(workspace_id);
CREATE INDEX idx_workspace_people_manager ON workspace_people(manager_id);
CREATE INDEX idx_workspace_people_status ON workspace_people(workspace_id, status);
CREATE INDEX idx_workspace_people_job_role ON workspace_people(job_role);
CREATE INDEX idx_workspace_people_active_bonus ON workspace_people(workspace_id) WHERE status = 'active' AND active_for_bonus = true;
CREATE INDEX idx_workspace_teams_workspace ON workspace_teams(workspace_id);
CREATE INDEX idx_workspace_teams_manager ON workspace_teams(manager_id);
CREATE INDEX idx_team_memberships_team ON team_memberships(team_id);
CREATE INDEX idx_team_memberships_person ON team_memberships(person_id);
CREATE INDEX idx_workspace_managers_workspace ON workspace_managers(workspace_id);
CREATE INDEX idx_workspace_managers_person ON workspace_managers(person_id);
CREATE INDEX idx_workspace_managers_reporting ON workspace_managers(reporting_to);

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_workspace_settings_updated_at
    BEFORE UPDATE ON workspace_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_workspace_people_updated_at
    BEFORE UPDATE ON workspace_people
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_workspace_teams_updated_at
    BEFORE UPDATE ON workspace_teams
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_workspace_managers_updated_at
    BEFORE UPDATE ON workspace_managers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
