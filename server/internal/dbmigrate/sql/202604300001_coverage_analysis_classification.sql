-- Add classification fields to support_coverage_conversation_analyses
-- for filtering non-support emails (newsletters, cold outreach, auto-replies, etc.)
-- from coverage gap detection.

ALTER TABLE support_coverage_conversation_analyses
  ADD COLUMN IF NOT EXISTS is_support_query boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS conversation_type text NOT NULL DEFAULT 'support_query',
  ADD COLUMN IF NOT EXISTS classification_reason text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_type
  ON support_coverage_conversation_analyses(workspace_id, conversation_type, created_at DESC);
