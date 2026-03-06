import { api } from '../api';
import type { RewardCompanyGoal, RewardSprintGoal } from '../types';

export const rewardGoalsService = {
  list: (workspaceId: string, quarterId: string) =>
    api.get<RewardCompanyGoal[]>(`/rewards/goals?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  listSprintGoals: (sprintId: string, teamId?: string) =>
    api.get<RewardSprintGoal[]>(`/rewards/goals/sprint?sprint_id=${encodeURIComponent(sprintId)}${teamId ? `&team_id=${encodeURIComponent(teamId)}` : ''}`),
  create: (data: { workspace_id: string; quarter_id: string; title: string; description?: string; goal_type: string; baseline?: number; target?: number; unit?: string; team_contributions?: { team_id: string; contribution_pct: number; target_value?: number; rationale?: string }[] }) =>
    api.post<RewardCompanyGoal>('/rewards/goals', data),
  upsertSprintGoal: (data: Omit<RewardSprintGoal, 'id'>) =>
    api.post<RewardSprintGoal>('/rewards/goals/sprint', data),
};
