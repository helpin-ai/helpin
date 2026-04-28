DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'allowed_targets'
  ) THEN
    UPDATE agents
    SET allowed_targets = COALESCE(allowed_targets, '[]'::jsonb) || '["crm_contact"]'::jsonb
    WHERE preset_key = 'crm_operator'
      AND COALESCE(allowed_targets, '[]'::jsonb) @> '["crm_deal"]'::jsonb
      AND NOT (COALESCE(allowed_targets, '[]'::jsonb) @> '["crm_contact"]'::jsonb);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'allowed_targets'
  ) THEN
    UPDATE workspace_agent_preset_versions
    SET allowed_targets = COALESCE(allowed_targets, '[]'::jsonb) || '["crm_contact"]'::jsonb
    WHERE family_key = 'crm_operator'
      AND COALESCE(allowed_targets, '[]'::jsonb) @> '["crm_deal"]'::jsonb
      AND NOT (COALESCE(allowed_targets, '[]'::jsonb) @> '["crm_contact"]'::jsonb);
  END IF;
END $$;
