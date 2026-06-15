-- Rename native-specific planner artifact types to backend-neutral contract artifact types.
UPDATE agent_run_artifacts
SET artifact_type = 'agent_turn_debug'
WHERE artifact_type = 'native_turn_debug';

UPDATE agent_run_artifacts
SET artifact_type = 'agent_repair_state'
WHERE artifact_type = 'native_repair_state';
