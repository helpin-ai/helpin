import { api } from '../api';
import type { RewardGoalDraft } from '../types';

export const rewardDraftsService = {
  list: (workspaceId: string, quarterId: string) =>
    api.get<RewardGoalDraft[]>(`/rewards/drafts?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  create: (data: { workspace_id: string; quarter_id: string; draft_data: Record<string, unknown> }) =>
    api.post<RewardGoalDraft>('/rewards/drafts', data),
  get: (id: string) => api.get<RewardGoalDraft>(`/rewards/drafts/${id}`),
  update: (id: string, data: { draft_data?: Record<string, unknown>; status?: string }) =>
    api.put<RewardGoalDraft>(`/rewards/drafts/${id}`, data),
  delete: (id: string) => api.del(`/rewards/drafts/${id}`),
};
