-- Migration: upgrade_scribe_to_codex_gpt56_terra
-- Move existing built-in Scribe agents to Codex. Agents still on Scribe's
-- previous product-default routing also move to OpenAI GPT-5.6 Terra; explicit
-- custom provider/model selections are preserved.

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
            provider = CASE
                WHEN provider = 'openrouter' AND model = 'deepseek/deepseek-v4-flash'
                    THEN 'openai'
                ELSE provider
            END,
            model = CASE
                WHEN provider = 'openrouter' AND model = 'deepseek/deepseek-v4-flash'
                    THEN 'gpt-5.6-terra'
                ELSE model
            END
        WHERE is_system = true
          AND preset_key = 'task_planner'
          AND (
              runtime_kind IS DISTINCT FROM 'codex'
              OR (provider = 'openrouter' AND model = 'deepseek/deepseek-v4-flash')
          );
    END IF;
END $$;
