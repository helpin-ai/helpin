-- 007_scoring.sql
-- Individual performance checks / scoring

-- ============================================================
-- Individual checks (per-sprint, per-employee scoring)
-- ============================================================
CREATE TABLE individual_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sprint_id UUID NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    employee_id UUID NOT NULL REFERENCES workspace_people(id) ON DELETE CASCADE,
    scored_by UUID REFERENCES users(id),
    criteria_id TEXT NOT NULL,
    answer BOOLEAN NOT NULL DEFAULT false,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(sprint_id, employee_id, criteria_id)
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_individual_checks_sprint ON individual_checks(sprint_id);
CREATE INDEX idx_individual_checks_workspace ON individual_checks(workspace_id);
CREATE INDEX idx_individual_checks_employee ON individual_checks(employee_id);
CREATE INDEX idx_individual_checks_scored_by ON individual_checks(scored_by);
CREATE INDEX idx_individual_checks_criteria ON individual_checks(sprint_id, criteria_id);

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_individual_checks_updated_at
    BEFORE UPDATE ON individual_checks
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
