-- Migration: support_inbox_core_counters

CREATE OR REPLACE FUNCTION support_apply_core_counter_contribution(
    target_conversation_id UUID,
    target_workspace_id UUID,
    target_mailbox_scope_id TEXT,
    target_audience_type TEXT,
    target_audience_id TEXT,
    target_bucket_id TEXT,
    target_total SMALLINT,
    target_attention SMALLINT,
    target_unread SMALLINT
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    previous support_inbox_counter_contributions%ROWTYPE;
    total_delta BIGINT;
    attention_delta BIGINT;
    unread_delta BIGINT;
BEGIN
    target_total := COALESCE(target_total, 0);
    target_attention := COALESCE(target_attention, 0);
    target_unread := COALESCE(target_unread, 0);

    SELECT * INTO previous
    FROM support_inbox_counter_contributions
    WHERE conversation_id = target_conversation_id
      AND audience_type = target_audience_type
      AND audience_id = target_audience_id
      AND bucket_id = target_bucket_id
    FOR UPDATE;

    IF FOUND AND previous.mailbox_scope_id <> target_mailbox_scope_id THEN
        UPDATE support_inbox_counter_buckets
        SET total_count = total_count - previous.total_count,
            needs_human_reply_count = needs_human_reply_count - previous.needs_human_reply_count,
            unread_count = unread_count - previous.unread_count,
            version = version + 1,
            updated_at = NOW()
        WHERE workspace_id = previous.workspace_id
          AND mailbox_scope_id = previous.mailbox_scope_id
          AND audience_type = previous.audience_type
          AND audience_id = previous.audience_id
          AND bucket_type = 'core'
          AND bucket_id = previous.bucket_id;
        previous.total_count := 0;
        previous.needs_human_reply_count := 0;
        previous.unread_count := 0;
    END IF;

    total_delta := target_total - COALESCE(previous.total_count, 0);
    attention_delta := target_attention - COALESCE(previous.needs_human_reply_count, 0);
    unread_delta := target_unread - COALESCE(previous.unread_count, 0);

    IF total_delta <> 0 OR attention_delta <> 0 OR unread_delta <> 0 THEN
        INSERT INTO support_inbox_counter_buckets (
            workspace_id, mailbox_scope_id, audience_type, audience_id,
            bucket_type, bucket_id, total_count, needs_human_reply_count,
            unread_count, version, updated_at
        ) VALUES (
            target_workspace_id, target_mailbox_scope_id, target_audience_type,
            target_audience_id, 'core', target_bucket_id, target_total,
            target_attention, target_unread, 1, NOW()
        )
        ON CONFLICT (workspace_id, mailbox_scope_id, audience_type, audience_id, bucket_type, bucket_id)
        DO UPDATE SET
            total_count = support_inbox_counter_buckets.total_count + total_delta,
            needs_human_reply_count = support_inbox_counter_buckets.needs_human_reply_count + attention_delta,
            unread_count = support_inbox_counter_buckets.unread_count + unread_delta,
            version = support_inbox_counter_buckets.version + 1,
            updated_at = NOW();
    END IF;

    INSERT INTO support_inbox_counter_contributions (
        conversation_id, workspace_id, mailbox_scope_id, audience_type,
        audience_id, bucket_id, total_count, needs_human_reply_count,
        unread_count, updated_at
    ) VALUES (
        target_conversation_id, target_workspace_id, target_mailbox_scope_id,
        target_audience_type, target_audience_id, target_bucket_id,
        target_total, target_attention, target_unread, NOW()
    )
    ON CONFLICT (conversation_id, audience_type, audience_id, bucket_id)
    DO UPDATE SET
        workspace_id = EXCLUDED.workspace_id,
        mailbox_scope_id = EXCLUDED.mailbox_scope_id,
        total_count = EXCLUDED.total_count,
        needs_human_reply_count = EXCLUDED.needs_human_reply_count,
        unread_count = EXCLUDED.unread_count,
        updated_at = NOW();
END;
$$;

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
        (conversation.status = 'waiting_on_customer')::INTEGER::SMALLINT,
        (conversation.status = 'waiting_on_customer' AND conversation.needs_human_reply)::INTEGER::SMALLINT,
        (conversation.status = 'waiting_on_customer' AND effective_unread)::INTEGER::SMALLINT);
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
    support_member := conversation.status NOT IN ('resolved', 'spam') AND NOT ai_resolved;

    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'shared', 'shared', 'inbox', human_inbox::INTEGER::SMALLINT,
        (human_inbox AND conversation.needs_human_reply)::INTEGER::SMALLINT, 0::SMALLINT);
    PERFORM support_apply_core_counter_contribution(conversation.id, conversation.workspace_id, scope_id,
        'shared', 'shared', 'waiting', (conversation.status = 'waiting_on_customer')::INTEGER::SMALLINT,
        (conversation.status = 'waiting_on_customer' AND conversation.needs_human_reply)::INTEGER::SMALLINT, 0::SMALLINT);
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

