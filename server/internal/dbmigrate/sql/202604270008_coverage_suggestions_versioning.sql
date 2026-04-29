ALTER TABLE support_gap_suggestions
  ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS superseded_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_support_gap_suggestions_gap_active
  ON support_gap_suggestions(gap_id)
  WHERE is_active;
