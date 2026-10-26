-- Older redeliveries copied the payload without its conversation link.
-- Restore that link so contact anonymization removes every copy.
UPDATE webhook_deliveries
SET conversation_id = (payload->'data'->'conversation'->>'id')::uuid
WHERE conversation_id IS NULL
    AND payload->'data'->'conversation'->>'id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

-- Anonymization may already have run while these copies were unlinked.
DELETE FROM webhook_deliveries d
USING support_conversations c
WHERE d.workspace_id = c.workspace_id AND d.conversation_id = c.id
    AND c.anonymized_at IS NOT NULL;
