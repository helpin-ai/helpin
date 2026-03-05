-- 017_rename_iterations_to_sprints.sql
-- Rename pm_iterations → pm_sprints, pm_iteration_labels → pm_sprint_labels

-- Drop existing indexes and trigger
DROP TRIGGER IF EXISTS trg_pm_iterations_updated_at ON pm_iterations;
DROP INDEX IF EXISTS idx_pm_iterations_workspace;
DROP INDEX IF EXISTS idx_pm_iterations_team;
DROP INDEX IF EXISTS idx_pm_iterations_archived;
DROP INDEX IF EXISTS idx_pm_iterations_dates;
DROP INDEX IF EXISTS idx_pm_iteration_labels_label;
DROP INDEX IF EXISTS idx_pm_stories_iteration;

-- Rename iteration_id column in pm_iteration_labels before renaming table
ALTER TABLE pm_iteration_labels RENAME COLUMN iteration_id TO sprint_id;

-- Rename tables
ALTER TABLE pm_iterations RENAME TO pm_sprints;
ALTER TABLE pm_iteration_labels RENAME TO pm_sprint_labels;

-- Rename iteration_id column in pm_stories
ALTER TABLE pm_stories RENAME COLUMN iteration_id TO sprint_id;

-- Recreate indexes with new names
CREATE INDEX idx_pm_sprints_workspace ON pm_sprints(workspace_id);
CREATE INDEX idx_pm_sprints_team ON pm_sprints(team_id);
CREATE INDEX idx_pm_sprints_archived ON pm_sprints(workspace_id, archived);
CREATE INDEX idx_pm_sprints_dates ON pm_sprints(workspace_id, start_date, end_date);
CREATE INDEX idx_pm_sprint_labels_label ON pm_sprint_labels(label_id);
CREATE INDEX idx_pm_stories_sprint ON pm_stories(sprint_id);

-- Recreate trigger
CREATE TRIGGER trg_pm_sprints_updated_at
    BEFORE UPDATE ON pm_sprints
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
