import { api } from '../api';
import type { Sprint, IndividualCheck } from '../types';

export const sprintsService = {
  list: (quarterId: string) => api.get<Sprint[]>(`/sprints?quarter_id=${quarterId}`),
  get: (id: string) => api.get<Sprint>(`/sprints/${id}`),
  getIndividualChecks: (sprintId: string, _workspaceId?: string) =>
    api.get<IndividualCheck[]>(`/sprints/${sprintId}/checks`),
  upsertIndividualCheck: (data: Omit<IndividualCheck, 'id'>) =>
    api.post<IndividualCheck>(`/sprints/${data.sprint_id}/checks`, data),
  lock: (id: string) => api.post(`/sprints/${id}/lock`),
  unlock: (id: string) => api.post(`/sprints/${id}/unlock`),
};
