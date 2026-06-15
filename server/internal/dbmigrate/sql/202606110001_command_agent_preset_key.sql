DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'preset_key'
  ) THEN
    UPDATE agents
       SET preset_key = 'command_agent'
     WHERE preset_key = 'researcher';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'source_preset_key'
  ) THEN
    UPDATE agents
       SET source_preset_key = 'command_agent'
     WHERE source_preset_key = 'researcher';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'preset_version_key'
  ) THEN
    UPDATE agents
       SET preset_version_key = 'command_agent_default'
     WHERE preset_version_key = 'researcher_default';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'source_preset_version_key'
  ) THEN
    UPDATE agents
       SET source_preset_version_key = 'command_agent_default'
     WHERE source_preset_version_key = 'researcher_default';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'family_key'
  ) THEN
    UPDATE workspace_agent_preset_versions
       SET family_key = 'command_agent'
     WHERE family_key = 'researcher';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'version_key'
  ) THEN
    UPDATE workspace_agent_preset_versions
       SET version_key = 'command_agent_default'
     WHERE version_key = 'researcher_default';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'workspace_agent_preset_versions' AND column_name = 'source_version_key'
  ) THEN
    UPDATE workspace_agent_preset_versions
       SET source_version_key = 'command_agent_default'
     WHERE source_version_key = 'researcher_default';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'command_bar_plans' AND column_name = 'steps'
  ) THEN
    UPDATE command_bar_plans
       SET steps = replace(steps::text, '"agent_key": "researcher"', '"agent_key": "command_agent"')::jsonb
     WHERE steps::text LIKE '%"agent_key": "researcher"%';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'command_bar_messages' AND column_name = 'proposal_json'
  ) THEN
    UPDATE command_bar_messages
       SET proposal_json = replace(proposal_json::text, '"agent_key": "researcher"', '"agent_key": "command_agent"')::jsonb
     WHERE proposal_json IS NOT NULL
       AND proposal_json::text LIKE '%"agent_key": "researcher"%';
  END IF;
END $$;
