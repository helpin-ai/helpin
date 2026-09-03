-- Migration: support_inbox_state_projection_triggers

CREATE OR REPLACE FUNCTION project_support_conversation_workload()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    ai_owns_response BOOLEAN;
    last_response_at TIMESTAMPTZ;
    last_response_id UUID;
BEGIN
    IF NEW.status IN ('resolved', 'spam') THEN
        NEW.customer_awaiting_response := FALSE;
        NEW.needs_human_reply := FALSE;
        NEW.unanswered_customer_message_count := 0;
        RETURN NEW;
    END IF;

    -- Resolving/spamming deliberately clears the workload projection, but a
    -- later reopen must restore it when the newest surviving public reply is
    -- still from the customer. Derive the burst again rather than trusting
    -- the zeroed value left by the terminal state.
    IF TG_OP = 'UPDATE'
       AND OLD.status IN ('resolved', 'spam')
       AND NEW.status NOT IN ('resolved', 'spam')
       AND NEW.last_public_sender_type = 'customer' THEN
        SELECT created_at, id INTO last_response_at, last_response_id
        FROM support_messages
        WHERE conversation_id = NEW.id
          AND deleted_at IS NULL
          AND system_event_type IS NULL
          AND message_type = 'reply'
          AND is_internal = FALSE
          AND sender_type <> 'customer'
        ORDER BY created_at DESC, id DESC
        LIMIT 1;

        SELECT COUNT(*)::INTEGER INTO NEW.unanswered_customer_message_count
        FROM support_messages
        WHERE conversation_id = NEW.id
          AND deleted_at IS NULL
          AND system_event_type IS NULL
          AND message_type = 'reply'
          AND is_internal = FALSE
          AND sender_type = 'customer'
          AND (last_response_at IS NULL OR (created_at, id) > (last_response_at, last_response_id));
        NEW.customer_awaiting_response := NEW.unanswered_customer_message_count > 0;
    END IF;

    ai_owns_response := COALESCE((COALESCE(NEW.human_takeover, FALSE) = FALSE
        AND (
            NEW.flow_state = 'ai_handling'
            OR (NEW.flow_state IS NULL AND NEW.ai_state = 'pending')
        )), FALSE);
    NEW.needs_human_reply := NEW.customer_awaiting_response AND NOT ai_owns_response;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_conversation_workload_changed ON support_conversations;
CREATE TRIGGER support_conversation_workload_changed
BEFORE INSERT OR UPDATE OF status, flow_state, ai_state, human_takeover,
    customer_awaiting_response, unanswered_customer_message_count
ON support_conversations
FOR EACH ROW
EXECUTE FUNCTION project_support_conversation_workload();

CREATE OR REPLACE FUNCTION project_support_message_state()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    conversation_row support_conversations%ROWTYPE;
    mailbox_scope TEXT;
    shared_version BIGINT;
    mutation_id UUID := gen_random_uuid();
    is_public_reply BOOLEAN;
    is_customer_reply BOOLEAN;
    is_list_message BOOLEAN;
