-- 006_goals.sql
-- Company goals, team contributions, sprint goals, and goal drafts

-- ============================================================
-- Company goals (OKR key results / company-level goals)
-- ============================================================
CREATE TABLE company_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    quarter_id UUID NOT NULL REFERENCES quarters(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    goal_type TEXT NOT NULL CHECK (goal_type IN ('metric', 'milestone')),
    baseline NUMERIC,
    target NUMERIC,
    current_value NUMERIC DEFAULT 0,
    unit TEXT,
    status TEXT NOT NULL CHECK (status IN ('draft', 'active', 'completed', 'cancelled')) DEFAULT 'draft',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Goal team contributions
-- ============================================================
CREATE TABLE goal_team_contributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    goal_id UUID NOT NULL REFERENCES company_goals(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES workspace_teams(id) ON DELETE CASCADE,
    contribution_pct NUMERIC NOT NULL DEFAULT 0,
    target_value NUMERIC,
    current_value NUMERIC DEFAULT 0,
    rationale TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(goal_id, team_id)
);

-- ============================================================
-- Sprint goals
-- ============================================================
CREATE TABLE sprint_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sprint_id UUID NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES workspace_teams(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    weight INT NOT NULL DEFAULT 1 CHECK (weight >= 1 AND weight <= 5),
    done BOOLEAN NOT NULL DEFAULT false,
    kr_id UUID REFERENCES company_goals(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(sprint_id, team_id, title)
);

-- ============================================================
-- Goal drafts
-- ============================================================
CREATE TABLE goal_drafts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    quarter_id UUID NOT NULL REFERENCES quarters(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id),
    draft_data JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'submitted', 'approved')) DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_company_goals_workspace ON company_goals(workspace_id);
CREATE INDEX idx_company_goals_quarter ON company_goals(quarter_id);
CREATE INDEX idx_company_goals_status ON company_goals(workspace_id, status);
CREATE INDEX idx_company_goals_created_by ON company_goals(created_by);
CREATE INDEX idx_goal_team_contributions_goal ON goal_team_contributions(goal_id);
CREATE INDEX idx_goal_team_contributions_team ON goal_team_contributions(team_id);
CREATE INDEX idx_sprint_goals_sprint ON sprint_goals(sprint_id);
CREATE INDEX idx_sprint_goals_team ON sprint_goals(team_id);
CREATE INDEX idx_sprint_goals_kr ON sprint_goals(kr_id);
CREATE INDEX idx_goal_drafts_workspace ON goal_drafts(workspace_id);
CREATE INDEX idx_goal_drafts_quarter ON goal_drafts(quarter_id);
CREATE INDEX idx_goal_drafts_created_by ON goal_drafts(created_by);

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_company_goals_updated_at
    BEFORE UPDATE ON company_goals
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_goal_team_contributions_updated_at
    BEFORE UPDATE ON goal_team_contributions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_sprint_goals_updated_at
    BEFORE UPDATE ON sprint_goals
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_goal_drafts_updated_at
    BEFORE UPDATE ON goal_drafts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
