-- Restore only externally authored email replies hidden by the former
-- unknown-sender rule. No outbound delivery or AI processing is enqueued.
CREATE TEMP TABLE restored_email_participants ON COMMIT DROP AS
SELECT DISTINCT m.id, m.workspace_id, m.conversation_id,
       COALESCE(m.origin_conversation_id, m.conversation_id) AS recipient_conversation_id,
       lower(btrim(m.metadata->>'email_sender')) AS sender_email,
       m.sender_display_name
FROM support_messages m
JOIN support_conversations c ON c.id = m.conversation_id AND c.workspace_id = m.workspace_id
JOIN support_conversations origin ON origin.id = COALESCE(m.origin_conversation_id, m.conversation_id)
    AND origin.workspace_id = m.workspace_id
WHERE m.is_internal = true AND m.sender_type = 'customer'
  AND m.sender_user_id IS NULL AND m.message_type = 'reply' AND m.via_channel = 'email'
  AND m.deleted_at IS NULL AND c.anonymized_at IS NULL AND origin.anonymized_at IS NULL
  AND m.metadata->>'email_unknown_sender' = 'true'
  AND EXISTS (
      SELECT 1 FROM support_email_logs l
      WHERE l.workspace_id = m.workspace_id AND l.direction = 'inbound'
        AND l.conversation_id IN (m.conversation_id, COALESCE(m.origin_conversation_id, m.conversation_id))
        AND m.id::text = ANY(l.message_ids)
        AND lower(btrim(l.from_email)) = lower(btrim(m.metadata->>'email_sender'))
  )
  AND NOT EXISTS (
      SELECT 1 FROM workspace_members member
      WHERE member.workspace_id = m.workspace_id
        AND lower(btrim(member.email)) = lower(btrim(m.metadata->>'email_sender'))
  );

-- Serialize recipient changes with live replies and manual CC edits.
SELECT c.id FROM support_conversations c
WHERE c.id IN (SELECT recipient_conversation_id FROM restored_email_participants)
ORDER BY c.id FOR UPDATE;

CREATE TEMP TABLE restored_email_visible_recipients ON COMMIT DROP AS
SELECT DISTINCT p.workspace_id, p.recipient_conversation_id AS conversation_id,
       lower(btrim(address.value)) AS email
FROM restored_email_participants p
JOIN support_email_logs l ON l.workspace_id = p.workspace_id AND p.id::text = ANY(l.message_ids)
    AND l.direction = 'inbound' AND lower(btrim(l.from_email)) = p.sender_email
    AND l.conversation_id IN (p.conversation_id, p.recipient_conversation_id)
JOIN support_conversations c ON c.id = p.recipient_conversation_id AND c.workspace_id = p.workspace_id
CROSS JOIN LATERAL (
    SELECT p.sender_email AS value
    UNION SELECT unnest(l.cc_emails)
    UNION SELECT matches[1] FROM regexp_matches(COALESCE(l.to_email, ''),
        $email$([A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,})$email$, 'g') AS matches
) address
-- Merged conversations retain their explicitly reviewed email audience.
WHERE c.merge_revision = 0 AND c.merged_into_id IS NULL
  AND btrim(address.value) <> ''
  AND lower(btrim(address.value)) <> lower(COALESCE(c.customer_email, ''))
  AND lower(btrim(address.value)) <> lower(COALESCE(l.recipient_address, ''))
  AND NOT EXISTS (
      SELECT 1 FROM support_email_routes r WHERE r.workspace_id = p.workspace_id
      AND lower(btrim(address.value)) IN (lower(r.inbound_address), lower(r.source_address))
  )
  AND NOT EXISTS (
      SELECT 1 FROM support_email_senders s WHERE s.workspace_id = p.workspace_id
      AND lower(btrim(address.value)) IN (lower(s.email), lower(s.forwarding_address))
  );

CREATE TEMP TABLE restored_email_cc_additions ON COMMIT DROP AS
SELECT v.* FROM restored_email_visible_recipients v
JOIN support_conversations c ON c.id = v.conversation_id AND c.workspace_id = v.workspace_id
WHERE NOT EXISTS (SELECT 1 FROM unnest(c.email_cc) e WHERE lower(btrim(e)) = v.email)
  -- A teammate may have explicitly removed a previously seen recipient.
  AND NOT EXISTS (SELECT 1 FROM unnest(c.email_thread_participants) e WHERE lower(btrim(e)) = v.email);

UPDATE support_conversations c
SET email_cc = ARRAY(
        SELECT lower(btrim(e)) FROM unnest(c.email_cc) e
        UNION SELECT a.email FROM restored_email_cc_additions a WHERE a.conversation_id = c.id
        ORDER BY 1),
    email_thread_participants = ARRAY(
        SELECT lower(btrim(e)) FROM unnest(c.email_thread_participants) e
        UNION SELECT v.email FROM restored_email_visible_recipients v WHERE v.conversation_id = c.id
        ORDER BY 1),
    updated_at = now()
WHERE c.id IN (SELECT recipient_conversation_id FROM restored_email_participants);

-- Updating is_internal invokes the existing inbox projection triggers, which
-- repair last-customer-message and unanswered/read-state calculations.
-- Keep original content, author, timestamps, ownership and resolved status.
UPDATE support_messages m
SET is_internal = false,
    metadata = (m.metadata - 'email_unknown_sender') || jsonb_build_object(
        'email_participant_sender', true,
        'email_participant_repaired', true,
        'email_ai_request', false,
        'email_ai_suppression_reason', COALESCE(NULLIF(m.metadata->>'email_ai_suppression_reason', ''), 'historical_participant_repair'))
WHERE m.id IN (SELECT id FROM restored_email_participants);

INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type,
    system_event_type, is_internal, metadata, content, created_at, updated_at)
SELECT gen_random_uuid(), a.workspace_id, a.conversation_id, 'system', 'system',
    'email_recipients_updated', true, '{"historical_participant_repair":true}'::jsonb,
    string_agg(COALESCE(NULLIF((SELECT max(p.sender_display_name)
        FROM restored_email_participants p WHERE p.workspace_id = a.workspace_id
        AND p.recipient_conversation_id = a.conversation_id AND p.sender_email = a.email), '')
        || ' (' || a.email || ')', a.email) || ' was added to CC.', ' ' ORDER BY a.email), now(), now()
FROM restored_email_cc_additions a GROUP BY a.workspace_id, a.conversation_id;
