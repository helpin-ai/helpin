ALTER TABLE support_coverage_gaps
  ADD COLUMN IF NOT EXISTS gap_kind text NOT NULL DEFAULT 'content',
  ADD COLUMN IF NOT EXISTS closed_at timestamptz,
  ADD COLUMN IF NOT EXISTS closed_evidence_count int,
  ADD COLUMN IF NOT EXISTS result_document_id uuid,
  ADD COLUMN IF NOT EXISTS rejection_reason text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gaps_workspace_topic_open
  ON support_coverage_gaps(workspace_id, topic_id)
  WHERE status = 'open' AND topic_id IS NOT NULL;

DROP INDEX IF EXISTS idx_support_coverage_gaps_workspace_dedupe;

CREATE INDEX IF NOT EXISTS idx_support_coverage_gaps_workspace_dedupe
  ON support_coverage_gaps(workspace_id, dedupe_key);
