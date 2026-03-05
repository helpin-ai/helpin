import { api } from '../api';
import type { RewardFinanceSettings } from '../types';

export const rewardFinanceService = {
  get: (workspaceId: string, quarterId: string) =>
    api.get<RewardFinanceSettings>(`/rewards/finance?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  upsert: (data: Partial<RewardFinanceSettings>) =>
    api.post<RewardFinanceSettings>('/rewards/finance', data),
};
