-- Stable event semantics and read-path indexes for the unified company timeline.
-- This version intentionally follows an already-used 202608210001 migration number.
ALTER TABLE pm_activity_log
    ADD COLUMN IF NOT EXISTS event_type TEXT;

CREATE INDEX IF NOT EXISTS idx_pm_activity_timeline_events
    ON pm_activity_log (workspace_id, entity_type, entity_id, created_at DESC, id DESC)
    WHERE event_type IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_crm_associations_from_timeline
    ON crm_associations (workspace_id, from_object_type, from_object_id, to_object_type, to_object_id);

CREATE INDEX IF NOT EXISTS idx_crm_associations_to_timeline
    ON crm_associations (workspace_id, to_object_type, to_object_id, from_object_type, from_object_id);

CREATE INDEX IF NOT EXISTS idx_crm_activities_company_timeline
    ON crm_activities (workspace_id, company_id, occurred_at DESC, id DESC)
    WHERE company_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_crm_activities_contact_timeline
    ON crm_activities (workspace_id, contact_id, occurred_at DESC, id DESC)
    WHERE contact_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_crm_email_message_contacts_contact_timeline
    ON crm_email_message_contacts (workspace_id, contact_id, message_id);

CREATE INDEX IF NOT EXISTS idx_crm_calendar_events_contacts_timeline
    ON crm_calendar_events USING GIN (contact_ids);

CREATE INDEX IF NOT EXISTS idx_support_conversations_company_timeline
    ON support_conversations (workspace_id, crm_company_id, id)
    WHERE crm_company_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_conversations_contact_timeline
    ON support_conversations (workspace_id, crm_contact_id, id)
    WHERE crm_contact_id IS NOT NULL;

UPDATE crm_activities a
SET metadata = COALESCE(a.metadata, '{}'::jsonb) || jsonb_build_object(
    'meeting_id', m.id,
    'event_type', 'meeting.captured',
    'immutable', true
)
FROM crm_meetings m
WHERE m.activity_id = a.id
  AND a.activity_type = 'meeting'
  AND COALESCE((a.metadata->>'immutable')::boolean, false) = false;

UPDATE pm_activity_log
SET event_type = CASE
    WHEN action LIKE 'created this task%' THEN 'task.created'
    WHEN action = 'marked this task as blocked' THEN 'task.blocked'
    WHEN action = 'unblocked this task' THEN 'task.unblocked'
    WHEN action LIKE 'moved this task to %' THEN 'task.state_changed'
    ELSE event_type
END
WHERE entity_type = 'task'
  AND event_type IS NULL
  AND (
    action LIKE 'created this task%'
    OR action LIKE 'moved this task to %'
    OR action IN ('marked this task as blocked', 'unblocked this task')
  );

-- Deal creation predates durable CRM activity events. Synthesize the milestone at
-- the original creation time so existing companies receive useful history.
INSERT INTO pm_activity_log (
    workspace_id, entity_type, entity_id, actor_id, event_type, action,
    field_name, old_value, new_value, metadata, created_at
)
SELECT
    d.workspace_id,
    'deal',
    d.id,
    NULL,
    'deal.created',
    'created this deal in ' || COALESCE(s.name, 'its initial stage'),
    'stage',
    NULL,
    d.stage_id::text,
    jsonb_build_object('stage_name', s.name, 'stage_type', s.stage_type, 'backfilled', true),
    d.created_at
FROM crm_deals d
LEFT JOIN crm_pipeline_stages s ON s.id = d.stage_id
WHERE NOT EXISTS (
    SELECT 1
    FROM pm_activity_log pal
    WHERE pal.workspace_id = d.workspace_id
      AND pal.entity_type = 'deal'
      AND pal.entity_id = d.id
      AND COALESCE(NULLIF(pal.event_type, ''), pal.metadata->>'event_type') = 'deal.created'
);

-- Recover stage milestones emitted before durable deal activity logging existed.
INSERT INTO pm_activity_log (
    workspace_id, entity_type, entity_id, actor_id, event_type, action,
    field_name, old_value, new_value, metadata, created_at
)
SELECT
    o.workspace_id,
    'deal',
    d.id,
    o.user_id,
    CASE o.attributes->>'stage_type'
        WHEN 'won' THEN 'deal.won'
        WHEN 'lost' THEN 'deal.lost'
        ELSE 'deal.stage_changed'
    END,
    'moved this deal to ' || COALESCE(s.name, 'another stage'),
    'stage',
    o.attributes->>'previous_stage_id',
    o.attributes->>'stage_id',
    o.attributes || jsonb_build_object('stage_name', s.name, 'backfilled', true),
    o.occurred_at
FROM product_analytics_outbox o
JOIN crm_deals d
  ON d.id::text = o.attributes->>'entity_id'
 AND d.workspace_id = o.workspace_id
LEFT JOIN crm_pipeline_stages s ON s.id::text = o.attributes->>'stage_id'
WHERE o.event_name = 'crm_deal_stage_changed'
  AND o.workspace_id IS NOT NULL
  AND NULLIF(o.attributes->>'entity_id', '') IS NOT NULL
  AND NULLIF(o.attributes->>'stage_id', '') IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM pm_activity_log pal
    WHERE pal.workspace_id = o.workspace_id
      AND pal.entity_type = 'deal'
      AND pal.entity_id::text = o.attributes->>'entity_id'
      AND COALESCE(NULLIF(pal.event_type, ''), pal.metadata->>'event_type') IN ('deal.stage_changed', 'deal.won', 'deal.lost')
      AND pal.created_at = o.occurred_at
  );
