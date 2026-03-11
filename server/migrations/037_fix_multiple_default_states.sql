-- Fix workflows that have more than one default state.
-- Keep only the state referenced by pm_workflows.default_state_id; clear the rest.
UPDATE pm_workflow_states
SET    is_default = false
WHERE  is_default = true
  AND  id NOT IN (
    SELECT default_state_id
    FROM   pm_workflows
    WHERE  default_state_id IS NOT NULL
  );
