import { api } from '../api';
import type { RewardQuarter } from '../types';

export const rewardQuartersService = {
  list: (workspaceId: string) => api.get<RewardQuarter[]>(`/rewards/quarters?workspace_id=${workspaceId}`),
  create: (data: { workspace_id: string; name: string; start_date: string; end_date: string }) =>
    api.post<RewardQuarter>('/rewards/quarters', data),
  get: (id: string) => api.get<RewardQuarter>(`/rewards/quarters/${id}`),
  updateStatus: (id: string, status: string) =>
    api.patch<RewardQuarter>(`/rewards/quarters/${id}/status`, { status }),
};
