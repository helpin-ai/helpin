-- Migration: support_coverage_cluster_rebuild
-- Adds durable audit records for coverage gap cluster rebuilds and merge suggestions.

CREATE TABLE IF NOT EXISTS support_coverage_cluster_rebuild_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  status text NOT NULL DEFAULT 'running',
  gaps_scanned integer NOT NULL DEFAULT 0,
  clusters_found integer NOT NULL DEFAULT 0,
  auto_merged integer NOT NULL DEFAULT 0,
  suggestions_created integer NOT NULL DEFAULT 0,
  skipped integer NOT NULL DEFAULT 0,
  error_message text,
  started_at timestamptz NOT NULL,
  completed_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_coverage_cluster_rebuild_runs_workspace
  ON support_coverage_cluster_rebuild_runs(workspace_id, started_at DESC);

CREATE TABLE IF NOT EXISTS support_coverage_gap_merge_suggestions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  run_id uuid,
  source_gap_id uuid NOT NULL,
  target_gap_id uuid NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  similarity_score double precision NOT NULL DEFAULT 0,
  reason text NOT NULL DEFAULT '',
  combined_evidence_count integer NOT NULL DEFAULT 0,
  reviewed_by uuid,
  reviewed_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gap_merge_suggestions_workspace_status
  ON support_coverage_gap_merge_suggestions(workspace_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gap_merge_suggestions_source
  ON support_coverage_gap_merge_suggestions(source_gap_id);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gap_merge_suggestions_target
  ON support_coverage_gap_merge_suggestions(target_gap_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gap_merge_suggestions_pending_pair
  ON support_coverage_gap_merge_suggestions(workspace_id, source_gap_id, target_gap_id)
  WHERE status = 'pending';
