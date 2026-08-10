-- Repair lossless TipTap structural defects left by older importers: empty
-- lists, list items that do not start with a paragraph, and duplicate marks.

CREATE OR REPLACE FUNCTION pg_temp.repair_tiptap_lists_and_marks(node jsonb)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
    child jsonb;
    mark jsonb;
    mark_type text;
    repaired_content jsonb := '[]'::jsonb;
    repaired_marks jsonb := '[]'::jsonb;
    seen_mark_types text[] := ARRAY[]::text[];
BEGIN
    IF jsonb_typeof(node) <> 'object' THEN
        RETURN node;
    END IF;

    IF jsonb_typeof(node->'content') = 'array' THEN
        FOR child IN SELECT value FROM jsonb_array_elements(node->'content')
        LOOP
            child := pg_temp.repair_tiptap_lists_and_marks(child);
            IF child->>'type' IN ('bulletList', 'orderedList')
               AND (
                   coalesce(jsonb_typeof(child->'content'), 'null') <> 'array'
                   OR jsonb_array_length(child->'content') = 0
               ) THEN
                CONTINUE;
            END IF;
            repaired_content := repaired_content || jsonb_build_array(child);
        END LOOP;

        IF node->>'type' = 'listItem'
           AND (
               jsonb_array_length(repaired_content) = 0
               OR repaired_content->0->>'type' <> 'paragraph'
           ) THEN
            repaired_content := jsonb_build_array(jsonb_build_object('type', 'paragraph'))
                || repaired_content;
        END IF;
        node := jsonb_set(node, '{content}', repaired_content);
    END IF;

    IF node->>'type' = 'text' AND jsonb_typeof(node->'marks') = 'array' THEN
        FOR mark IN SELECT value FROM jsonb_array_elements(node->'marks')
        LOOP
            mark_type := mark->>'type';
            IF mark_type IS NULL OR NOT (mark_type = ANY(seen_mark_types)) THEN
                repaired_marks := repaired_marks || jsonb_build_array(mark);
                IF mark_type IS NOT NULL THEN
                    seen_mark_types := array_append(seen_mark_types, mark_type);
                END IF;
            END IF;
        END LOOP;
        node := jsonb_set(node, '{marks}', repaired_marks);
    END IF;

    RETURN node;
END;
$$;

WITH RECURSIVE nodes(document_id, node) AS (
    SELECT document_id, content
    FROM docs_contents
    UNION ALL
    SELECT nodes.document_id, child.value
    FROM nodes
    CROSS JOIN LATERAL jsonb_array_elements(
        CASE WHEN jsonb_typeof(nodes.node->'content') = 'array'
            THEN nodes.node->'content' ELSE '[]'::jsonb END
    ) child(value)
), affected AS (
    SELECT DISTINCT document_id
    FROM nodes
    WHERE (
        node->>'type' IN ('bulletList', 'orderedList')
        AND (
            coalesce(jsonb_typeof(node->'content'), 'null') <> 'array'
            OR jsonb_array_length(node->'content') = 0
        )
    ) OR (
        node->>'type' = 'listItem'
        AND (
            coalesce(jsonb_typeof(node->'content'), 'null') <> 'array'
            OR jsonb_array_length(node->'content') = 0
            OR node->'content'->0->>'type' <> 'paragraph'
        )
    ) OR (
        node->>'type' = 'text'
        AND jsonb_typeof(node->'marks') = 'array'
        AND EXISTS (
            SELECT 1
            FROM jsonb_array_elements(node->'marks') mark
            GROUP BY mark->>'type'
            HAVING count(*) > 1
        )
    )
)
UPDATE docs_contents content
SET content = pg_temp.repair_tiptap_lists_and_marks(content.content),
    updated_at = now()
FROM affected
WHERE affected.document_id = content.document_id;

DROP FUNCTION pg_temp.repair_tiptap_lists_and_marks(jsonb);
