-- 004_job_roles.sql
-- Job-role evaluation criteria and bonus tiers

-- ============================================================
-- Job role criteria
-- ============================================================
CREATE TABLE job_role_criteria (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    job_role TEXT NOT NULL,
    criteria_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    question TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    weight INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, job_role, criteria_id)
);

-- ============================================================
-- Bonus tiers (A / B / C)
-- ============================================================
CREATE TABLE bonus_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    tier TEXT NOT NULL CHECK (tier IN ('A', 'B', 'C')),
    min_score INT NOT NULL CHECK (min_score >= 0 AND min_score <= 100),
    max_score INT NOT NULL CHECK (max_score >= 0 AND max_score <= 100),
    salary_multiplier DECIMAL(3, 2) NOT NULL CHECK (salary_multiplier >= 0),
    description TEXT,
    editable BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, tier),
    CHECK (min_score <= max_score)
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_job_role_criteria_workspace ON job_role_criteria(workspace_id);
CREATE INDEX idx_job_role_criteria_role ON job_role_criteria(workspace_id, job_role);
CREATE INDEX idx_bonus_tiers_workspace ON bonus_tiers(workspace_id);

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_job_role_criteria_updated_at
    BEFORE UPDATE ON job_role_criteria
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_bonus_tiers_updated_at
    BEFORE UPDATE ON bonus_tiers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
