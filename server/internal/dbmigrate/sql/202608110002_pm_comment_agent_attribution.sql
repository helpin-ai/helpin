ALTER TABLE pm_comments
    ADD COLUMN IF NOT EXISTS agent_id UUID,
    ADD COLUMN IF NOT EXISTS agent_name TEXT,
    ADD COLUMN IF NOT EXISTS agent_run_id UUID;

CREATE INDEX IF NOT EXISTS idx_pm_comments_agent_id
    ON pm_comments (agent_id)
    WHERE agent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pm_comments_agent_run_id
    ON pm_comments (agent_run_id)
    WHERE agent_run_id IS NOT NULL;
