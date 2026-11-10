-- Preserve an exact recovery snapshot before renaming the standard image node
-- to the image extension registered by the document editor. Historical versions
-- and published snapshots remain unchanged.
CREATE OR REPLACE FUNCTION pg_temp.normalize_document_images(node jsonb)
RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE
    child jsonb;
    children jsonb := '[]'::jsonb;
BEGIN
    IF jsonb_typeof(node) IS DISTINCT FROM 'object' THEN RETURN node; END IF;
    IF node->>'type' = 'image' THEN
        node := jsonb_set(node, '{type}', '"resizableImage"'::jsonb);
    END IF;
    IF jsonb_typeof(node->'content') = 'array' THEN
        FOR child IN SELECT value FROM jsonb_array_elements(node->'content') LOOP
            children := children || jsonb_build_array(pg_temp.normalize_document_images(child));
        END LOOP;
        node := jsonb_set(node, '{content}', children);
    END IF;
    RETURN node;
END;
$$;

-- Saves lock the parent document first; serialize snapshots and block updates
-- with those saves so a concurrent edit cannot be overwritten by this repair.
LOCK TABLE docs_documents, docs_contents, docs_blocks, docs_versions IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE image_node_repairs ON COMMIT DROP AS
SELECT c.document_id, c.content AS original, pg_temp.normalize_document_images(c.content) AS repaired,
       c.content_text, c.word_count, d.created_by
FROM docs_contents c JOIN docs_documents d ON d.id = c.document_id
WHERE d.deleted_at IS NULL
  AND c.content->>'type' = 'doc'
  AND jsonb_path_exists(c.content, '$.** ? (@.type == "image")')
  AND c.content IS DISTINCT FROM pg_temp.normalize_document_images(c.content);

INSERT INTO docs_versions (id, document_id, content, content_text, snapshot_label, version_type, word_count, created_by, created_at)
SELECT gen_random_uuid(), document_id, original, content_text,
       'Recovery: before image node normalization', 'auto', word_count, created_by, now()
FROM image_node_repairs;

UPDATE docs_contents c SET content = r.repaired, updated_at = now()
FROM image_node_repairs r WHERE c.document_id = r.document_id;

UPDATE docs_blocks b
SET content = pg_temp.normalize_document_images(b.content),
    type = CASE WHEN b.type = 'image' THEN 'resizableImage' ELSE b.type END,
    revision = b.revision + 1, updated_at = now()
FROM image_node_repairs r
WHERE b.document_id = r.document_id AND b.deleted_at IS NULL
  AND b.content IS DISTINCT FROM pg_temp.normalize_document_images(b.content);

UPDATE docs_documents d SET updated_at = now()
FROM image_node_repairs r WHERE d.id = r.document_id;

DROP TABLE pg_temp.image_node_repairs;
DROP FUNCTION pg_temp.normalize_document_images(jsonb);