BEGIN
    IF NEW.deleted_at IS NOT NULL OR NEW.system_event_type IS NOT NULL THEN
        RETURN NEW;
    END IF;

    SELECT * INTO conversation_row
    FROM support_conversations
    WHERE id = NEW.conversation_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RETURN NEW;
    END IF;

    mailbox_scope := COALESCE(conversation_row.mailbox_id::TEXT, 'shared');
    is_public_reply := NOT NEW.is_internal AND NEW.message_type = 'reply';
    is_customer_reply := is_public_reply AND NEW.sender_type = 'customer';
    is_list_message := NEW.message_type = 'reply' AND (NOT NEW.is_internal OR TRIM(NEW.content) <> '');

    INSERT INTO support_inbox_scope_heads (workspace_id, mailbox_scope_id, shared_version, updated_at)
    VALUES (conversation_row.workspace_id, mailbox_scope, 1, NOW())
    ON CONFLICT (workspace_id, mailbox_scope_id)
    DO UPDATE SET shared_version = support_inbox_scope_heads.shared_version + 1, updated_at = NOW()
    RETURNING support_inbox_scope_heads.shared_version INTO shared_version;

    UPDATE support_conversations
    SET list_last_message_id = CASE
            WHEN is_list_message AND (list_last_message_at IS NULL OR (NEW.created_at, NEW.id) > (list_last_message_at, list_last_message_id))
            THEN NEW.id ELSE list_last_message_id END,
        list_last_message_at = CASE
            WHEN is_list_message AND (list_last_message_at IS NULL OR (NEW.created_at, NEW.id) > (list_last_message_at, list_last_message_id))
            THEN NEW.created_at ELSE list_last_message_at END,
        list_last_message_preview = CASE
            WHEN is_list_message AND (list_last_message_at IS NULL OR (NEW.created_at, NEW.id) > (list_last_message_at, list_last_message_id))
            THEN LEFT(TRIM(NEW.content), 500) ELSE list_last_message_preview END,
        list_last_message_is_internal = CASE
            WHEN is_list_message AND (list_last_message_at IS NULL OR (NEW.created_at, NEW.id) > (list_last_message_at, list_last_message_id))
            THEN NEW.is_internal ELSE list_last_message_is_internal END,
        last_public_message_id = CASE WHEN is_public_reply AND
            (last_public_message_at IS NULL OR (NEW.created_at, NEW.id) > (last_public_message_at, last_public_message_id))
            THEN NEW.id ELSE last_public_message_id END,
        last_public_message_at = CASE WHEN is_public_reply AND
            (last_public_message_at IS NULL OR (NEW.created_at, NEW.id) > (last_public_message_at, last_public_message_id))
            THEN NEW.created_at ELSE last_public_message_at END,
        last_public_sender_type = CASE WHEN is_public_reply AND
            (last_public_message_at IS NULL OR (NEW.created_at, NEW.id) > (last_public_message_at, last_public_message_id))
            THEN NEW.sender_type ELSE last_public_sender_type END,
        last_public_sender_display_name = CASE WHEN is_public_reply AND
            (last_public_message_at IS NULL OR (NEW.created_at, NEW.id) > (last_public_message_at, last_public_message_id))
            THEN NEW.sender_display_name ELSE last_public_sender_display_name END,
        last_customer_message_id = CASE WHEN is_customer_reply AND
            (last_customer_message_at IS NULL OR (NEW.created_at, NEW.id) > (last_customer_message_at, last_customer_message_id))
            THEN NEW.id ELSE last_customer_message_id END,
        last_customer_message_at = CASE WHEN is_customer_reply AND
            (last_customer_message_at IS NULL OR (NEW.created_at, NEW.id) > (last_customer_message_at, last_customer_message_id))
            THEN NEW.created_at ELSE last_customer_message_at END,
        unanswered_customer_message_count = CASE
            WHEN is_customer_reply THEN unanswered_customer_message_count + 1
            WHEN is_public_reply THEN 0
            ELSE unanswered_customer_message_count END,
        customer_awaiting_response = CASE
            WHEN is_customer_reply THEN TRUE
            WHEN is_public_reply THEN FALSE
            ELSE customer_awaiting_response END,
        support_state_version = support_state_version + 1,
        updated_at = GREATEST(updated_at, NEW.created_at)
    WHERE id = NEW.conversation_id;

    WITH recipient_bits AS (
        SELECT conversation_row.assigned_user_id AS user_id, 1 AS mask
        UNION ALL
        SELECT conversation_row.opened_by_user_id, 2
        UNION ALL
        SELECT mention_id::UUID, 4
        FROM jsonb_array_elements_text(COALESCE(NEW.metadata, '{}'::jsonb)->'mentioned_user_ids') mention_id
    ), recipients AS (
        SELECT user_id, bit_or(mask) AS mask
        FROM recipient_bits
        WHERE user_id IS NOT NULL
        GROUP BY user_id
    )
    INSERT INTO support_conversation_user_states (
        workspace_id, conversation_id, user_id, manually_unread, mentioned_at,
        relevance_mask, version, created_at, updated_at
    )
    SELECT conversation_row.workspace_id, NEW.conversation_id, user_id,
        NEW.is_internal AND (mask & 4) <> 0,
        CASE WHEN (mask & 4) <> 0 THEN NEW.created_at ELSE NULL END,
        mask, CASE WHEN NEW.is_internal AND (mask & 4) <> 0 THEN 1 ELSE 0 END,
        NOW(), NOW()
    FROM recipients
    ON CONFLICT (conversation_id, user_id)
    DO UPDATE SET
        relevance_mask = support_conversation_user_states.relevance_mask | EXCLUDED.relevance_mask,
        manually_unread = support_conversation_user_states.manually_unread OR EXCLUDED.manually_unread,
        mentioned_at = CASE WHEN (EXCLUDED.relevance_mask & 4) <> 0
            THEN GREATEST(support_conversation_user_states.mentioned_at, EXCLUDED.mentioned_at)
            ELSE support_conversation_user_states.mentioned_at END,
        version = support_conversation_user_states.version
            + CASE WHEN EXCLUDED.manually_unread AND NOT support_conversation_user_states.manually_unread THEN 1 ELSE 0 END,
        updated_at = NOW();

    IF is_customer_reply THEN
        UPDATE support_conversation_user_states
        SET unread_customer_message_count = support_conversation_user_states.unread_customer_message_count + 1,
            version = support_conversation_user_states.version + 1,
            updated_at = NOW()
        WHERE conversation_id = NEW.conversation_id
          AND relevance_mask <> 0
          AND (
              last_read_customer_message_at IS NULL
              OR (NEW.created_at, NEW.id) > (last_read_customer_message_at, last_read_customer_message_id)
          );
    END IF;

    INSERT INTO support_inbox_conversation_changes (
        mutation_id, workspace_id, conversation_id, mailbox_scope_id,
        audience_type, audience_id, source_version, new_values, affected_user_ids
    )
    VALUES (
        mutation_id, conversation_row.workspace_id, NEW.conversation_id, mailbox_scope,
        'shared', 'shared', shared_version,
        jsonb_build_object('message_id', NEW.id, 'sender_type', NEW.sender_type,
            'is_internal', NEW.is_internal, 'message_type', NEW.message_type),
        COALESCE((SELECT jsonb_agg(user_id) FROM support_conversation_user_states
            WHERE conversation_id = NEW.conversation_id
              AND (unread_customer_message_count > 0 OR manually_unread OR relevance_mask <> 0)), '[]'::jsonb)
    );

    INSERT INTO support_realtime_outbox (
        workspace_id, event_type, entity_id, state_version, payload
    )
    SELECT workspace_id, 'support.message_created', NEW.id, support_state_version,
        jsonb_build_object('conversation_id', NEW.conversation_id, 'reason', 'message_created')
    FROM support_conversations
    WHERE id = NEW.conversation_id;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_message_state_inserted ON support_messages;
