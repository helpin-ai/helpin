-- Migration: support_inbox_state_backfill

-- Processes a bounded set of previously uninitialized conversations. The
-- conversation row lock serializes the baseline with live message projection
-- triggers, while SKIP LOCKED lets multiple operators cooperate safely.
CREATE OR REPLACE FUNCTION support_inbox_backfill_batch(requested_batch_size INTEGER DEFAULT 250)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    conversation support_conversations%ROWTYPE;
    list_message support_messages%ROWTYPE;
    public_message support_messages%ROWTYPE;
    customer_message support_messages%ROWTYPE;
    read_message support_messages%ROWTYPE;
    last_response_at TIMESTAMPTZ;
    last_response_id UUID;
    unanswered_count INTEGER;
    processed INTEGER := 0;
BEGIN
    IF requested_batch_size < 1 OR requested_batch_size > 5000 THEN
        RAISE EXCEPTION 'batch size must be between 1 and 5000';
    END IF;

    FOR conversation IN
        SELECT candidate.*
        FROM support_conversations candidate
        LEFT JOIN support_inbox_conversation_projection_states projection
          ON projection.conversation_id = candidate.id
         AND projection.generation = 1
        WHERE projection.conversation_id IS NULL
        ORDER BY candidate.id
        FOR UPDATE OF candidate SKIP LOCKED
        LIMIT requested_batch_size
    LOOP
        SELECT * INTO list_message
        FROM support_messages
        WHERE conversation_id = conversation.id
          AND deleted_at IS NULL
          AND system_event_type IS NULL
          AND message_type = 'reply'
          AND (NOT is_internal OR TRIM(content) <> '')
        ORDER BY created_at DESC, id DESC LIMIT 1;

        SELECT * INTO public_message
        FROM support_messages
        WHERE conversation_id = conversation.id
          AND deleted_at IS NULL
          AND system_event_type IS NULL
          AND message_type = 'reply'
          AND is_internal = FALSE
        ORDER BY created_at DESC, id DESC LIMIT 1;

        SELECT * INTO customer_message
        FROM support_messages
        WHERE conversation_id = conversation.id
          AND deleted_at IS NULL
          AND system_event_type IS NULL
          AND message_type = 'reply'
          AND is_internal = FALSE
          AND sender_type = 'customer'
        ORDER BY created_at DESC, id DESC LIMIT 1;

        SELECT * INTO read_message
        FROM support_messages
        WHERE conversation_id = conversation.id
          AND deleted_at IS NULL
          AND system_event_type IS NULL
          AND message_type = 'reply'
          AND is_internal = FALSE
          AND sender_type = 'customer'
          AND created_at <= conversation.team_last_seen_at
        ORDER BY created_at DESC, id DESC LIMIT 1;

        last_response_at := NULL;
        last_response_id := NULL;
        unanswered_count := 0;
        IF public_message.id IS NOT NULL AND public_message.sender_type = 'customer' THEN
            SELECT created_at, id INTO last_response_at, last_response_id
            FROM support_messages
            WHERE conversation_id = conversation.id
              AND deleted_at IS NULL
              AND system_event_type IS NULL
              AND message_type = 'reply'
              AND is_internal = FALSE
              AND sender_type <> 'customer'
            ORDER BY created_at DESC, id DESC LIMIT 1;

            SELECT COUNT(*)::INTEGER INTO unanswered_count
            FROM support_messages
            WHERE conversation_id = conversation.id
              AND deleted_at IS NULL
              AND system_event_type IS NULL
              AND message_type = 'reply'
              AND is_internal = FALSE
              AND sender_type = 'customer'
              AND (last_response_at IS NULL OR (created_at, id) > (last_response_at, last_response_id));
        END IF;

        UPDATE support_conversations
        SET list_last_message_id = list_message.id,
            list_last_message_at = list_message.created_at,
            list_last_message_preview = CASE WHEN list_message.id IS NULL THEN NULL ELSE LEFT(TRIM(list_message.content), 500) END,
            list_last_message_is_internal = COALESCE(list_message.is_internal, FALSE),
            last_public_message_id = public_message.id,
            last_public_message_at = public_message.created_at,
            last_public_sender_type = public_message.sender_type,
            last_public_sender_display_name = public_message.sender_display_name,
            last_customer_message_id = customer_message.id,
            last_customer_message_at = customer_message.created_at,
            unanswered_customer_message_count = unanswered_count,
            customer_awaiting_response = COALESCE(public_message.sender_type = 'customer', FALSE),
            visitor_country_code = (
                SELECT session.country_code FROM support_widget_sessions session
                WHERE session.workspace_id = conversation.workspace_id
                  AND session.anonymous_id = conversation.anonymous_id
                ORDER BY session.created_at DESC LIMIT 1
            ),
            visitor_country_name = (
                SELECT session.country_name FROM support_widget_sessions session
                WHERE session.workspace_id = conversation.workspace_id
                  AND session.anonymous_id = conversation.anonymous_id
                ORDER BY session.created_at DESC LIMIT 1
            ),
            view_search_document = LOWER(CONCAT_WS(' ', subject, customer_name, customer_email)),
            support_state_version = GREATEST(support_state_version, 1)
        WHERE support_conversations.id = conversation.id;

        WITH recipient_bits AS (
            SELECT conversation.assigned_user_id AS user_id, 1 AS mask, NULL::TIMESTAMPTZ AS mentioned_at
            UNION ALL
            SELECT conversation.opened_by_user_id, 2, NULL::TIMESTAMPTZ
            UNION ALL
            SELECT mention.user_id, 4, MAX(mention.created_at)
            FROM (
                SELECT (jsonb_array_elements_text(COALESCE(message.metadata, '{}'::jsonb)->'mentioned_user_ids'))::UUID AS user_id,
                       message.created_at
                FROM support_messages message
                WHERE message.conversation_id = conversation.id
                  AND message.deleted_at IS NULL
            ) mention
            GROUP BY mention.user_id
        ), recipients AS (
            SELECT user_id, bit_or(mask) AS mask, MAX(mentioned_at) AS mentioned_at
            FROM recipient_bits
            WHERE user_id IS NOT NULL
            GROUP BY user_id
        )
        INSERT INTO support_conversation_user_states (
            workspace_id, conversation_id, user_id,
            last_read_customer_message_id, last_read_customer_message_at,
            unread_customer_message_count, mentioned_at, relevance_mask,
            version, created_at, updated_at
        )
        SELECT conversation.workspace_id, conversation.id, recipient.user_id,
               read_message.id, read_message.created_at,
               (
                   SELECT COUNT(*)::INTEGER
                   FROM support_messages message
                   WHERE message.conversation_id = conversation.id
                     AND message.deleted_at IS NULL
                     AND message.system_event_type IS NULL
                     AND message.message_type = 'reply'
                     AND message.is_internal = FALSE
                     AND message.sender_type = 'customer'
                     AND (conversation.team_last_seen_at IS NULL OR message.created_at > conversation.team_last_seen_at)
               ),
               recipient.mentioned_at, recipient.mask, 1, NOW(), NOW()
        FROM recipients recipient
        ON CONFLICT (conversation_id, user_id) DO NOTHING;

        INSERT INTO support_conversation_user_states (
            workspace_id, conversation_id, user_id,
            unread_customer_message_count, manually_unread, relevance_mask,
            version, created_at, updated_at
        )
        SELECT conversation.workspace_id, conversation.id, notification.recipient_id,
               0, TRUE, 0, 1, NOW(), NOW()
        FROM notifications notification
        WHERE notification.workspace_id = conversation.workspace_id
          AND notification.entity_type = 'support_conversation'
          AND notification.entity_id = conversation.id
          AND notification.latest_event_category = 'support_replies'
          AND notification.status = 'unread'
          AND NOT EXISTS (
              SELECT 1 FROM support_conversation_user_states existing
              WHERE existing.conversation_id = conversation.id
                AND existing.user_id = notification.recipient_id
                AND existing.unread_customer_message_count > 0
          )
        ON CONFLICT (conversation_id, user_id)
        DO UPDATE SET manually_unread = TRUE,
                      version = support_conversation_user_states.version + 1,
                      updated_at = NOW()
        WHERE support_conversation_user_states.unread_customer_message_count = 0
          AND support_conversation_user_states.manually_unread = FALSE;

        INSERT INTO support_inbox_conversation_projection_states (
            conversation_id, generation, contribution_hash, initialized_at, updated_at
        ) VALUES (
            conversation.id, 1,
            md5(CONCAT_WS('|', conversation.id::TEXT, public_message.id::TEXT, customer_message.id::TEXT, unanswered_count::TEXT)),
            NOW(), NOW()
        ) ON CONFLICT (conversation_id) DO NOTHING;

        -- The function is installed by the following migration before this
        -- backfill command is run. Keeping it here makes projection and counter
        -- initialization atomic for each locked conversation.
        PERFORM support_refresh_core_counters(conversation.id);

        processed := processed + 1;
    END LOOP;

    RETURN processed;
END;
$$;
