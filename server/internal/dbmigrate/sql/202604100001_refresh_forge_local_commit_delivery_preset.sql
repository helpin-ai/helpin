-- Migration: refresh_forge_local_commit_delivery_preset
-- Move default built-in Forge agents to the local-commit delivery preset and
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
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'system_prompt'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'source_preset_version_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'runtime_kind'
    ) THEN
        UPDATE agents
        SET preset_version_key = 'code_builder_local_commit_delivery',
            runtime_kind = 'codex',
            source_preset_version_key = CASE
                WHEN source_preset_version_key IS NULL OR source_preset_version_key = '' OR source_preset_version_key = 'code_builder_default'
                    THEN 'code_builder_local_commit_delivery'
                ELSE source_preset_version_key
            END,
            system_prompt = NULL
        WHERE is_system = true
          AND preset_key = 'code_builder'
          AND (
              preset_version_key IS NULL OR
              preset_version_key = '' OR
              preset_version_key = 'code_builder_default'
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
        SET source_version_key = 'code_builder_local_commit_delivery'
        WHERE family_key = 'code_builder'
          AND source_version_key = 'code_builder_default';
    END IF;
END $$;
