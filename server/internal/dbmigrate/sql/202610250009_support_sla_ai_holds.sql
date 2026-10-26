-- SLA targets wait while the AI answers again after a handoff, and policies
-- that start applying to existing conversations measure from that moment.

ALTER TABLE support_conversation_slas
    ADD COLUMN IF NOT EXISTS ai_holding_since timestamptz,
    ADD COLUMN IF NOT EXISTS targets_from timestamptz;

ALTER TABLE support_sla_policies
    ADD COLUMN IF NOT EXISTS conditions_changed_at timestamptz;

-- The server always writes these flags; a column default of true would
-- hide a false value GORM leaves out of the insert.
ALTER TABLE support_sla_policies
    ALTER COLUMN enabled DROP DEFAULT,
    ALTER COLUMN pause_while_waiting DROP DEFAULT;

ALTER TABLE support_conversation_slas
    ALTER COLUMN target_pause_while_waiting DROP DEFAULT;
