-- Reversible conversation handling. Original records, storage keys, and message
-- IDs stay intact. Shared history is granted per committed message, not by time.
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS merged_into_id uuid;
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS merge_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS origin_conversation_id uuid;
ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS merge_revision bigint NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_support_conversations_merged_into_id ON support_conversations (merged_into_id);
CREATE INDEX IF NOT EXISTS idx_support_messages_origin_conversation_id ON support_messages (origin_conversation_id);
CREATE TABLE IF NOT EXISTS support_conversation_merges (
 id uuid PRIMARY KEY,
 workspace_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 source_id uuid NOT NULL,
 target_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 cross_customer boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL,
 ended_at timestamptz,
 ended_by uuid,
 unmerge_operation_id uuid,
 CHECK (source_id <> target_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS support_merge_active_source ON support_conversation_merges (workspace_id, source_id) WHERE ended_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS support_merge_operation_source ON support_conversation_merges (workspace_id, operation_id, source_id);
CREATE INDEX IF NOT EXISTS support_merge_target ON support_conversation_merges (workspace_id, target_id);
CREATE INDEX IF NOT EXISTS support_merge_source ON support_conversation_merges (workspace_id, source_id);
CREATE TABLE IF NOT EXISTS support_merge_messages (
 workspace_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 message_id uuid NOT NULL,
 merge_id uuid NOT NULL,
 PRIMARY KEY (conversation_id, message_id)
);
CREATE INDEX IF NOT EXISTS support_merge_message_history ON support_merge_messages (workspace_id, conversation_id, merge_id);
CREATE INDEX IF NOT EXISTS support_merge_message_lookup ON support_merge_messages (message_id);
CREATE TABLE IF NOT EXISTS support_merge_jobs (
 id uuid PRIMARY KEY,
 workspace_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 run_id uuid,
 available_at timestamptz NOT NULL,
 lease_until timestamptz,
 completed_at timestamptz,
 created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS support_merge_jobs_pending ON support_merge_jobs (available_at) WHERE completed_at IS NULL;

-- Merged originals retain their history but leave active queue badges.
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

    IF conversation.merged_into_id IS NOT NULL THEN
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

    IF conversation.merged_into_id IS NOT NULL THEN
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
    customer_awaiting_response, needs_human_reply, last_public_sender_type
ON support_conversations
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION support_refresh_conversation_core_counters();