CREATE TRIGGER support_message_state_inserted
AFTER INSERT ON support_messages
FOR EACH ROW
EXECUTE FUNCTION project_support_message_state();

CREATE OR REPLACE FUNCTION recompute_support_message_state()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    affected_conversation_id UUID := COALESCE(NEW.conversation_id, OLD.conversation_id);
    conversation_row support_conversations%ROWTYPE;
    list_message support_messages%ROWTYPE;
    public_message support_messages%ROWTYPE;
    customer_message support_messages%ROWTYPE;
    last_response_at TIMESTAMPTZ;
    last_response_id UUID;
    unanswered_count INTEGER := 0;
    mailbox_scope TEXT;
    shared_version BIGINT;
BEGIN
    SELECT * INTO conversation_row
    FROM support_conversations
    WHERE id = affected_conversation_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RETURN COALESCE(NEW, OLD);
    END IF;

    SELECT * INTO list_message
    FROM support_messages
    WHERE conversation_id = affected_conversation_id
      AND deleted_at IS NULL
      AND system_event_type IS NULL
      AND message_type = 'reply'
      AND (NOT is_internal OR TRIM(content) <> '')
    ORDER BY created_at DESC, id DESC
    LIMIT 1;

    SELECT * INTO public_message
    FROM support_messages
    WHERE conversation_id = affected_conversation_id
      AND deleted_at IS NULL
      AND system_event_type IS NULL
      AND message_type = 'reply'
      AND is_internal = FALSE
    ORDER BY created_at DESC, id DESC
    LIMIT 1;

    SELECT * INTO customer_message
    FROM support_messages
    WHERE conversation_id = affected_conversation_id
      AND deleted_at IS NULL
      AND system_event_type IS NULL
      AND message_type = 'reply'
      AND is_internal = FALSE
      AND sender_type = 'customer'
    ORDER BY created_at DESC, id DESC
    LIMIT 1;

    IF public_message.id IS NOT NULL AND public_message.sender_type = 'customer' THEN
        SELECT created_at, id INTO last_response_at, last_response_id
        FROM support_messages
        WHERE conversation_id = affected_conversation_id
          AND deleted_at IS NULL
          AND system_event_type IS NULL
          AND message_type = 'reply'
          AND is_internal = FALSE
          AND sender_type <> 'customer'
        ORDER BY created_at DESC, id DESC
        LIMIT 1;

        SELECT COUNT(*)::INTEGER INTO unanswered_count
        FROM support_messages
        WHERE conversation_id = affected_conversation_id
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
        support_state_version = support_state_version + 1
    WHERE id = affected_conversation_id
    RETURNING * INTO conversation_row;

    -- Message edits/deletes can remove the final historical mention. Keep the
    -- other relevance bits and an explicit personal reminder, but remove Mine
    -- membership and derived unread when mention was the last automatic reason.
    WITH mention_state AS (
        SELECT state.user_id,
               MAX(message.created_at) AS mentioned_at
        FROM support_conversation_user_states state
        LEFT JOIN support_messages message
          ON message.conversation_id = state.conversation_id
         AND message.deleted_at IS NULL
         AND COALESCE(message.metadata, '{}'::jsonb) @>
             jsonb_build_object('mentioned_user_ids', jsonb_build_array(state.user_id::TEXT))
        WHERE state.conversation_id = affected_conversation_id
          AND (state.relevance_mask & 4) <> 0
        GROUP BY state.user_id
    )
    UPDATE support_conversation_user_states state
    SET mentioned_at = mention_state.mentioned_at,
        relevance_mask = CASE WHEN mention_state.mentioned_at IS NULL
            THEN state.relevance_mask & ~4 ELSE state.relevance_mask END,
        unread_customer_message_count = CASE
            WHEN mention_state.mentioned_at IS NULL AND (state.relevance_mask & ~4) = 0 THEN 0
            ELSE state.unread_customer_message_count END,
        version = state.version + 1,
        updated_at = NOW()
    FROM mention_state
    WHERE state.conversation_id = affected_conversation_id
      AND state.user_id = mention_state.user_id
      AND mention_state.mentioned_at IS NULL;

    WITH recalculated AS (
        SELECT state.conversation_id, state.user_id, COUNT(message.id)::INTEGER AS unread_count
        FROM support_conversation_user_states state
        LEFT JOIN support_messages message
          ON message.conversation_id = state.conversation_id
         AND message.deleted_at IS NULL
         AND message.system_event_type IS NULL
         AND message.message_type = 'reply'
         AND message.is_internal = FALSE
         AND message.sender_type = 'customer'
         AND (
              state.last_read_customer_message_at IS NULL
              OR (message.created_at, message.id) > (state.last_read_customer_message_at, state.last_read_customer_message_id)
         )
        WHERE state.conversation_id = affected_conversation_id
          AND state.relevance_mask <> 0
        GROUP BY state.conversation_id, state.user_id
    )
    UPDATE support_conversation_user_states state
    SET unread_customer_message_count = recalculated.unread_count,
        version = state.version + 1,
        updated_at = NOW()
    FROM recalculated
    WHERE state.conversation_id = recalculated.conversation_id
      AND state.user_id = recalculated.user_id
      AND state.relevance_mask <> 0
      AND state.unread_customer_message_count <> recalculated.unread_count;

    mailbox_scope := COALESCE(conversation_row.mailbox_id::TEXT, 'shared');
    INSERT INTO support_inbox_scope_heads (workspace_id, mailbox_scope_id, shared_version, updated_at)
    VALUES (conversation_row.workspace_id, mailbox_scope, 1, NOW())
    ON CONFLICT (workspace_id, mailbox_scope_id)
    DO UPDATE SET shared_version = support_inbox_scope_heads.shared_version + 1, updated_at = NOW()
    RETURNING support_inbox_scope_heads.shared_version INTO shared_version;

    INSERT INTO support_inbox_conversation_changes (
        mutation_id, workspace_id, conversation_id, mailbox_scope_id,
        audience_type, audience_id, source_version, new_values, affected_user_ids
    ) VALUES (
        gen_random_uuid(), conversation_row.workspace_id, affected_conversation_id, mailbox_scope,
        'shared', 'shared', shared_version,
        jsonb_build_object('reason', TG_OP, 'message_id', COALESCE(NEW.id, OLD.id)),
        COALESCE((SELECT jsonb_agg(user_id) FROM support_conversation_user_states
            WHERE conversation_id = affected_conversation_id
              AND (unread_customer_message_count > 0 OR manually_unread OR relevance_mask <> 0)), '[]'::jsonb)
    );

    INSERT INTO support_realtime_outbox (
        workspace_id, event_type, entity_id, state_version, payload
    ) VALUES (
        conversation_row.workspace_id,
        CASE WHEN TG_OP = 'DELETE' OR NEW.deleted_at IS NOT NULL THEN 'support.message_deleted' ELSE 'support.message_updated' END,
        COALESCE(NEW.id, OLD.id)::TEXT,
        conversation_row.support_state_version,
        jsonb_build_object('conversation_id', affected_conversation_id, 'reason', LOWER(TG_OP))
    );

    RETURN COALESCE(NEW, OLD);
