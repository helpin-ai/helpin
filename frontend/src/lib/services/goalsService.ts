import { api } from '../api';
import type { CompanyGoal, SprintGoal } from '../types';

export const goalsService = {
  list: (workspaceId: string, quarterId: string) =>
    api.get<CompanyGoal[]>(`/goals?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  listSprintGoals: (sprintId: string, teamId?: string) =>
    api.get<SprintGoal[]>(`/goals/sprint?sprint_id=${encodeURIComponent(sprintId)}${teamId ? `&team_id=${encodeURIComponent(teamId)}` : ''}`),
  create: (data: { workspace_id: string; quarter_id: string; title: string; description?: string; goal_type: string; baseline?: number; target?: number; unit?: string; team_contributions?: { team_id: string; contribution_pct: number; target_value?: number; rationale?: string }[] }) =>
    api.post<CompanyGoal>('/goals', data),
  upsertSprintGoal: (data: Omit<SprintGoal, 'id'>) =>
    api.post<SprintGoal>('/goals/sprint', data),
};
