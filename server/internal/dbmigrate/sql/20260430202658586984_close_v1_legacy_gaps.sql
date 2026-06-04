-- Migration: close_v1_legacy_gaps
-- Close noisy v1 legacy gaps that have no actionable information.
-- These were created by heuristic event rules (human_reply, conversation_resolved_by_human)
-- before the daily LLM analyzer was active. They have no topic, no analysis explanation,
-- no recommendations, and confidence 0.3-0.4.
UPDATE support_coverage_gaps
SET status = 'done',
    closed_at = NOW(),
    updated_at = NOW()
WHERE source_signal IN ('human_reply', 'conversation_resolved_by_human')
  AND gap_category = 'unknown'
  AND topic_id IS NULL
  AND status = 'open';
