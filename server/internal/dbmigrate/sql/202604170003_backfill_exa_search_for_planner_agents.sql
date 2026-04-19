-- Migration: backfill_exa_search_for_planner_agents
-- Grant Exa web search to stale Atlas/Scribe planner rows that already have
-- Brave web search enabled but predate the Exa tool rollout.

DO $$
BEGIN
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
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["web_search_exa"]'::jsonb
        WHERE is_system = true
          AND preset_key IN ('epic_planner', 'task_planner')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["web_search_brave"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["web_search_exa"]'::jsonb);
    END IF;

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
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["web_search_exa"]'::jsonb
        WHERE family_key IN ('epic_planner', 'task_planner')
          AND COALESCE(allowed_tools, '[]'::jsonb) @> '["web_search_brave"]'::jsonb
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["web_search_exa"]'::jsonb);
    END IF;
END $$;
