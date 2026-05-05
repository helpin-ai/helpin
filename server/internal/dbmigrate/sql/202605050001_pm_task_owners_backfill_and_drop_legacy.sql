-- Backfill any legacy PM task owner rows into pm_task_owners, then drop legacy columns.

-- Backfill #1 (idempotent): translate workspace_member_id -> user_id and insert.
INSERT INTO pm_task_owners (task_id, user_id, created_at)
SELECT t.id, wm.user_id, NOW()
FROM pm_tasks t
JOIN workspace_members wm ON wm.id = t.owner_member_id
WHERE t.owner_member_id IS NOT NULL
ON CONFLICT (task_id, user_id) DO NOTHING;

-- Backfill #2 (idempotent): pre-member-era rows that only set owner_id (user-id directly).
INSERT INTO pm_task_owners (task_id, user_id, created_at)
SELECT t.id, t.owner_id, NOW()
FROM pm_tasks t
WHERE t.owner_id IS NOT NULL
ON CONFLICT (task_id, user_id) DO NOTHING;

-- Drop legacy single-owner columns from pm_tasks.
ALTER TABLE pm_tasks DROP COLUMN IF EXISTS owner_member_id;
ALTER TABLE pm_tasks DROP COLUMN IF EXISTS owner_id;
