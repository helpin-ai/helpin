-- Rollback for 202606140001_rename_native_agent_contract_artifacts.
--
-- WARNING:
-- This script is intentionally NOT embedded in the runtime migration runner.
-- Use it only for a controlled rollback together with native-planner-era
-- application code and a release that runs the API with RUN_AUTO_MIGRATE=false.

UPDATE agent_run_artifacts
SET artifact_type = 'native_turn_debug'
WHERE artifact_type = 'agent_turn_debug';

UPDATE agent_run_artifacts
SET artifact_type = 'native_repair_state'
WHERE artifact_type = 'agent_repair_state';
