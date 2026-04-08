-- Migration: upgrade_forge_lens_default_model_to_gpt54
-- Move existing built-in Forge and Lens agents to OpenAI gpt-5.4.

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
            model = 'gpt-5.4'
        WHERE is_system = true
          AND preset_key IN ('code_builder', 'review_agent')
          AND (
              runtime_kind IS DISTINCT FROM 'codex' OR
              provider IS DISTINCT FROM 'openai' OR
              model IS DISTINCT FROM 'gpt-5.4'
          );
    END IF;
END $$;
