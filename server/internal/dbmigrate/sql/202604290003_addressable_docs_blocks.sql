-- Add addressable document blocks while preserving the existing full-document
-- docs_contents aggregate for compatibility.

CREATE TABLE IF NOT EXISTS docs_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    document_id UUID NOT NULL,
    parent_id UUID NULL,
    type TEXT NOT NULL,
    content JSONB NOT NULL DEFAULT '{}'::jsonb,
    content_text TEXT,
    sort_key TEXT NOT NULL DEFAULT '~',
    revision INTEGER NOT NULL DEFAULT 1,
    authored_by UUID NULL,
    last_edited_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_docs_block_ws_doc_sort
    ON docs_blocks (workspace_id, document_id, sort_key)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_docs_block_doc_deleted
    ON docs_blocks (document_id, deleted_at);

CREATE INDEX IF NOT EXISTS idx_docs_block_type
    ON docs_blocks (type);

CREATE INDEX IF NOT EXISTS idx_docs_block_parent
    ON docs_blocks (parent_id);

ALTER TABLE docs_links
    ADD COLUMN IF NOT EXISTS block_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_docs_links_block_id
    ON docs_links (block_id);

ALTER TABLE docs_chunks
    ADD COLUMN IF NOT EXISTS block_id UUID NULL,
    ADD COLUMN IF NOT EXISTS block_range JSONB NULL;

CREATE INDEX IF NOT EXISTS idx_docs_chunks_block_id
    ON docs_chunks (block_id);

-- Backfill existing top-level TipTap nodes into block rows. Existing documents
-- that already have active block rows are skipped so the migration remains
-- safe to rerun.
WITH expanded AS MATERIALIZED (
    SELECT
        dc.id AS content_id,
        dc.document_id,
        dd.workspace_id,
        (elem.ordinality - 1)::integer AS block_index,
        elem.value AS original_node,
        gen_random_uuid() AS block_id
    FROM docs_contents dc
    JOIN docs_documents dd ON dd.id = dc.document_id
    CROSS JOIN LATERAL jsonb_array_elements(COALESCE(dc.content::jsonb -> 'content', '[]'::jsonb))
        WITH ORDINALITY AS elem(value, ordinality)
    WHERE dc.content::jsonb ->> 'type' = 'doc'
      AND NOT EXISTS (
          SELECT 1
          FROM docs_blocks b
          WHERE b.document_id = dc.document_id
            AND b.deleted_at IS NULL
      )
),
patched AS (
    SELECT
        content_id,
        document_id,
        workspace_id,
        block_index,
        original_node,
        block_id,
        jsonb_set(
            CASE
                WHEN jsonb_typeof(original_node -> 'attrs') = 'object' THEN original_node
                ELSE jsonb_set(original_node, '{attrs}', '{}'::jsonb, true)
            END,
            '{attrs,blockId}',
            to_jsonb(block_id::text),
            true
        ) AS node
    FROM expanded
    WHERE COALESCE(original_node ->> 'type', '') <> ''
),
updated_content AS (
    UPDATE docs_contents dc
    SET content = jsonb_set(dc.content::jsonb, '{content}', grouped.nodes, true),
        updated_at = dc.updated_at
    FROM (
        SELECT content_id, jsonb_agg(node ORDER BY block_index) AS nodes
        FROM patched
        GROUP BY content_id
    ) grouped
    WHERE dc.id = grouped.content_id
    RETURNING dc.id
)
INSERT INTO docs_blocks (
    id,
    workspace_id,
    document_id,
    type,
    content,
    sort_key,
    revision,
    created_at,
    updated_at
)
SELECT
    block_id,
    workspace_id,
    document_id,
    original_node ->> 'type',
    node,
    lpad(block_index::text, 12, '0'),
    1,
    NOW(),
    NOW()
FROM patched
ON CONFLICT (id) DO NOTHING;
