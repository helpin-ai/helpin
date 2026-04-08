-- Migration: migrate_forge_agents_to_codex
-- Move existing built-in Forge agents to Codex with OpenAI gpt-5-mini.

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
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'runtime_kind'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'provider'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'model'
    ) THEN
        UPDATE agents
        SET runtime_kind = 'codex',
            provider = 'openai',
            model = 'gpt-5-mini'
        WHERE is_system = true
          AND preset_key = 'code_builder'
          AND (
              runtime_kind IS DISTINCT FROM 'codex' OR
              provider IS DISTINCT FROM 'openai' OR
              model IS DISTINCT FROM 'gpt-5-mini'
          );
    END IF;
END $$;
