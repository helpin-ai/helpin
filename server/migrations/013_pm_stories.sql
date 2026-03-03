-- 013_pm_stories.sql
-- PM stories and story associations

CREATE SEQUENCE IF NOT EXISTS pm_story_display_id_seq;

CREATE TABLE pm_stories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    display_id INT NOT NULL DEFAULT nextval('pm_story_display_id_seq'),
    name TEXT NOT NULL,
    description TEXT,
    story_type TEXT NOT NULL CHECK (story_type IN ('feature', 'bug', 'chore')) DEFAULT 'feature',
    workflow_id UUID NOT NULL REFERENCES pm_workflows(id) ON DELETE RESTRICT,
    workflow_state_id UUID NOT NULL REFERENCES pm_workflow_states(id) ON DELETE RESTRICT,
    epic_id UUID REFERENCES pm_epics(id) ON DELETE SET NULL,
    iteration_id UUID REFERENCES pm_iterations(id) ON DELETE SET NULL,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    requester_id UUID REFERENCES users(id) ON DELETE SET NULL,
    estimate INT,
    priority TEXT NOT NULL CHECK (priority IN ('none', 'low', 'medium', 'high', 'urgent')) DEFAULT 'none',
    severity TEXT NOT NULL CHECK (severity IN ('none', 'minor', 'major', 'critical')) DEFAULT 'none',
    deadline DATE,
    position INT NOT NULL DEFAULT 0,
    started BOOLEAN NOT NULL DEFAULT false,
    started_at TIMESTAMPTZ,
    completed BOOLEAN NOT NULL DEFAULT false,
    completed_at TIMESTAMPTZ,
    moved_at TIMESTAMPTZ,
    blocked BOOLEAN NOT NULL DEFAULT false,
    blocker TEXT,
    archived BOOLEAN NOT NULL DEFAULT false,
    template_id TEXT,
    external_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, display_id)
);

CREATE TABLE pm_story_owners (
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (story_id, user_id)
);

CREATE TABLE pm_story_followers (
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (story_id, user_id)
);

CREATE TABLE pm_story_labels (
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES pm_labels(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (story_id, label_id)
);

CREATE INDEX idx_pm_stories_workspace ON pm_stories(workspace_id);
CREATE INDEX idx_pm_stories_state ON pm_stories(workflow_state_id);
CREATE INDEX idx_pm_stories_epic ON pm_stories(epic_id);
CREATE INDEX idx_pm_stories_iteration ON pm_stories(iteration_id);
CREATE INDEX idx_pm_stories_team ON pm_stories(team_id);
CREATE INDEX idx_pm_stories_owner ON pm_stories(owner_id);
CREATE INDEX idx_pm_stories_display_id ON pm_stories(workspace_id, display_id);
CREATE INDEX idx_pm_stories_workflow_state_position ON pm_stories(workflow_state_id, position);
CREATE INDEX idx_pm_story_owners_user ON pm_story_owners(user_id);
CREATE INDEX idx_pm_story_followers_user ON pm_story_followers(user_id);
CREATE INDEX idx_pm_story_labels_label ON pm_story_labels(label_id);

CREATE TRIGGER trg_pm_stories_updated_at
    BEFORE UPDATE ON pm_stories
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
