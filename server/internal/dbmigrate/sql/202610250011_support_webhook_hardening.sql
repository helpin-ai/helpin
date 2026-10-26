-- Webhook hardening: replies wait out their undo window, endpoints turn off
-- only after a day of continuous failures, and deliveries can be purged
-- when a customer's data is deleted.

ALTER TABLE webhook_endpoints
    ADD COLUMN IF NOT EXISTS failing_since timestamptz;

ALTER TABLE webhook_deliveries
    ADD COLUMN IF NOT EXISTS conversation_id uuid;

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_conversation_id
    ON webhook_deliveries (conversation_id);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at
    ON webhook_deliveries (created_at);

-- The undo deadline is stamped just after a reply is inserted; holding
-- message events briefly lets dispatch see it and wait.
CREATE OR REPLACE FUNCTION webhook_message_inserted()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM webhook_enqueue_event(NEW.workspace_id, 'message.created', NEW.conversation_id, NEW.id, '{}'::jsonb,
        GREATEST(NOW() + INTERVAL '10 seconds', COALESCE(NEW.cancellable_until, NOW())));
    RETURN NEW;
END;
$$;
