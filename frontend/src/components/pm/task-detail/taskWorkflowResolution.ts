import type { WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';

export function resolveTaskWorkflowStates(
  workflows: WorkflowWithStates[] | undefined,
  fallbackStates: WorkflowState[],
  workflowId?: string | null,
): WorkflowState[] {
  if (!workflowId) return fallbackStates;
  return workflows?.find((candidate) => candidate.workflow.id === workflowId)?.states ?? fallbackStates;
}

export function resolveTaskTeamWorkflow(
  workflows: WorkflowWithStates[] | undefined,
  teamId?: string | null,
): WorkflowWithStates | null {
  if (!workflows || workflows.length === 0) return null;
  if (teamId) {
    const teamWorkflow = workflows.find((candidate) => candidate.workflow.team_id === teamId);
    if (teamWorkflow) return teamWorkflow;
  }
  return workflows.find((candidate) => !candidate.workflow.team_id) ?? null;
}
