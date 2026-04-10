-- Migration: refresh_lens_interactive_review_loop
-- Move built-in Lens agents to the interactive review-loop preset version and
-- clear the stored system prompt so startup reconciliation rewrites it.

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
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'preset_version_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'source_preset_version_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'system_prompt'
    ) THEN
        UPDATE agents
        SET preset_version_key = 'review_agent_interactive_loop',
            source_preset_version_key = CASE
                WHEN source_preset_version_key IS NULL OR source_preset_version_key = '' OR source_preset_version_key = 'review_agent_default'
                    THEN 'review_agent_interactive_loop'
                ELSE source_preset_version_key
            END,
            system_prompt = NULL
        WHERE is_system = true
          AND preset_key = 'review_agent'
          AND (
              preset_version_key IS NULL OR
              preset_version_key = '' OR
              preset_version_key = 'review_agent_default'
          );
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'family_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'source_version_key'
    ) THEN
        UPDATE workspace_agent_preset_versions
        SET source_version_key = 'review_agent_interactive_loop'
        WHERE family_key = 'review_agent'
          AND source_version_key = 'review_agent_default';
    END IF;
END $$;
