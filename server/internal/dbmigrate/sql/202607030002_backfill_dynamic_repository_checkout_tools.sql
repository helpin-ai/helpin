-- Migration: backfill_dynamic_repository_checkout_tools
-- Existing workspace preset versions and system agents copied allowed_tools
-- before dynamic runtime repository checkout existed.

DO $$
DECLARE
    repo_tool text;
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'family_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'allowed_tools'
    ) THEN
        UPDATE workspace_agent_preset_versions
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["list_repositories"]'::jsonb
        WHERE family_key IN ('epic_planner', 'task_planner', 'documentation_agent', 'command_agent')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["read_file"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["list_repositories"]'::jsonb);

        UPDATE workspace_agent_preset_versions
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["checkout_repository"]'::jsonb
        WHERE family_key IN ('epic_planner', 'task_planner', 'documentation_agent', 'command_agent')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["read_file"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["checkout_repository"]'::jsonb);

        UPDATE workspace_agent_preset_versions
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["checkout_repositories"]'::jsonb
        WHERE family_key IN ('epic_planner', 'task_planner', 'documentation_agent', 'command_agent')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["read_file"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["checkout_repositories"]'::jsonb);

        FOREACH repo_tool IN ARRAY ARRAY['list_repositories','checkout_repository','checkout_repositories','list_commits','read_file','read_file_range','read_files','list_directory','search_files','ripgrep','grep','list_symbols']
        LOOP
            UPDATE workspace_agent_preset_versions
            SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || jsonb_build_array(repo_tool)
            WHERE family_key = 'marketer'
              AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> jsonb_build_array(repo_tool));
        END LOOP;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'is_system'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'preset_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'allowed_tools'
    ) THEN
        UPDATE agents
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["list_repositories"]'::jsonb
        WHERE is_system = true
          AND preset_key IN ('epic_planner', 'task_planner', 'documentation_agent', 'command_agent')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["read_file"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["list_repositories"]'::jsonb);

        UPDATE agents
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["checkout_repository"]'::jsonb
        WHERE is_system = true
          AND preset_key IN ('epic_planner', 'task_planner', 'documentation_agent', 'command_agent')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["read_file"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["checkout_repository"]'::jsonb);

        UPDATE agents
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["checkout_repositories"]'::jsonb
        WHERE is_system = true
          AND preset_key IN ('epic_planner', 'task_planner', 'documentation_agent', 'command_agent')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["read_file"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["checkout_repositories"]'::jsonb);

        FOREACH repo_tool IN ARRAY ARRAY['list_repositories','checkout_repository','checkout_repositories','list_commits','read_file','read_file_range','read_files','list_directory','search_files','ripgrep','grep','list_symbols']
        LOOP
            UPDATE agents
            SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || jsonb_build_array(repo_tool)
            WHERE is_system = true
              AND preset_key = 'marketer'
              AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> jsonb_build_array(repo_tool));
        END LOOP;
    END IF;
END $$;
