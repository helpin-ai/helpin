-- Conversation-analysis findings could attach to gaps created straight from
-- support events ("Human resolved after AI failure" and similar rule
-- placeholders). Those gaps are now hidden and excluded from matching, so
-- release the findings: drop their evidence rows from raw gaps and mark the
-- analyses unmaterialized. The next analysis run turns them into gaps titled
-- by the customer need. Running it again finds nothing to change.
DELETE FROM support_gap_evidence e
USING support_coverage_gaps g
WHERE e.gap_id = g.id
  AND e.evidence_type = 'daily_conversation_analysis'
  AND COALESCE(g.metadata ->> 'source', '') = 'event_detection';

UPDATE support_coverage_conversation_analyses a
SET gap_id = NULL,
    updated_at = NOW()
FROM support_coverage_gaps g
WHERE a.gap_id = g.id
  AND COALESCE(g.metadata ->> 'source', '') = 'event_detection';
