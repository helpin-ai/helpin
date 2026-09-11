-- Waiting includes open conversations whose last public reply came from a teammate.
-- Conversation lifecycle statuses remain unchanged.

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
AFTER UPDATE OF status, mailbox_id, assigned_user_id, assigned_agent_id,
    flow_state, ai_state, human_takeover, customer_requested_human_at,
    customer_awaiting_response, needs_human_reply, last_public_sender_type
ON support_conversations
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION support_refresh_conversation_core_counters();

-- Historical rows can still await the general projection backfill. Initialize
-- just the public-reply fields needed by Waiting without marking the other
-- projections complete or changing lifecycle status.
DO $$
DECLARE
    historical_id UUID;
    public_reply support_messages%ROWTYPE;
BEGIN
    FOR historical_id IN
        SELECT id FROM support_conversations
        WHERE status = 'open' AND support_state_version = 0
        ORDER BY id FOR UPDATE
    LOOP
        SELECT * INTO public_reply FROM support_messages
        WHERE conversation_id = historical_id
          AND deleted_at IS NULL AND system_event_type IS NULL
          AND message_type = 'reply' AND is_internal = FALSE
        ORDER BY created_at DESC, id DESC LIMIT 1;
        IF public_reply.sender_type = 'user' THEN
            UPDATE support_conversations
            SET last_public_message_id = public_reply.id,
                last_public_message_at = public_reply.created_at,
                last_public_sender_type = public_reply.sender_type,
                customer_awaiting_response = FALSE,
                needs_human_reply = FALSE
            WHERE id = historical_id;
        END IF;
    END LOOP;
END;
$$;

-- Populate Waiting for existing public teammate replies, including V2 workspaces.
DO $$
DECLARE
    conversation_id UUID;
BEGIN
    FOR conversation_id IN
        SELECT id FROM support_conversations
        WHERE status = 'open' AND last_public_sender_type = 'user'
          AND NOT COALESCE(customer_awaiting_response, FALSE)
        ORDER BY id
        FOR UPDATE
    LOOP
        PERFORM support_refresh_core_counters(conversation_id);
    END LOOP;
END;
$$;
