-- 019_pm_team_estimate_settings.sql
-- Per-team estimate configuration (scale, extended, zero, hours).

CREATE TABLE pm_team_estimate_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES workspace_teams(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    scale TEXT NOT NULL DEFAULT 'linear' CHECK (scale IN ('exponential', 'fibonacci', 'linear', 'tshirt', 'hours')),
    extended BOOLEAN NOT NULL DEFAULT false,
    allow_zero BOOLEAN NOT NULL DEFAULT false,
    count_unestimated_as_one BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(team_id)
);

CREATE INDEX idx_pm_team_estimate_settings_team ON pm_team_estimate_settings(team_id);

CREATE TRIGGER trg_pm_team_estimate_settings_updated_at
    BEFORE UPDATE ON pm_team_estimate_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
