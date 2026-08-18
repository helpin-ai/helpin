-- TipTap's resizableImage is a block node. Older Markdown imports could place
-- it inside paragraphs or headings when an image shared a line with text,
-- producing JSON that the strict editor content check rejects.

CREATE OR REPLACE FUNCTION pg_temp.repair_tiptap_block_images(node jsonb)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
    child jsonb;
    inline_node jsonb;
    repaired_content jsonb := '[]'::jsonb;
    inline_run jsonb;
    text_block jsonb;
BEGIN
    IF jsonb_typeof(node) <> 'object'
       OR jsonb_typeof(node->'content') <> 'array' THEN
        RETURN node;
    END IF;

    FOR child IN SELECT value FROM jsonb_array_elements(node->'content')
    LOOP
        child := pg_temp.repair_tiptap_block_images(child);

        IF child->>'type' IN ('paragraph', 'heading')
           AND jsonb_typeof(child->'content') = 'array'
           AND jsonb_path_exists(
               child,
               '$.content[*] ? (@.type == "resizableImage")'
           ) THEN
            inline_run := '[]'::jsonb;
            FOR inline_node IN
                SELECT value FROM jsonb_array_elements(child->'content')
            LOOP
                IF inline_node->>'type' = 'resizableImage' THEN
                    IF jsonb_array_length(inline_run) > 0 THEN
                        text_block := jsonb_set(child, '{content}', inline_run);
                        repaired_content := repaired_content || jsonb_build_array(text_block);
                        inline_run := '[]'::jsonb;
                    END IF;
                    repaired_content := repaired_content || jsonb_build_array(inline_node);
                ELSE
                    inline_run := inline_run || jsonb_build_array(inline_node);
                END IF;
            END LOOP;

            IF jsonb_array_length(inline_run) > 0 THEN
                text_block := jsonb_set(child, '{content}', inline_run);
                repaired_content := repaired_content || jsonb_build_array(text_block);
            END IF;
        ELSE
            repaired_content := repaired_content || jsonb_build_array(child);
        END IF;
    END LOOP;

    IF node->>'type' = 'listItem'
       AND (
           jsonb_array_length(repaired_content) = 0
           OR repaired_content->0->>'type' <> 'paragraph'
       ) THEN
        repaired_content := jsonb_build_array(jsonb_build_object('type', 'paragraph'))
            || repaired_content;
    END IF;

    RETURN jsonb_set(node, '{content}', repaired_content);
END;
$$;

UPDATE docs_contents
SET content = pg_temp.repair_tiptap_block_images(content),
    updated_at = now()
WHERE jsonb_path_exists(
    content,
    '$.** ? ((@.type == "paragraph" || @.type == "heading") && @.content[*].type == "resizableImage")'
);

DROP FUNCTION pg_temp.repair_tiptap_block_images(jsonb);
