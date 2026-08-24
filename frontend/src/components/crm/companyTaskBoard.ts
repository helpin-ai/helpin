import type { StateType, Task, WorkflowWithStates } from '@/lib/pmTypes';

export const COMPANY_TASK_STATE_GROUPS: Array<{ type: StateType; label: string }> = [
  { type: 'backlog', label: 'Backlog' },
  { type: 'unstarted', label: 'Not Started' },
  { type: 'started', label: 'In Progress' },
  { type: 'done', label: 'Completed' },
];

export function companyTaskStateMap(workflows: WorkflowWithStates[]) {
  const map = new Map<string, WorkflowWithStates['states'][number]>();
  for (const workflow of workflows) {
    for (const state of workflow.states) map.set(state.id, state);
  }
  return map;
}

export function groupCompanyTasksByStateType(tasks: Task[], workflows: WorkflowWithStates[]) {
  const stateByID = companyTaskStateMap(workflows);
  const groups = new Map<StateType, Task[]>(
    COMPANY_TASK_STATE_GROUPS.map((group) => [group.type, []]),
  );

  for (const task of tasks) {
    const stateType = task.state_type ?? stateByID.get(task.workflow_state_id)?.state_type as StateType | undefined;
    if (stateType && groups.has(stateType)) groups.get(stateType)?.push(task);
  }
  return groups;
}

export function companyTaskDropStates(
  task: Pick<Task, 'workflow_id'> | null,
  workflows: WorkflowWithStates[],
  stateType: StateType,
) {
  if (!task) return [];
  return (workflows.find((workflow) => workflow.workflow.id === task.workflow_id)?.states ?? [])
    .filter((state) => state.state_type === stateType)
    .sort((left, right) => left.position - right.position);
}
