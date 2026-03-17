import type { PMAutomation, UpsertAutomationRequest } from '@/lib/pmTypes';

export interface SprintAutomationPromptState {
  teamId: string;
  teamName: string;
  sprintCount: number;
  weeks: number;
  startDay: number;
  moveUnfinished: boolean;
}

export function shouldPromptSprintAutomation(automations: PMAutomation[] | null | undefined, teamId: string) {
  return !automations?.some(
    (automation) => automation.automation_type === 'sprint_auto_create' && automation.team_id === teamId,
  );
}

export function buildSprintAutomationDismissRequest(
  workspaceId: string,
  prompt: SprintAutomationPromptState,
): UpsertAutomationRequest {
  return {
    workspace_id: workspaceId,
    automation_type: 'sprint_auto_create',
    enabled: false,
    team_id: prompt.teamId,
    config_int: prompt.sprintCount,
    config_int2: prompt.weeks,
    config_int3: prompt.startDay,
  };
}

export async function dismissSprintAutomationPrompt({
  workspaceId,
  prompt,
  upsert,
}: {
  workspaceId: string;
  prompt: SprintAutomationPromptState;
  upsert: (workspaceId: string, payload: UpsertAutomationRequest) => Promise<{ data: unknown; error: string | null }>;
}) {
  const request = buildSprintAutomationDismissRequest(workspaceId, prompt);
  const { data, error } = await upsert(workspaceId, request);
  if (error) {
    throw new Error(error);
  }
  return data;
}
