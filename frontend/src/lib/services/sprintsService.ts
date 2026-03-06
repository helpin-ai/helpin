import { api } from '../api';
import type { RewardSprint, RewardIndividualCheck } from '../types';

export const rewardSprintsService = {
  list: (quarterId: string) => api.get<RewardSprint[]>(`/rewards/sprints?quarter_id=${quarterId}`),
  get: (id: string) => api.get<RewardSprint>(`/rewards/sprints/${id}`),
  getIndividualChecks: (sprintId: string, _workspaceId?: string) =>
    api.get<RewardIndividualCheck[]>(`/rewards/sprints/${sprintId}/checks`),
  upsertIndividualCheck: (data: Omit<RewardIndividualCheck, 'id'>) =>
    api.post<RewardIndividualCheck>(`/rewards/sprints/${data.sprint_id}/checks`, data),
  lock: (id: string) => api.post(`/rewards/sprints/${id}/lock`),
  unlock: (id: string) => api.post(`/rewards/sprints/${id}/unlock`),
};
