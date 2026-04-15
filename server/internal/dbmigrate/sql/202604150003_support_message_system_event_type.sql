-- Add explicit system_event_type to support_messages so each surface can
-- branch on intent instead of keyword-matching the rendered prose.
-- Plan: docs/plans/2026-04-15-system-message-event-type-plan.md

ALTER TABLE support_messages
    ADD COLUMN IF NOT EXISTS system_event_type VARCHAR(40);

CREATE INDEX IF NOT EXISTS idx_support_messages_system_event
    ON support_messages (conversation_id, system_event_type)
    WHERE system_event_type IS NOT NULL;
