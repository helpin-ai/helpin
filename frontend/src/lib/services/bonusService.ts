import { api } from '../api';
import type { RewardBonusCalculation, RewardFinanceSettings } from '../types';

export const rewardBonusService = {
  getCalculations: (workspaceId: string, quarterId: string) =>
    api.get<RewardBonusCalculation[]>(`/rewards/bonus/calculations?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  saveCalculations: (calculations: Omit<RewardBonusCalculation, 'id'>[]) =>
    api.post('/rewards/bonus/calculations', { calculations }),
  lock: (workspaceId: string, quarterId: string) =>
    api.post('/rewards/bonus/lock', { workspace_id: workspaceId, quarter_id: quarterId }),
  unlock: (workspaceId: string, quarterId: string) =>
    api.post('/rewards/bonus/unlock', { workspace_id: workspaceId, quarter_id: quarterId }),
  getFinance: (workspaceId: string, quarterId: string) =>
    api.get<RewardFinanceSettings>(`/rewards/finance?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  upsertFinance: (data: Partial<RewardFinanceSettings>) =>
    api.post<RewardFinanceSettings>('/rewards/finance', data),
  getTeamSprintData: (sprintId: string) =>
    api.get(`/rewards/bonus/team-sprint-data?sprint_id=${encodeURIComponent(sprintId)}`),
};
