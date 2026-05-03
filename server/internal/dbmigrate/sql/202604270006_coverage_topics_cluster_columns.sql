ALTER TABLE support_coverage_topics
  ADD COLUMN IF NOT EXISTS cluster_key text,
  ADD COLUMN IF NOT EXISTS canonical_title text,
  ADD COLUMN IF NOT EXISTS last_enriched_at timestamptz,
  ADD COLUMN IF NOT EXISTS cooldown_until timestamptz;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_topics_workspace_cluster_key
  ON support_coverage_topics(workspace_id, cluster_key)
  WHERE cluster_key IS NOT NULL;
