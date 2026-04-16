-- Fix workspace_skill unique index: allow multiple archived rows for the same key.
-- The old index (workspace_id, key, is_archived) prevents archive → recreate → archive-again
-- because only one archived row per (workspace_id, key) is allowed.
-- The new partial index enforces uniqueness only for active (non-archived) rows.

DROP INDEX IF EXISTS idx_workspace_skill_key;

DO $$
BEGIN
    IF to_regclass('public.workspace_skills') IS NOT NULL THEN
        EXECUTE '
            CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_skill_key
                ON workspace_skills (workspace_id, key)
                WHERE is_archived = false
        ';
    END IF;
END
$$;
