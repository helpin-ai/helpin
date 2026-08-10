-- TipTap's code mark excludes all other marks. Older Markdown imports could
-- emit combinations such as bold+code, which the strict editor rejects.

CREATE OR REPLACE FUNCTION pg_temp.repair_tiptap_code_marks(node jsonb)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
    child jsonb;
    repaired_content jsonb := '[]'::jsonb;
    code_mark jsonb;
BEGIN
    IF jsonb_typeof(node) <> 'object' THEN
        RETURN node;
    END IF;

    IF jsonb_typeof(node->'content') = 'array' THEN
        FOR child IN SELECT value FROM jsonb_array_elements(node->'content')
        LOOP
            repaired_content := repaired_content || jsonb_build_array(
                pg_temp.repair_tiptap_code_marks(child)
            );
        END LOOP;
        node := jsonb_set(node, '{content}', repaired_content);
    END IF;

    IF node->>'type' = 'text'
       AND jsonb_typeof(node->'marks') = 'array'
       AND jsonb_array_length(node->'marks') > 1
       AND jsonb_path_exists(node, '$.marks[*] ? (@.type == "code")') THEN
        SELECT value
        INTO code_mark
        FROM jsonb_array_elements(node->'marks')
        WHERE value->>'type' = 'code'
        LIMIT 1;

        node := jsonb_set(node, '{marks}', jsonb_build_array(code_mark));
    END IF;

    RETURN node;
END;
$$;

UPDATE docs_contents
SET content = pg_temp.repair_tiptap_code_marks(content),
    updated_at = now()
WHERE jsonb_path_exists(
    content,
    '$.** ? (@.type == "text" && @.marks.size() > 1 && @.marks[*].type == "code")'
);

DROP FUNCTION pg_temp.repair_tiptap_code_marks(jsonb);
