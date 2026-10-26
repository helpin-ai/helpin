-- SLA records keep the targets a conversation started with, so editing a
-- policy never re-judges history, and conversations the AI handled start
-- their clocks at the handoff to the team.

ALTER TABLE support_conversation_slas
    ADD COLUMN IF NOT EXISTS policy_applied_at timestamptz,
    ADD COLUMN IF NOT EXISTS detached_at timestamptz,
    ADD COLUMN IF NOT EXISTS target_first_response_minutes integer,
    ADD COLUMN IF NOT EXISTS target_next_response_minutes integer,
    ADD COLUMN IF NOT EXISTS target_resolution_minutes integer,
    ADD COLUMN IF NOT EXISTS target_business_hours boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS target_pause_while_waiting boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS ai_involved boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS human_since timestamptz,
    ADD COLUMN IF NOT EXISTS next_response_carried_breaches integer NOT NULL DEFAULT 0;

-- Records written before snapshots existed take their policy's current
-- targets.
UPDATE support_conversation_slas AS sla
SET target_first_response_minutes = policy.first_response_minutes,
    target_next_response_minutes = policy.next_response_minutes,
    target_resolution_minutes = policy.resolution_minutes,
    target_business_hours = policy.business_hours,
    target_pause_while_waiting = policy.pause_while_waiting
FROM support_sla_policies AS policy
WHERE policy.id = sla.policy_id
  AND sla.target_first_response_minutes IS NULL
  AND sla.target_next_response_minutes IS NULL
  AND sla.target_resolution_minutes IS NULL;

-- AI handoffs change when the team's clocks start.
DROP TRIGGER IF EXISTS support_conversation_sla_updated ON support_conversations;
CREATE TRIGGER support_conversation_sla_updated
AFTER UPDATE OF status, priority, mailbox_id, channel, snoozed_until, resolved_at, merged_into_id,
    flow_state, ai_state, human_takeover, ai_escalated_at, customer_requested_human_at, ai_paused_at
ON support_conversations
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status
    OR OLD.priority IS DISTINCT FROM NEW.priority
    OR OLD.mailbox_id IS DISTINCT FROM NEW.mailbox_id
    OR OLD.channel IS DISTINCT FROM NEW.channel
    OR OLD.snoozed_until IS DISTINCT FROM NEW.snoozed_until
    OR OLD.resolved_at IS DISTINCT FROM NEW.resolved_at
    OR OLD.merged_into_id IS DISTINCT FROM NEW.merged_into_id
    OR OLD.flow_state IS DISTINCT FROM NEW.flow_state
    OR OLD.ai_state IS DISTINCT FROM NEW.ai_state
    OR OLD.human_takeover IS DISTINCT FROM NEW.human_takeover
    OR OLD.ai_escalated_at IS DISTINCT FROM NEW.ai_escalated_at
    OR OLD.customer_requested_human_at IS DISTINCT FROM NEW.customer_requested_human_at
    OR OLD.ai_paused_at IS DISTINCT FROM NEW.ai_paused_at)
EXECUTE FUNCTION support_conversation_sla_changed();

-- Resolving or marking spam ends a snooze on every code path.
CREATE OR REPLACE FUNCTION support_clear_snooze_on_close()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status IN ('resolved', 'spam') AND NEW.snoozed_until IS NOT NULL THEN
        NEW.snoozed_until := NULL;
        NEW.snoozed_by_user_id := NULL;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_conversation_clears_snooze ON support_conversations;
CREATE TRIGGER support_conversation_clears_snooze
BEFORE UPDATE OF status ON support_conversations
FOR EACH ROW
WHEN (NEW.status IN ('resolved', 'spam'))
EXECUTE FUNCTION support_clear_snooze_on_close();
