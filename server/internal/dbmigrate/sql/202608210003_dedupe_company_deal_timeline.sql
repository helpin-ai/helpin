-- Remove analytics-backfilled deal stage milestones when a richer canonical
-- activity row exists for the same transition. The original backfill compared
-- timestamps exactly, even though the two records can differ by milliseconds.
DELETE FROM pm_activity_log AS backfilled
USING pm_activity_log AS canonical
WHERE backfilled.entity_type = 'deal'
  AND COALESCE(backfilled.metadata->>'backfilled', 'false') = 'true'
  AND COALESCE(NULLIF(backfilled.event_type, ''), backfilled.metadata->>'event_type') IN (
    'deal.stage_changed', 'deal.won', 'deal.lost'
  )
  AND canonical.workspace_id = backfilled.workspace_id
  AND canonical.entity_type = backfilled.entity_type
  AND canonical.entity_id = backfilled.entity_id
  AND canonical.id <> backfilled.id
  AND COALESCE(canonical.metadata->>'backfilled', 'false') <> 'true'
  AND COALESCE(NULLIF(canonical.event_type, ''), canonical.metadata->>'event_type') =
      COALESCE(NULLIF(backfilled.event_type, ''), backfilled.metadata->>'event_type')
  AND canonical.new_value IS NOT DISTINCT FROM backfilled.new_value
  AND canonical.created_at BETWEEN
      backfilled.created_at - INTERVAL '5 minutes'
      AND backfilled.created_at + INTERVAL '5 minutes';
