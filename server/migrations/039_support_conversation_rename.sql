-- Migration 039: Rename support_ticket references to support_conversation
-- This completes the terminology migration started in 038.

-- Update CRM associations
UPDATE crm_associations SET from_object_type = 'support_conversation'
  WHERE from_object_type = 'support_ticket';
UPDATE crm_associations SET to_object_type = 'support_conversation'
  WHERE to_object_type = 'support_ticket';

-- Update activity log
UPDATE pm_activity_log SET entity_type = 'support_conversation'
  WHERE entity_type = 'support_ticket';

-- Rename agent_runs column
ALTER TABLE agent_runs RENAME COLUMN ticket_id TO conversation_id;

-- Drop legacy columns (data backfilled in migration 038)
ALTER TABLE support_messages DROP COLUMN IF EXISTS ticket_id;
ALTER TABLE support_widget_sessions DROP COLUMN IF EXISTS ticket_id;
