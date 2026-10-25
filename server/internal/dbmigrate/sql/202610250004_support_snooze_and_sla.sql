-- Snooze, editable priority and SLA policies.
--
-- A snoozed conversation leaves every team queue and counter (like a merged
-- original) until the snooze ends, the customer replies, or a teammate wakes
-- it. Customers still see an open conversation.

ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS snoozed_until timestamptz,
    ADD COLUMN IF NOT EXISTS snoozed_by_user_id uuid;

CREATE INDEX IF NOT EXISTS idx_support_conversations_snoozed_until
    ON support_conversations (snoozed_until);

-- Snoozed conversations contribute to no queue counters.
CREATE OR REPLACE FUNCTION support_refresh_user_core_counters(
    target_conversation_id UUID,
    target_user_id UUID
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    conversation support_conversations%ROWTYPE;
    personal_state support_conversation_user_states%ROWTYPE;
    scope_id TEXT;
    ai_active BOOLEAN;
    ai_resolved BOOLEAN;
    human_inbox BOOLEAN;
    waiting BOOLEAN;
    support_member BOOLEAN;
    effective_unread BOOLEAN;
BEGIN
    SELECT * INTO conversation FROM support_conversations WHERE id = target_conversation_id;
    SELECT * INTO personal_state
    FROM support_conversation_user_states
    WHERE conversation_id = target_conversation_id AND user_id = target_user_id;
    IF conversation.id IS NULL OR personal_state.user_id IS NULL THEN
        RETURN;
    END IF;

    scope_id := COALESCE(conversation.mailbox_id::TEXT, 'shared');
    ai_active := COALESCE(conversation.human_takeover, FALSE) = FALSE
        AND (COALESCE(conversation.flow_state, '') = 'ai_handling'
             OR (COALESCE(conversation.flow_state, '') = '' AND COALESCE(conversation.ai_state, '') = 'pending'));
    ai_resolved := COALESCE(conversation.human_takeover, FALSE) = FALSE
        AND (COALESCE(conversation.flow_state, '') = 'resolved_by_ai'
             OR (COALESCE(conversation.flow_state, '') = '' AND COALESCE(conversation.ai_state, '') = 'resolved'));
    human_inbox := conversation.status IN ('open', 'waiting_on_customer')
        AND ((NOT ai_active AND NOT ai_resolved)
             OR conversation.ai_state = 'escalated'
             OR conversation.customer_requested_human_at IS NOT NULL);
    waiting := conversation.status = 'waiting_on_customer'
        OR (conversation.status = 'open' AND COALESCE(conversation.last_public_sender_type, '') = 'user'
            AND NOT COALESCE(conversation.customer_awaiting_response, FALSE));
    support_member := conversation.status NOT IN ('resolved', 'spam') AND NOT ai_resolved;
    effective_unread := personal_state.unread_customer_message_count > 0 OR personal_state.manually_unread;

    IF conversation.merged_into_id IS NOT NULL OR conversation.snoozed_until IS NOT NULL THEN
        ai_active := FALSE; ai_resolved := FALSE; human_inbox := FALSE;
        waiting := FALSE; support_member := FALSE;
    END IF;

    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'user', personal_state.user_id::TEXT, 'inbox', human_inbox::INTEGER::SMALLINT,
        (human_inbox AND conversation.needs_human_reply)::INTEGER::SMALLINT,
        (human_inbox AND effective_unread)::INTEGER::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'user', personal_state.user_id::TEXT, 'mine',
        (human_inbox AND personal_state.relevance_mask <> 0)::INTEGER::SMALLINT,
        (human_inbox AND personal_state.relevance_mask <> 0 AND conversation.needs_human_reply)::INTEGER::SMALLINT,
        (human_inbox AND personal_state.relevance_mask <> 0 AND effective_unread)::INTEGER::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'user', personal_state.user_id::TEXT, 'waiting',
        (waiting)::INTEGER::SMALLINT,
        (waiting AND conversation.needs_human_reply)::INTEGER::SMALLINT,
        (waiting AND effective_unread)::INTEGER::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'user', personal_state.user_id::TEXT, 'ai_active', ai_active::INTEGER::SMALLINT,
        (ai_active AND conversation.needs_human_reply)::INTEGER::SMALLINT,
        (ai_active AND effective_unread)::INTEGER::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'user', personal_state.user_id::TEXT, 'unassigned',
        (human_inbox AND conversation.assigned_agent_id IS NULL AND conversation.assigned_user_id IS NULL)::INTEGER::SMALLINT,
        (human_inbox AND conversation.assigned_agent_id IS NULL AND conversation.assigned_user_id IS NULL
            AND conversation.needs_human_reply)::INTEGER::SMALLINT,
        (human_inbox AND conversation.assigned_agent_id IS NULL AND conversation.assigned_user_id IS NULL
            AND effective_unread)::INTEGER::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'user', personal_state.user_id::TEXT, 'support', support_member::INTEGER::SMALLINT,
        (support_member AND conversation.needs_human_reply)::INTEGER::SMALLINT,
        (support_member AND effective_unread)::INTEGER::SMALLINT);
