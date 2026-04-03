-- Add workspace_key column to workspaces (nullable first for backfill)
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS workspace_key VARCHAR(5);

-- Backfill existing workspaces using alpha-only strategy.
-- Try progressively longer substrings from slug (3, 4, 5 chars),
-- then append alpha suffixes (A-Z) at each length, all within VARCHAR(5).
DO $$
DECLARE
  ws RECORD;
  alpha_slug TEXT;
  candidate TEXT;
  base_len INT;
  suffix_char INT;
  found BOOLEAN;
BEGIN
  FOR ws IN SELECT id, slug FROM workspaces WHERE workspace_key IS NULL ORDER BY created_at LOOP
    -- Strip non-alpha, uppercase
    alpha_slug := UPPER(REGEXP_REPLACE(ws.slug, '[^a-zA-Z]', '', 'g'));
    IF LENGTH(alpha_slug) < 2 THEN
      alpha_slug := 'WS';
    END IF;

    found := FALSE;

    -- Try bare substrings first: 3, 4, 5 chars
    FOR base_len IN 3..LEAST(LENGTH(alpha_slug), 5) LOOP
      candidate := LEFT(alpha_slug, base_len);
      IF NOT EXISTS (SELECT 1 FROM workspaces WHERE workspace_key = candidate AND id != ws.id) THEN
        found := TRUE;
        EXIT;
      END IF;
    END LOOP;

    -- If bare substrings all collide, try alpha suffixes within 5-char limit
    IF NOT found THEN
      FOR base_len IN 2..4 LOOP
        FOR suffix_char IN 65..90 LOOP  -- A=65, Z=90
          candidate := LEFT(alpha_slug, base_len) || CHR(suffix_char);
          IF LENGTH(candidate) <= 5 AND NOT EXISTS (
            SELECT 1 FROM workspaces WHERE workspace_key = candidate AND id != ws.id
          ) THEN
            found := TRUE;
            EXIT;
          END IF;
        END LOOP;
        EXIT WHEN found;
      END LOOP;
    END IF;

    -- Final fallback
    IF NOT found THEN
      candidate := LEFT(alpha_slug, 2) || 'X';
    END IF;

    UPDATE workspaces SET workspace_key = candidate WHERE id = ws.id;
  END LOOP;
END $$;

-- Now make it NOT NULL and add unique index
ALTER TABLE workspaces ALTER COLUMN workspace_key SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspaces_workspace_key ON workspaces (workspace_key);

-- Add workspace_key_history table for alias resolution on key changes
CREATE TABLE IF NOT EXISTS workspace_key_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    old_key VARCHAR(5) NOT NULL,
    new_key VARCHAR(5) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_by UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_wkh_workspace_id ON workspace_key_history (workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wkh_old_key ON workspace_key_history (old_key);
