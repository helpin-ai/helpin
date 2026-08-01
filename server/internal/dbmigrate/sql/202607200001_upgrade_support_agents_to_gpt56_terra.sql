-- Migration: upgrade_support_agents_to_gpt56_terra
-- Route Helpin-managed support agents through OpenAI GPT-5.6 Terra without
-- changing customer-created agents or editable workspace preset versions.

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
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'provider'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'model'
    ) THEN
        UPDATE agents
        SET provider = 'openai',
            model = 'gpt-5.6-terra'
        WHERE is_system = true
          AND preset_key = 'support_agent'
          AND (
              provider IS DISTINCT FROM 'openai'
              OR model IS DISTINCT FROM 'gpt-5.6-terra'
          );
    END IF;
END $$;
