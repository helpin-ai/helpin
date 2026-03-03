-- 008_bonus.sql
-- Quarterly bonus calculations, finance settings, and audit log

-- ============================================================
-- Quarterly bonus calculations (per-employee results)
-- ============================================================
CREATE TABLE quarterly_bonus_calculations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    quarter_id UUID NOT NULL REFERENCES quarters(id) ON DELETE CASCADE,
    employee_id UUID NOT NULL REFERENCES workspace_people(id) ON DELETE CASCADE,
    final_amount DECIMAL(10, 2) NOT NULL DEFAULT 0,
    bonus_tier TEXT CHECK (bonus_tier IN ('A', 'B', 'C')),
    team_tqi INT CHECK (team_tqi >= 0 AND team_tqi <= 100),
    individual_iqi INT CHECK (individual_iqi >= 0 AND individual_iqi <= 100),
    final_score INT CHECK (final_score >= 0 AND final_score <= 100),
    base_salary DECIMAL(10, 2) DEFAULT 0,
    is_override BOOLEAN NOT NULL DEFAULT false,
    override_reason TEXT,
    override_applied_by UUID REFERENCES users(id),
    override_applied_at TIMESTAMPTZ,
    calculation_details JSONB,
    locked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, quarter_id, employee_id)
);

-- ============================================================
-- Quarterly finance settings (per-workspace, per-quarter)
-- ============================================================
CREATE TABLE quarterly_finance_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    quarter_id UUID NOT NULL REFERENCES quarters(id) ON DELETE CASCADE,
    mrr_start DECIMAL(12, 2) NOT NULL DEFAULT 0 CHECK (mrr_start >= 0),
    mrr_end DECIMAL(12, 2) NOT NULL DEFAULT 0 CHECK (mrr_end >= 0),
    bonus_pool_percentage INT NOT NULL DEFAULT 15 CHECK (bonus_pool_percentage >= 0 AND bonus_pool_percentage <= 100),
    max_bonus_pool DECIMAL(12, 2) CHECK (max_bonus_pool IS NULL OR max_bonus_pool >= 0),
    team_weight INT NOT NULL DEFAULT 30 CHECK (team_weight >= 0 AND team_weight <= 100),
    bonus_tiers JSONB NOT NULL DEFAULT '[]'::jsonb,
    total_pool DECIMAL(12, 2) NOT NULL DEFAULT 0 CHECK (total_pool >= 0),
    total_paid DECIMAL(12, 2) NOT NULL DEFAULT 0 CHECK (total_paid >= 0),
    pool_utilization DECIMAL(5, 2) NOT NULL DEFAULT 0 CHECK (pool_utilization >= 0 AND pool_utilization <= 100),
    budget_factor NUMERIC(5, 4) NOT NULL DEFAULT 0.60,
    total_basic_salary NUMERIC(14, 2) NOT NULL DEFAULT 0,
    locked_at TIMESTAMPTZ,
    locked_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, quarter_id)
);

-- ============================================================
-- Bonus audit log
-- ============================================================
CREATE TABLE bonus_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    quarter_id UUID NOT NULL REFERENCES quarters(id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN (
        'calculations_saved',
        'calculation_run',
        'tier_change',
        'override_applied',
        'override_removed',
        'finance_updated',
        'finance_settings_updated',
        'settings_updated',
        'quarter_locked',
        'quarter_unlocked',
        'bonus_approved',
        'bonus_exported',
        'bulk_save'
    )),
    employee_id UUID REFERENCES workspace_people(id),
    performed_by UUID NOT NULL REFERENCES users(id),
    performed_by_name TEXT NOT NULL,
    performed_by_role TEXT NOT NULL,
    old_value JSONB,
    new_value JSONB,
    justification TEXT,
    affected_count INT NOT NULL DEFAULT 1,
    performed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_quarterly_bonus_calc_workspace ON quarterly_bonus_calculations(workspace_id);
CREATE INDEX idx_quarterly_bonus_calc_quarter ON quarterly_bonus_calculations(quarter_id);
CREATE INDEX idx_quarterly_bonus_calc_employee ON quarterly_bonus_calculations(employee_id);
CREATE INDEX idx_quarterly_bonus_calc_tier ON quarterly_bonus_calculations(workspace_id, bonus_tier);
CREATE INDEX idx_quarterly_finance_workspace ON quarterly_finance_settings(workspace_id);
CREATE INDEX idx_quarterly_finance_quarter ON quarterly_finance_settings(quarter_id);
CREATE INDEX idx_bonus_audit_workspace ON bonus_audit_log(workspace_id);
CREATE INDEX idx_bonus_audit_quarter ON bonus_audit_log(quarter_id);
CREATE INDEX idx_bonus_audit_employee ON bonus_audit_log(employee_id);
CREATE INDEX idx_bonus_audit_performed_by ON bonus_audit_log(performed_by);
CREATE INDEX idx_bonus_audit_action ON bonus_audit_log(workspace_id, action);
CREATE INDEX idx_bonus_audit_performed_at ON bonus_audit_log(performed_at);

-- Partial indexes for locked records (fast lookups)
CREATE INDEX idx_quarterly_bonus_calc_locked ON quarterly_bonus_calculations(workspace_id, quarter_id, locked_at) WHERE locked_at IS NOT NULL;
CREATE INDEX idx_quarterly_finance_locked ON quarterly_finance_settings(workspace_id, locked_at) WHERE locked_at IS NOT NULL;

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_quarterly_bonus_calculations_updated_at
    BEFORE UPDATE ON quarterly_bonus_calculations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_quarterly_finance_settings_updated_at
    BEFORE UPDATE ON quarterly_finance_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
