-- Warnings before an SLA target is missed, and who else hears about
-- warnings and breaches.

ALTER TABLE support_sla_policies
    ADD COLUMN IF NOT EXISTS warn_before_minutes integer,
    ADD COLUMN IF NOT EXISTS notify_user_ids text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS notify_cs_owner boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS assign_unassigned_user_id uuid;

-- The due time each target was last warned for, so every due time warns
-- once, and a new next-response cycle warns again.
ALTER TABLE support_conversation_slas
    ADD COLUMN IF NOT EXISTS first_response_warned_for timestamptz,
    ADD COLUMN IF NOT EXISTS next_response_warned_for timestamptz,
    ADD COLUMN IF NOT EXISTS resolution_warned_for timestamptz;
