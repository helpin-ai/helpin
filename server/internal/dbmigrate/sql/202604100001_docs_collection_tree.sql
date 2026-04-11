-- Migration: docs_collection_tree
-- Turns docs_collections into a bounded tree and tightens slug uniqueness.
--
-- Adds parent_collection_id + depth columns.
-- Dedupes duplicate (workspace_id, slug) pairs on live rows before creating a
-- partial unique index. The oldest row (by created_at, then id) keeps its
-- original slug; later rows get suffixed to {slug}-2, {slug}-3, ... No
-- redirects are emitted for the renamed losers because the original slug
-- stays canonical for the kept row.
--
-- Replaces the legacy flat ordering index idx_docs_collection_space_pos with
-- a tree-aware variant on (space_id, parent_collection_id, position) plus a
-- standalone parent lookup index.
--
-- Idempotent: safe to re-run.

-- Step 1: add the new columns if they don't already exist.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'docs_collections'
          AND column_name = 'parent_collection_id'
    ) THEN
        ALTER TABLE docs_collections
            ADD COLUMN parent_collection_id uuid NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'docs_collections'
          AND column_name = 'depth'
    ) THEN
        ALTER TABLE docs_collections
            ADD COLUMN depth integer NOT NULL DEFAULT 0;
    END IF;
END $$;

-- Step 2: dedupe duplicate (workspace_id, slug) pairs on live rows.
--
-- For each duplicate group we keep the oldest row on the original slug and
-- rename the rest to {slug}-2, {slug}-3, ... The inner loop advances n until
-- it finds a free suffix in the workspace, which handles the edge case
-- where {slug}-2 already exists as an unrelated collection.
DO $$
DECLARE
    dup RECORD;
    loser RECORD;
    new_slug TEXT;
    n INTEGER;
BEGIN
    FOR dup IN
        SELECT workspace_id, slug
        FROM docs_collections
        WHERE deleted_at IS NULL
        GROUP BY workspace_id, slug
        HAVING COUNT(*) > 1
    LOOP
        n := 2;
        FOR loser IN
            SELECT id, slug
            FROM docs_collections
            WHERE deleted_at IS NULL
              AND workspace_id = dup.workspace_id
              AND slug = dup.slug
            ORDER BY created_at ASC, id ASC
            OFFSET 1
        LOOP
            LOOP
                new_slug := loser.slug || '-' || n::TEXT;
                IF NOT EXISTS (
                    SELECT 1
                    FROM docs_collections
                    WHERE workspace_id = dup.workspace_id
                      AND slug = new_slug
                      AND deleted_at IS NULL
                ) THEN
                    UPDATE docs_collections
                    SET slug = new_slug,
                        updated_at = now()
                    WHERE id = loser.id;
                    EXIT;
                END IF;
                n := n + 1;
            END LOOP;
            n := n + 1;
        END LOOP;
    END LOOP;
END $$;

-- Step 3: partial unique index on (workspace_id, slug) for non-deleted rows.
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collections_ws_slug_alive
    ON docs_collections (workspace_id, slug)
    WHERE deleted_at IS NULL;

-- Step 4: tree-aware ordering index and parent lookup index.
--
-- Postgres cannot include NULL in a btree tie-breaker efficiently, but the
-- (space_id, parent_collection_id, position) composite is still the right
-- key: null parents cluster together and we always scope sibling reads by
-- parent_collection_id IS NULL or parent_collection_id = ? at the repo
-- layer.
CREATE INDEX IF NOT EXISTS idx_docs_collections_space_parent_pos
    ON docs_collections (space_id, parent_collection_id, position);

CREATE INDEX IF NOT EXISTS idx_docs_collections_parent
    ON docs_collections (parent_collection_id);

-- Step 5: drop the legacy flat ordering index — superseded by the
-- tree-aware index above.
DROP INDEX IF EXISTS idx_docs_collection_space_pos;

-- Note: the docs_slug_aliases -> docs_redirects data migration is handled
-- as a Go-level backfill in a later task (Task 7), because it requires
-- joining across docs_documents and docs_collections to reconstruct the
-- canonical public path for each alias. Doing that join safely in plain
-- SQL is fragile; the Go backfill can skip orphaned aliases and log
-- warnings instead.
