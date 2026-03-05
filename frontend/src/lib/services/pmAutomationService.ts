import { api } from '../api';
import type { PMAutomation, UpsertAutomationRequest } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmAutomationService = {
  list: (workspaceId: string) =>
    api.get<PMAutomation[]>(`/pm/automations${qs(workspaceId)}`),
  upsert: (workspaceId: string, payload: UpsertAutomationRequest) =>
    api.put<PMAutomation>(`/pm/automations${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, automationType: string, teamId?: string) => {
    const params = new URLSearchParams({ workspace_id: workspaceId, automation_type: automationType });
    if (teamId) params.set('team_id', teamId);
    return api.del(`/pm/automations?${params.toString()}`);
  },
};
