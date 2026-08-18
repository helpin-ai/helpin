import type { WorkflowWithStates } from '@/lib/pmTypes';

export function resolveMeetingActionStateId(
  workflow: WorkflowWithStates | undefined,
  selectedStateId?: string,
): string {
  if (!workflow) return '';

  if (selectedStateId && workflow.states.some((state) => state.id === selectedStateId)) {
    return selectedStateId;
  }

  const states = [...workflow.states].sort((left, right) => left.position - right.position);
  const defaultStateId = workflow.workflow.default_state_id;
  if (defaultStateId && states.some((state) => state.id === defaultStateId)) {
    return defaultStateId;
  }

  return states.find((state) => state.is_default)?.id ?? states[0]?.id ?? '';
}