END;
$$;

DROP TRIGGER IF EXISTS support_message_state_updated ON support_messages;
CREATE TRIGGER support_message_state_updated
AFTER UPDATE OF deleted_at, is_internal, sender_type, message_type, content, created_at
ON support_messages
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION recompute_support_message_state();

DROP TRIGGER IF EXISTS support_message_state_deleted ON support_messages;
CREATE TRIGGER support_message_state_deleted
AFTER DELETE ON support_messages
FOR EACH ROW
EXECUTE FUNCTION recompute_support_message_state();

CREATE OR REPLACE FUNCTION project_support_conversation_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    old_scope TEXT := COALESCE(OLD.mailbox_id::TEXT, 'shared');
    new_scope TEXT := COALESCE(NEW.mailbox_id::TEXT, 'shared');
    scope TEXT;
    next_version BIGINT;
    conversation_version BIGINT;
    mutation_id UUID := gen_random_uuid();
    recipient_id UUID;
    relevance_bit INTEGER;
    outstanding_unread INTEGER;
BEGIN
    -- Remove only the relevance bit owned by the changed relationship. A
    -- historical mention or the other relationship keeps the user relevant.
    IF OLD.assigned_user_id IS DISTINCT FROM NEW.assigned_user_id AND OLD.assigned_user_id IS NOT NULL THEN
        UPDATE support_conversation_user_states
        SET relevance_mask = relevance_mask & ~1,
            unread_customer_message_count = CASE WHEN (relevance_mask & ~1) = 0 THEN 0 ELSE unread_customer_message_count END,
            version = version + 1,
            updated_at = NOW()
        WHERE conversation_id = NEW.id AND user_id = OLD.assigned_user_id;
    END IF;
    IF OLD.opened_by_user_id IS DISTINCT FROM NEW.opened_by_user_id AND OLD.opened_by_user_id IS NOT NULL THEN
        UPDATE support_conversation_user_states
        SET relevance_mask = relevance_mask & ~2,
            unread_customer_message_count = CASE WHEN (relevance_mask & ~2) = 0 THEN 0 ELSE unread_customer_message_count END,
            version = version + 1,
            updated_at = NOW()
        WHERE conversation_id = NEW.id AND user_id = OLD.opened_by_user_id;
    END IF;

    FOR recipient_id, relevance_bit IN
        SELECT NEW.assigned_user_id, 1 WHERE OLD.assigned_user_id IS DISTINCT FROM NEW.assigned_user_id AND NEW.assigned_user_id IS NOT NULL
        UNION ALL
        SELECT NEW.opened_by_user_id, 2 WHERE OLD.opened_by_user_id IS DISTINCT FROM NEW.opened_by_user_id AND NEW.opened_by_user_id IS NOT NULL
    LOOP
        SELECT COUNT(*)::INTEGER INTO outstanding_unread
        FROM support_messages message
        WHERE message.conversation_id = NEW.id
          AND message.deleted_at IS NULL
          AND message.system_event_type IS NULL
          AND message.message_type = 'reply'
          AND message.is_internal = FALSE
          AND message.sender_type = 'customer'
          AND (
              NOT EXISTS (
                  SELECT 1 FROM support_conversation_user_states existing
                  WHERE existing.conversation_id = NEW.id
                    AND existing.user_id = recipient_id
                    AND existing.last_read_customer_message_at IS NOT NULL
              )
              OR (message.created_at, message.id) > (
                  SELECT existing.last_read_customer_message_at, existing.last_read_customer_message_id
                  FROM support_conversation_user_states existing
                  WHERE existing.conversation_id = NEW.id AND existing.user_id = recipient_id
              )
          )
          AND (
              NOT EXISTS (
                  SELECT 1 FROM support_messages response
                  WHERE response.conversation_id = NEW.id
                    AND response.deleted_at IS NULL
                    AND response.system_event_type IS NULL
                    AND response.message_type = 'reply'
                    AND response.is_internal = FALSE
                    AND response.sender_type <> 'customer'
                    AND (response.created_at, response.id) > (message.created_at, message.id)
              )
          );

        INSERT INTO support_conversation_user_states (
            workspace_id, conversation_id, user_id, unread_customer_message_count,
            relevance_mask, version, created_at, updated_at
        ) VALUES (
            NEW.workspace_id, NEW.id, recipient_id, outstanding_unread,
            relevance_bit, 1, NOW(), NOW()
        )
        ON CONFLICT (conversation_id, user_id)
        DO UPDATE SET
            relevance_mask = support_conversation_user_states.relevance_mask | EXCLUDED.relevance_mask,
            unread_customer_message_count = outstanding_unread,
            version = support_conversation_user_states.version + 1,
            updated_at = NOW();
    END LOOP;

    UPDATE support_conversations
    SET support_state_version = support_state_version + 1,
        view_search_document = LOWER(CONCAT_WS(' ', subject, customer_name, customer_email))
    WHERE id = NEW.id
    RETURNING support_state_version INTO conversation_version;

    FOREACH scope IN ARRAY ARRAY[old_scope, new_scope]
    LOOP
        IF scope = new_scope OR old_scope <> new_scope THEN
            INSERT INTO support_inbox_scope_heads (workspace_id, mailbox_scope_id, shared_version, updated_at)
            VALUES (NEW.workspace_id, scope, 1, NOW())
            ON CONFLICT (workspace_id, mailbox_scope_id)
            DO UPDATE SET shared_version = support_inbox_scope_heads.shared_version + 1, updated_at = NOW()
            RETURNING shared_version INTO next_version;

            INSERT INTO support_inbox_conversation_changes (
                mutation_id, workspace_id, conversation_id, mailbox_scope_id,
                audience_type, audience_id, source_version, old_values, new_values, affected_user_ids
            ) VALUES (
                mutation_id, NEW.workspace_id, NEW.id, scope, 'shared', 'shared', next_version,
                to_jsonb(OLD), to_jsonb(NEW),
                COALESCE((SELECT jsonb_agg(user_id) FROM support_conversation_user_states
                    WHERE conversation_id = NEW.id
                      AND (unread_customer_message_count > 0 OR manually_unread OR relevance_mask <> 0)), '[]'::jsonb)
            );
        END IF;
        EXIT WHEN old_scope = new_scope;
    END LOOP;

    INSERT INTO support_realtime_outbox (
        workspace_id, event_type, entity_id, state_version, payload
    ) VALUES (
        NEW.workspace_id, 'support.conversation_state_changed', NEW.id::TEXT, conversation_version,
        jsonb_build_object(
            'status', NEW.status, 'old_status', OLD.status,
            'mailbox_id', NEW.mailbox_id, 'flow_state', NEW.flow_state,
            'customer_awaiting_response', NEW.customer_awaiting_response,
            'needs_human_reply', NEW.needs_human_reply,
            'updated_at', NEW.updated_at
        )
    );

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_conversation_state_updated ON support_conversations;
CREATE TRIGGER support_conversation_state_updated
AFTER UPDATE OF status, mailbox_id, assigned_user_id, opened_by_user_id,
    flow_state, ai_state, human_takeover, subject, customer_name, customer_email
ON support_conversations
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION project_support_conversation_change();
