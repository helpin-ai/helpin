-- One-time backfill of system_event_type for historic support_messages rows.
-- Classifies rows by content with high-confidence keyword rules. Ambiguous
-- rows stay NULL and fall through the legacy content-match branch on the
-- admin MessageBubble until they age out.
-- Plan: docs/plans/2026-04-15-system-message-event-type-plan.md

-- Resolved / reopened / closed labels emitted by SupportInboxService.
UPDATE support_messages
SET system_event_type = 'resolved'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND content ILIKE '%resolved%';

UPDATE support_messages
SET system_event_type = 'reopened'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND content ILIKE '%reopened%';

UPDATE support_messages
SET system_event_type = 'closed'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND (content ILIKE '%closed%' OR content ILIKE '%marked as closed%');

-- Assignment family — match new copy produced by emitAssignmentSystemMessage.
UPDATE support_messages
SET system_event_type = 'took'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND content ILIKE '% took this conversation%';

UPDATE support_messages
SET system_event_type = 'unassigned'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND content ILIKE '%moved this conversation to unassigned%';

UPDATE support_messages
SET system_event_type = 'assigned'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND (content ILIKE '% assigned this conversation to %' OR content ILIKE 'Assigned to %');

-- Teammate joined — Intercom-style first-reply pill.
UPDATE support_messages
SET system_event_type = 'teammate_joined'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND content ILIKE '% joined the conversation';

-- Mailbox move ("Moved to X by Y").
UPDATE support_messages
SET system_event_type = 'mailbox_moved'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND content ILIKE 'Moved to % by %';

-- Triage routing / dismissals.
UPDATE support_messages
SET system_event_type = 'triage_dismissed'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND content ILIKE '%Routing suggestion dismissed%';

UPDATE support_messages
SET system_event_type = 'triage_routed'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND sender_type = 'ai';

-- AI escalation ("Let me connect you with a team member...").
UPDATE support_messages
SET system_event_type = 'ai_escalated'
WHERE message_type = 'system'
  AND system_event_type IS NULL
  AND sender_type = 'agent'
  AND content ILIKE '%connect you with a team member%';
