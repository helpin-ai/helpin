CREATE OR REPLACE FUNCTION support_channel_outbox_enqueue()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    transport text := COALESCE(NEW.metadata->>'reply_transport', '');
    selected_origin uuid;
    identity_id UUID;
    account_id UUID;
    account_live BOOLEAN;
BEGIN
    IF transport IN ('slack', 'telegram') THEN
        -- Validate and freeze the reviewed destination inside the message transaction.
        WITH RECURSIVE members AS (
            SELECT id FROM support_conversations
             WHERE workspace_id = NEW.workspace_id AND id = NEW.conversation_id
            UNION
            SELECT c.id FROM support_conversations c JOIN members m ON c.merged_into_id = m.id
             WHERE c.workspace_id = NEW.workspace_id
        )
        SELECT c.id, c.channel_identity_id INTO selected_origin, identity_id
          FROM members m JOIN support_conversations c ON c.id = m.id
         WHERE c.id::text = NEW.metadata->>'provider_origin_id' AND c.channel = transport
           AND c.anonymized_at IS NULL;
        IF identity_id IS NULL THEN
            RAISE EXCEPTION 'The selected reply destination is unavailable';
        END IF;
    ELSE
        SELECT c.channel_identity_id INTO identity_id
          FROM support_conversations c
         WHERE c.id = NEW.conversation_id AND c.workspace_id = NEW.workspace_id;
        -- In a merged conversation, reply to the person behind the latest
        -- provider message routed into it.
        IF identity_id IS NULL THEN
            SELECT origin.channel_identity_id INTO identity_id
              FROM support_messages m
              JOIN support_conversations origin ON origin.id = COALESCE(m.origin_conversation_id, m.conversation_id)
             WHERE m.conversation_id = NEW.conversation_id AND m.workspace_id = NEW.workspace_id
               AND m.sender_type = 'customer' AND m.channel_account_id IS NOT NULL AND m.deleted_at IS NULL
               AND origin.channel_identity_id IS NOT NULL
             ORDER BY m.created_at DESC
             LIMIT 1;
        END IF;
    END IF;
    IF identity_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT i.channel_account_id, (a.disabled_at IS NULL AND a.status = 'active')
      INTO account_id, account_live
      FROM support_channel_identities i
      JOIN support_channel_accounts a ON a.id = i.channel_account_id
     WHERE i.id = identity_id;
    IF account_id IS NULL THEN
        RETURN NEW;
    END IF;
    -- created_at orders a customer's replies. clock_timestamp(), unlike now(),
    -- differs between replies saved in one transaction.
    INSERT INTO support_channel_outbox (
        workspace_id, channel_account_id, channel_identity_id, conversation_id, message_id, provider_origin_id, semantic_key,
        dispatch_status, last_error_code, next_attempt_at, created_at
    ) VALUES (
        NEW.workspace_id, account_id, identity_id, NEW.conversation_id, NEW.id, selected_origin, 'message:' || NEW.id,
        -- A disconnected bot cannot send; the row shows the reply as not delivered.
        CASE WHEN account_live THEN 'pending' ELSE 'cancelled' END,
        CASE WHEN account_live THEN NULL ELSE 'disconnected' END,
        -- A reply inside its undo window waits for the window to end.
        GREATEST(NOW(), COALESCE(NEW.cancellable_until, NOW())),
        clock_timestamp()
    )
    ON CONFLICT (semantic_key) DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_channel_outbox_enqueue ON support_messages;
CREATE TRIGGER support_channel_outbox_enqueue
AFTER INSERT ON support_messages
FOR EACH ROW
WHEN (NEW.message_type = 'reply' AND NOT NEW.is_internal AND NEW.system_event_type IS NULL
    AND NEW.sender_type IN ('user', 'ai', 'agent') AND NEW.deleted_at IS NULL
    -- Messages that came from the provider are never sent back to it.
    AND NEW.external_message_key IS NULL
    AND COALESCE(NEW.metadata->>'imported_from', '') = ''
    AND COALESCE(NEW.metadata->>'delivery_mode', '') <> 'email_only'
    AND COALESCE(NEW.metadata->>'reply_transport', '') IN ('', 'slack', 'telegram'))
EXECUTE FUNCTION support_channel_outbox_enqueue();