CREATE OR REPLACE FUNCTION support_refresh_personal_core_counters()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    conversation_id UUID := COALESCE(NEW.conversation_id, OLD.conversation_id);
    removed_user_id TEXT;
    contribution support_inbox_counter_contributions%ROWTYPE;
BEGIN
    IF TG_OP = 'DELETE' THEN
        removed_user_id := OLD.user_id::TEXT;
        FOR contribution IN
            SELECT * FROM support_inbox_counter_contributions
            WHERE conversation_id = OLD.conversation_id
              AND audience_type = 'user'
              AND audience_id = removed_user_id
            FOR UPDATE
        LOOP
            PERFORM support_apply_core_counter_contribution(
                contribution.conversation_id, contribution.workspace_id,
                contribution.mailbox_scope_id, 'user', removed_user_id,
                contribution.bucket_id, 0::SMALLINT, 0::SMALLINT, 0::SMALLINT);
        END LOOP;
        DELETE FROM support_inbox_counter_contributions
        WHERE conversation_id = OLD.conversation_id
          AND audience_type = 'user'
          AND audience_id = removed_user_id;
    ELSE
        PERFORM support_refresh_user_core_counters(conversation_id, NEW.user_id);
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$;

DROP TRIGGER IF EXISTS support_user_state_core_counters_inserted ON support_conversation_user_states;
CREATE TRIGGER support_user_state_core_counters_inserted
AFTER INSERT ON support_conversation_user_states
FOR EACH ROW
EXECUTE FUNCTION support_refresh_personal_core_counters();

DROP TRIGGER IF EXISTS support_user_state_core_counters_updated ON support_conversation_user_states;
CREATE TRIGGER support_user_state_core_counters_updated
AFTER UPDATE ON support_conversation_user_states
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION support_refresh_personal_core_counters();

DROP TRIGGER IF EXISTS support_user_state_core_counters_deleted ON support_conversation_user_states;
CREATE TRIGGER support_user_state_core_counters_deleted
AFTER DELETE ON support_conversation_user_states
FOR EACH ROW
EXECUTE FUNCTION support_refresh_personal_core_counters();

CREATE OR REPLACE FUNCTION support_refresh_conversation_core_counters()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    contribution support_inbox_counter_contributions%ROWTYPE;
BEGIN
    IF TG_OP = 'DELETE' THEN
        FOR contribution IN
            SELECT * FROM support_inbox_counter_contributions
            WHERE conversation_id = OLD.id
            FOR UPDATE
        LOOP
            PERFORM support_apply_core_counter_contribution(
                contribution.conversation_id, contribution.workspace_id,
                contribution.mailbox_scope_id, contribution.audience_type,
                contribution.audience_id, contribution.bucket_id,
                0::SMALLINT, 0::SMALLINT, 0::SMALLINT);
        END LOOP;
        DELETE FROM support_inbox_counter_contributions WHERE conversation_id = OLD.id;
    ELSE
        PERFORM support_refresh_core_counters(NEW.id);
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$;

-- The update trigger name sorts before support_conversation_state_updated.
-- That decomposes a transition into (new conversation, old personal state),
-- followed by personal-state trigger deltas, without double counting.
DROP TRIGGER IF EXISTS support_conversation_core_counters_inserted ON support_conversations;
CREATE TRIGGER support_conversation_core_counters_inserted
AFTER INSERT ON support_conversations
FOR EACH ROW
EXECUTE FUNCTION support_refresh_conversation_core_counters();

DROP TRIGGER IF EXISTS support_conversation_core_counters_updated ON support_conversations;
CREATE TRIGGER support_conversation_core_counters_updated
AFTER UPDATE OF status, mailbox_id, assigned_user_id, assigned_agent_id,
    flow_state, ai_state, human_takeover, customer_requested_human_at,
    customer_awaiting_response, needs_human_reply
ON support_conversations
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION support_refresh_conversation_core_counters();

DROP TRIGGER IF EXISTS support_conversation_core_counters_deleted ON support_conversations;
CREATE TRIGGER support_conversation_core_counters_deleted
BEFORE DELETE ON support_conversations
FOR EACH ROW
EXECUTE FUNCTION support_refresh_conversation_core_counters();
