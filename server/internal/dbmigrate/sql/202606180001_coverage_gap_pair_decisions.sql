ALTER TABLE support_coverage_gap_merge_suggestions
  ADD COLUMN IF NOT EXISTS pair_key text NOT NULL DEFAULT '';

UPDATE support_coverage_gap_merge_suggestions
SET pair_key = LEAST(source_gap_id::text, target_gap_id::text) || ':' || GREATEST(source_gap_id::text, target_gap_id::text)
WHERE pair_key = '';

DROP INDEX IF EXISTS idx_support_coverage_gap_merge_suggestions_pending_pair;

WITH ranked_pending_pairs AS (
  SELECT
    id,
    ROW_NUMBER() OVER (
      PARTITION BY workspace_id, pair_key
      ORDER BY created_at DESC, id DESC
    ) AS row_number
  FROM support_coverage_gap_merge_suggestions
  WHERE status = 'pending' AND pair_key != ''
)
DELETE FROM support_coverage_gap_merge_suggestions
WHERE id IN (
  SELECT id
  FROM ranked_pending_pairs
  WHERE row_number > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gap_merge_suggestions_pending_pair_key
  ON support_coverage_gap_merge_suggestions(workspace_id, pair_key)
  WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS support_coverage_gap_pair_decisions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  pair_key text NOT NULL,
  gap_a_id uuid NOT NULL,
  gap_b_id uuid NOT NULL,
  decision text NOT NULL DEFAULT 'keep_separate',
  decided_by uuid,
  decided_at timestamptz NOT NULL DEFAULT now(),
  similarity_at_decision double precision NOT NULL DEFAULT 0,
  gap_a_text_hash text NOT NULL DEFAULT '',
  gap_b_text_hash text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gap_pair_decisions_workspace_pair
  ON support_coverage_gap_pair_decisions(workspace_id, pair_key);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gap_pair_decisions_workspace_decision
  ON support_coverage_gap_pair_decisions(workspace_id, decision);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gap_pair_decisions_gap_a
  ON support_coverage_gap_pair_decisions(gap_a_id);

CREATE INDEX IF NOT EXISTS idx_support_coverage_gap_pair_decisions_gap_b
  ON support_coverage_gap_pair_decisions(gap_b_id);

INSERT INTO support_coverage_gap_pair_decisions (
  workspace_id,
  pair_key,
  gap_a_id,
  gap_b_id,
  decision,
  decided_by,
  decided_at,
  similarity_at_decision,
  gap_a_text_hash,
  gap_b_text_hash,
  created_at,
  updated_at
)
SELECT
  dismissed.workspace_id,
  dismissed.pair_key,
  LEAST(dismissed.source_gap_id::text, dismissed.target_gap_id::text)::uuid,
  GREATEST(dismissed.source_gap_id::text, dismissed.target_gap_id::text)::uuid,
  'keep_separate',
  dismissed.reviewed_by,
  COALESCE(dismissed.reviewed_at, dismissed.updated_at, dismissed.created_at, now()),
  dismissed.similarity_score,
  COALESCE(gap_a.embedding_text_hash, ''),
  COALESCE(gap_b.embedding_text_hash, ''),
  now(),
  now()
FROM (
  SELECT DISTINCT ON (workspace_id, pair_key) *
  FROM support_coverage_gap_merge_suggestions
  WHERE status = 'dismissed' AND pair_key != ''
  ORDER BY workspace_id, pair_key, reviewed_at DESC NULLS LAST, updated_at DESC, created_at DESC
) dismissed
LEFT JOIN support_coverage_gaps gap_a
  ON gap_a.id = LEAST(dismissed.source_gap_id::text, dismissed.target_gap_id::text)::uuid
LEFT JOIN support_coverage_gaps gap_b
  ON gap_b.id = GREATEST(dismissed.source_gap_id::text, dismissed.target_gap_id::text)::uuid
ON CONFLICT (workspace_id, pair_key) DO NOTHING;