END;
$$;

CREATE OR REPLACE FUNCTION support_refresh_core_counters(target_conversation_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    conversation support_conversations%ROWTYPE;
    personal_state support_conversation_user_states%ROWTYPE;
    scope_id TEXT;
    ai_active BOOLEAN;
    ai_resolved BOOLEAN;
    human_inbox BOOLEAN;
    waiting BOOLEAN;
    support_member BOOLEAN;
BEGIN
    SELECT * INTO conversation
    FROM support_conversations
    WHERE id = target_conversation_id;
    IF NOT FOUND THEN
        RETURN;
    END IF;

    scope_id := COALESCE(conversation.mailbox_id::TEXT, 'shared');
    ai_active := COALESCE(conversation.human_takeover, FALSE) = FALSE
        AND (COALESCE(conversation.flow_state, '') = 'ai_handling'
             OR (COALESCE(conversation.flow_state, '') = '' AND COALESCE(conversation.ai_state, '') = 'pending'));
    ai_resolved := COALESCE(conversation.human_takeover, FALSE) = FALSE
        AND (COALESCE(conversation.flow_state, '') = 'resolved_by_ai'
             OR (COALESCE(conversation.flow_state, '') = '' AND COALESCE(conversation.ai_state, '') = 'resolved'));
    human_inbox := conversation.status IN ('open', 'waiting_on_customer')
        AND ((NOT ai_active AND NOT ai_resolved)
             OR conversation.ai_state = 'escalated'
             OR conversation.customer_requested_human_at IS NOT NULL);
    waiting := conversation.status = 'waiting_on_customer'
        OR (conversation.status = 'open' AND COALESCE(conversation.last_public_sender_type, '') = 'user'
            AND NOT COALESCE(conversation.customer_awaiting_response, FALSE));
    support_member := conversation.status NOT IN ('resolved', 'spam') AND NOT ai_resolved;

    IF conversation.merged_into_id IS NOT NULL OR conversation.snoozed_until IS NOT NULL THEN
        ai_active := FALSE; ai_resolved := FALSE; human_inbox := FALSE;
        waiting := FALSE; support_member := FALSE;
    END IF;

    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'shared', 'shared', 'inbox', human_inbox::INTEGER::SMALLINT,
        (human_inbox AND conversation.needs_human_reply)::INTEGER::SMALLINT, 0::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'shared', 'shared', 'waiting', (waiting)::INTEGER::SMALLINT,
        (waiting AND conversation.needs_human_reply)::INTEGER::SMALLINT, 0::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'shared', 'shared', 'ai_active', ai_active::INTEGER::SMALLINT,
        (ai_active AND conversation.needs_human_reply)::INTEGER::SMALLINT, 0::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'shared', 'shared', 'unassigned',
        (human_inbox AND conversation.assigned_agent_id IS NULL AND conversation.assigned_user_id IS NULL)::INTEGER::SMALLINT,
        (human_inbox AND conversation.assigned_agent_id IS NULL AND conversation.assigned_user_id IS NULL
            AND conversation.needs_human_reply)::INTEGER::SMALLINT, 0::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'shared', 'shared', 'support', support_member::INTEGER::SMALLINT,
        (support_member AND conversation.needs_human_reply)::INTEGER::SMALLINT, 0::SMALLINT);

    FOR personal_state IN
        SELECT * FROM support_conversation_user_states
        WHERE conversation_id = conversation.id
    LOOP
        PERFORM support_refresh_user_core_counters(conversation.id, personal_state.user_id);
    END LOOP;
END;
$$;

DROP TRIGGER IF EXISTS support_conversation_core_counters_updated ON support_conversations;
CREATE TRIGGER support_conversation_core_counters_updated
AFTER UPDATE OF merged_into_id, status, mailbox_id, assigned_user_id, assigned_agent_id,
    flow_state, ai_state, human_takeover, customer_requested_human_at,
    customer_awaiting_response, needs_human_reply, last_public_sender_type, snoozed_until
ON support_conversations
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION support_refresh_conversation_core_counters();

-- A public customer reply wakes a snoozed conversation on every write path.
CREATE OR REPLACE FUNCTION support_wake_snoozed_conversation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.deleted_at IS NULL AND NEW.system_event_type IS NULL AND NOT NEW.is_internal
        AND NEW.message_type = 'reply' AND NEW.sender_type = 'customer' THEN
        UPDATE support_conversations
        SET snoozed_until = NULL, snoozed_by_user_id = NULL
        WHERE id = NEW.conversation_id AND snoozed_until IS NOT NULL;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_message_wakes_snooze ON support_messages;
CREATE TRIGGER support_message_wakes_snooze
AFTER INSERT ON support_messages
FOR EACH ROW
EXECUTE FUNCTION support_wake_snoozed_conversation();

