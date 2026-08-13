-- Replace superseded model-facing tool names in every persisted tool policy.
-- The first occurrence wins so order remains stable while aliases collapse.

DO $$
DECLARE
    target_table text;
    target_column text;
    targets text[][] := ARRAY[
        ARRAY['agents', 'allowed_tools'],
        ARRAY['agent_versions', 'allowed_tools'],
        ARRAY['workspace_agent_preset_versions', 'allowed_tools'],
        ARRAY['agent_templates', 'allowed_tools'],
        ARRAY['workspace_skills', 'required_tools']
    ];
    target text[];
BEGIN
    FOREACH target SLICE 1 IN ARRAY targets
    LOOP
        target_table := target[1];
        target_column := target[2];
        IF EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = target_table
              AND column_name = target_column
        ) THEN
            EXECUTE format($sql$
                UPDATE %I row_value
                SET %I = COALESCE((
                    SELECT jsonb_agg(mapped_name ORDER BY first_position)
                    FROM (
                        SELECT mapped_name, min(position) AS first_position
                        FROM (
                            SELECT position,
                                   CASE tool_name
                                       WHEN 'checkout_repository' THEN 'checkout_repositories'
                                       WHEN 'read_file' THEN 'read_files'
                                       WHEN 'read_file_range' THEN 'read_files'
                                       WHEN 'search_files' THEN 'repository_search'
                                       WHEN 'ripgrep' THEN 'repository_search'
                                       WHEN 'grep' THEN 'repository_search'
                                       WHEN 'find_symbol' THEN 'read_symbol'
                                       WHEN 'find_callers' THEN 'trace_symbol'
                                       WHEN 'find_callees' THEN 'trace_symbol'
                                       WHEN 'list_available_skills' THEN 'find_skills'
                                       WHEN 'search_available_skills' THEN 'find_skills'
                                       WHEN 'web_search_brave' THEN 'web_search'
                                       WHEN 'web_search_exa' THEN 'web_search'
                                       ELSE tool_name
                                   END AS mapped_name
                            FROM jsonb_array_elements_text(COALESCE(row_value.%I, '[]'::jsonb))
                                 WITH ORDINALITY AS item(tool_name, position)
                        ) mapped
                        GROUP BY mapped_name
                    ) deduplicated
                ), '[]'::jsonb)
                WHERE jsonb_typeof(COALESCE(row_value.%I, '[]'::jsonb)) = 'array'
            $sql$, target_table, target_column, target_column, target_column);
        END IF;
    END LOOP;
END $$;
