-- Use the product name in the inbox while retaining provider/model audit metadata.
-- Existing automatic tag events are recognizable by their type and provenance.
UPDATE support_messages
SET sender_display_name = 'Helpin AI',
    content = 'Helpin AI' || substring(content FROM 4)
WHERE sender_type = 'ai'
  AND message_type = 'system'
  AND system_event_type = 'tag_added'
  AND metadata->>'provider' = 'typesafe'
  AND sender_display_name = 'Jev'
  AND content LIKE 'Jev added tag %';

UPDATE support_conversation_triage
SET reason = 'Helpin AI matched this conversation to the inbox.'
WHERE reason = 'Jev jev-1.13.0; provider probability (not locally calibrated)';