CREATE TABLE IF NOT EXISTS support_sla_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    name text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    position integer NOT NULL DEFAULT 0,
    priorities text[] NOT NULL DEFAULT '{}',
    mailbox_ids text[] NOT NULL DEFAULT '{}',
    channels text[] NOT NULL DEFAULT '{}',
    first_response_minutes integer CHECK (first_response_minutes BETWEEN 1 AND 129600),
    next_response_minutes integer CHECK (next_response_minutes BETWEEN 1 AND 129600),
    resolution_minutes integer CHECK (resolution_minutes BETWEEN 1 AND 129600),
    business_hours boolean NOT NULL DEFAULT false,
    pause_while_waiting boolean NOT NULL DEFAULT true,
    active_since timestamptz NOT NULL DEFAULT now(),
    created_by_id uuid,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_support_sla_policies_workspace_id
    ON support_sla_policies (workspace_id);

CREATE TABLE IF NOT EXISTS support_conversation_slas (
    conversation_id uuid PRIMARY KEY REFERENCES support_conversations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    policy_id uuid,
    policy_name text NOT NULL DEFAULT '',
    first_response_state text NOT NULL DEFAULT '',
    first_response_due_at timestamptz,
    first_response_achieved_at timestamptz,
    first_response_breached_at timestamptz,
    next_response_state text NOT NULL DEFAULT '',
    next_response_due_at timestamptz,
    next_response_breached_at timestamptz,
    next_response_breaches integer NOT NULL DEFAULT 0,
    resolution_state text NOT NULL DEFAULT '',
    resolution_due_at timestamptz,
    resolution_achieved_at timestamptz,
    resolution_breached_at timestamptz,
    paused_since timestamptz,
    paused_seconds bigint NOT NULL DEFAULT 0,
    next_due_at timestamptz,
    wake_at timestamptz,
    breached boolean NOT NULL DEFAULT false,
    updated_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_support_conversation_slas_due
    ON support_conversation_slas (workspace_id, next_due_at);

CREATE INDEX IF NOT EXISTS idx_support_conversation_slas_wake_at
    ON support_conversation_slas (wake_at);

CREATE TABLE IF NOT EXISTS support_sla_work (
    conversation_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    enqueued_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_support_sla_work_enqueued_at
    ON support_sla_work (enqueued_at);

-- Queue a conversation for SLA evaluation when its workspace uses SLAs or
-- it already carries an SLA record.
CREATE OR REPLACE FUNCTION support_enqueue_sla_work(target_conversation_id uuid, target_workspace_id uuid)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM support_sla_policies WHERE workspace_id = target_workspace_id AND enabled)
        OR EXISTS (SELECT 1 FROM support_conversation_slas WHERE conversation_id = target_conversation_id) THEN
        INSERT INTO support_sla_work (conversation_id, workspace_id, enqueued_at)
        VALUES (target_conversation_id, target_workspace_id, NOW())
        ON CONFLICT (conversation_id) DO UPDATE SET enqueued_at = EXCLUDED.enqueued_at;
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION support_conversation_sla_changed()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM support_enqueue_sla_work(NEW.id, NEW.workspace_id);
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_conversation_sla_inserted ON support_conversations;
CREATE TRIGGER support_conversation_sla_inserted
AFTER INSERT ON support_conversations
FOR EACH ROW
EXECUTE FUNCTION support_conversation_sla_changed();

DROP TRIGGER IF EXISTS support_conversation_sla_updated ON support_conversations;
CREATE TRIGGER support_conversation_sla_updated
AFTER UPDATE OF status, priority, mailbox_id, channel, snoozed_until, resolved_at, merged_into_id
ON support_conversations
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status
    OR OLD.priority IS DISTINCT FROM NEW.priority
    OR OLD.mailbox_id IS DISTINCT FROM NEW.mailbox_id
    OR OLD.channel IS DISTINCT FROM NEW.channel
    OR OLD.snoozed_until IS DISTINCT FROM NEW.snoozed_until
    OR OLD.resolved_at IS DISTINCT FROM NEW.resolved_at
    OR OLD.merged_into_id IS DISTINCT FROM NEW.merged_into_id)
EXECUTE FUNCTION support_conversation_sla_changed();

CREATE OR REPLACE FUNCTION support_message_sla_changed()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.message_type = 'reply' AND NOT NEW.is_internal AND NEW.system_event_type IS NULL THEN
        PERFORM support_enqueue_sla_work(NEW.conversation_id, NEW.workspace_id);
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_message_sla_inserted ON support_messages;
CREATE TRIGGER support_message_sla_inserted
AFTER INSERT ON support_messages
FOR EACH ROW
EXECUTE FUNCTION support_message_sla_changed();

DROP TRIGGER IF EXISTS support_message_sla_deleted ON support_messages;
CREATE TRIGGER support_message_sla_deleted
AFTER UPDATE OF deleted_at ON support_messages
FOR EACH ROW
WHEN (OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
EXECUTE FUNCTION support_message_sla_changed();
