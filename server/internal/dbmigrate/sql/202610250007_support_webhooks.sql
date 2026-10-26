-- Outbound support webhooks: endpoints, an event outbox filled by triggers,
-- and durable deliveries with retries.

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    url text NOT NULL,
    description text NOT NULL DEFAULT '',
    events text[] NOT NULL DEFAULT '{}',
    mailbox_ids text[] NOT NULL DEFAULT '{}',
    encrypted_secret text NOT NULL,
    secret_hint text NOT NULL DEFAULT '',
    enabled boolean NOT NULL,
    disabled_reason text NOT NULL DEFAULT '',
    failure_count integer NOT NULL DEFAULT 0,
    last_success_at timestamptz,
    last_failure_at timestamptz,
    created_by_id uuid,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_workspace_id
    ON webhook_endpoints (workspace_id);

CREATE TABLE IF NOT EXISTS webhook_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    event_type text NOT NULL,
    conversation_id uuid,
    message_id uuid,
    data jsonb NOT NULL DEFAULT '{}',
    available_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_available_at
    ON webhook_events (available_at);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    endpoint_id uuid NOT NULL,
    event_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL,
    last_status_code integer,
    last_error text NOT NULL DEFAULT '',
    last_attempt_at timestamptz,
    duration_ms integer,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint
    ON webhook_deliveries (endpoint_id, created_at);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_due
    ON webhook_deliveries (status, next_attempt_at);

-- Record an event when an enabled endpoint of the workspace subscribes to it.
CREATE OR REPLACE FUNCTION webhook_enqueue_event(
    target_workspace_id uuid,
    target_event_type text,
    target_conversation_id uuid,
    target_message_id uuid,
    event_data jsonb,
    event_available_at timestamptz
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM webhook_endpoints
        WHERE workspace_id = target_workspace_id AND enabled
          AND (cardinality(events) = 0 OR target_event_type = ANY(events))
    ) THEN
        INSERT INTO webhook_events (workspace_id, event_type, conversation_id, message_id, data, available_at, created_at)
        VALUES (target_workspace_id, target_event_type, target_conversation_id, target_message_id,
                COALESCE(event_data, '{}'::jsonb), GREATEST(NOW(), COALESCE(event_available_at, NOW())), NOW());
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION webhook_conversation_inserted()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    -- Imported history and merge copies are not news.
    IF NEW.merged_into_id IS NULL AND NEW.anonymized_at IS NULL AND NEW.created_at > NOW() - INTERVAL '1 hour' THEN
        PERFORM webhook_enqueue_event(NEW.workspace_id, 'conversation.created', NEW.id, NULL, '{}'::jsonb, NULL);
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS webhook_conversation_inserted ON support_conversations;
CREATE TRIGGER webhook_conversation_inserted
AFTER INSERT ON support_conversations
FOR EACH ROW
EXECUTE FUNCTION webhook_conversation_inserted();

CREATE OR REPLACE FUNCTION webhook_conversation_updated()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.anonymized_at IS NOT NULL THEN
        RETURN NEW;
    END IF;
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        PERFORM webhook_enqueue_event(NEW.workspace_id, 'conversation.status_changed', NEW.id, NULL,
            jsonb_build_object('changes', jsonb_build_object('status', jsonb_build_object('from', OLD.status, 'to', NEW.status))), NULL);
    END IF;
    IF OLD.assigned_user_id IS DISTINCT FROM NEW.assigned_user_id THEN
        PERFORM webhook_enqueue_event(NEW.workspace_id, 'conversation.assigned', NEW.id, NULL,
            jsonb_build_object('changes', jsonb_build_object('assignee_id', jsonb_build_object('from', OLD.assigned_user_id, 'to', NEW.assigned_user_id))), NULL);
    END IF;
    IF OLD.priority IS DISTINCT FROM NEW.priority THEN
        PERFORM webhook_enqueue_event(NEW.workspace_id, 'conversation.priority_changed', NEW.id, NULL,
            jsonb_build_object('changes', jsonb_build_object('priority', jsonb_build_object('from', OLD.priority, 'to', NEW.priority))), NULL);
    END IF;
    IF NEW.snoozed_until IS NOT NULL AND OLD.snoozed_until IS DISTINCT FROM NEW.snoozed_until THEN
        PERFORM webhook_enqueue_event(NEW.workspace_id, 'conversation.snoozed', NEW.id, NULL,
            jsonb_build_object('snoozed_until', NEW.snoozed_until), NULL);
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS webhook_conversation_updated ON support_conversations;
CREATE TRIGGER webhook_conversation_updated
AFTER UPDATE OF status, assigned_user_id, priority, snoozed_until ON support_conversations
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status
    OR OLD.assigned_user_id IS DISTINCT FROM NEW.assigned_user_id
    OR OLD.priority IS DISTINCT FROM NEW.priority
    OR (NEW.snoozed_until IS NOT NULL AND OLD.snoozed_until IS DISTINCT FROM NEW.snoozed_until))
EXECUTE FUNCTION webhook_conversation_updated();

-- Public replies; a reply inside its undo window waits for the window to end.
-- Imported history is skipped.
CREATE OR REPLACE FUNCTION webhook_message_inserted()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM webhook_enqueue_event(NEW.workspace_id, 'message.created', NEW.conversation_id, NEW.id, '{}'::jsonb, NEW.cancellable_until);
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS webhook_message_inserted ON support_messages;
CREATE TRIGGER webhook_message_inserted
AFTER INSERT ON support_messages
FOR EACH ROW
WHEN (NEW.message_type = 'reply' AND NOT NEW.is_internal AND NEW.system_event_type IS NULL
    AND NEW.sender_type IN ('customer', 'user', 'ai', 'agent') AND NEW.deleted_at IS NULL
    AND NEW.created_at > NOW() - INTERVAL '1 hour')
EXECUTE FUNCTION webhook_message_inserted();
