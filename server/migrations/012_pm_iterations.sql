-- 012_pm_iterations.sql
-- PM iterations and label associations

CREATE TABLE pm_iterations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status TEXT GENERATED ALWAYS AS (
      CASE
        WHEN CURRENT_DATE < start_date THEN 'unstarted'
        WHEN CURRENT_DATE > end_date THEN 'done'
        ELSE 'started'
      END
    ) STORED,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    archived BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date > start_date)
);

CREATE TABLE pm_iteration_labels (
    iteration_id UUID NOT NULL REFERENCES pm_iterations(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES pm_labels(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (iteration_id, label_id)
);

CREATE INDEX idx_pm_iterations_workspace ON pm_iterations(workspace_id);
CREATE INDEX idx_pm_iterations_team ON pm_iterations(team_id);
CREATE INDEX idx_pm_iterations_archived ON pm_iterations(workspace_id, archived);
CREATE INDEX idx_pm_iterations_dates ON pm_iterations(workspace_id, start_date, end_date);
CREATE INDEX idx_pm_iteration_labels_label ON pm_iteration_labels(label_id);

CREATE TRIGGER trg_pm_iterations_updated_at
    BEFORE UPDATE ON pm_iterations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
