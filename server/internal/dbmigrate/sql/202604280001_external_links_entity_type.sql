-- Add entity_type and entity_id columns (nullable initially for safe backfill)
ALTER TABLE pm_external_links ADD COLUMN IF NOT EXISTS entity_type VARCHAR(32);
ALTER TABLE pm_external_links ADD COLUMN IF NOT EXISTS entity_id UUID;

-- Backfill entity_id from task_id for existing rows
UPDATE pm_external_links SET entity_type = 'task', entity_id = task_id WHERE entity_id IS NULL AND task_id IS NOT NULL;

-- Delete orphaned rows with no task_id (bad data safety net)
DELETE FROM pm_external_links WHERE task_id IS NULL AND entity_id IS NULL;

-- Now make entity_type and entity_id NOT NULL with defaults
ALTER TABLE pm_external_links ALTER COLUMN entity_type SET NOT NULL;
ALTER TABLE pm_external_links ALTER COLUMN entity_type SET DEFAULT 'task';
ALTER TABLE pm_external_links ALTER COLUMN entity_id SET NOT NULL;

-- Make task_id nullable (was NOT NULL)
ALTER TABLE pm_external_links ALTER COLUMN task_id DROP NOT NULL;

-- Add composite index
CREATE INDEX IF NOT EXISTS idx_pm_external_links_entity ON pm_external_links (entity_type, entity_id);
