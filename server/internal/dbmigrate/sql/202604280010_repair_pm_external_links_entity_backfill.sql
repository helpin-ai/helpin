-- Repair legacy pm_external_links rows where task_id may be blank or otherwise
-- invalid for UUID casting. 202604280001 did the original entity_id cutover; this
-- migration is intentionally additive so already-applied checksums stay stable.

ALTER TABLE pm_external_links ADD COLUMN IF NOT EXISTS entity_type VARCHAR(32);
ALTER TABLE pm_external_links ADD COLUMN IF NOT EXISTS entity_id UUID;

UPDATE pm_external_links
SET entity_type = COALESCE(NULLIF(entity_type, ''), 'task')
WHERE entity_type IS NULL
   OR entity_type = '';

UPDATE pm_external_links
SET entity_id = task_id::uuid
WHERE entity_id IS NULL
  AND task_id::text ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

DELETE FROM pm_external_links
WHERE entity_id IS NULL
  AND (
    task_id IS NULL
    OR task_id::text !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  );
