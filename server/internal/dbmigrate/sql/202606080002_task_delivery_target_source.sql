ALTER TABLE task_delivery_targets
    ADD COLUMN IF NOT EXISTS target_source TEXT NOT NULL DEFAULT 'manual';

ALTER TABLE task_delivery_targets
    ADD COLUMN IF NOT EXISTS source_epic_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_task_delivery_targets_source_epic_id
    ON task_delivery_targets(source_epic_id);

DO $$
BEGIN
    ALTER TABLE task_delivery_targets
        ADD CONSTRAINT chk_task_delivery_targets_target_source
        CHECK (target_source IN ('manual', 'team_default', 'epic'));
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
