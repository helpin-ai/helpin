-- 014_pm_comments.sql
-- PM comments for stories, epics, docs

CREATE TABLE pm_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('story', 'epic', 'doc')),
    entity_id UUID NOT NULL,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    parent_id UUID REFERENCES pm_comments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pm_comments_entity ON pm_comments(entity_type, entity_id);
CREATE INDEX idx_pm_comments_author ON pm_comments(author_id);
CREATE INDEX idx_pm_comments_parent ON pm_comments(parent_id);

CREATE TRIGGER trg_pm_comments_updated_at
    BEFORE UPDATE ON pm_comments
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
