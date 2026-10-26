-- Coverage health, retry claims and manual reassessment all need the newest
-- attempt for a source, across transcript hashes and lifecycle segments.
CREATE INDEX IF NOT EXISTS idx_coverage_analysis_attempt_source_history
  ON coverage_analysis_attempts
    (workspace_id, source_kind, source_id, created_at DESC, attempt DESC);
