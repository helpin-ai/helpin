import { api } from '../api';
import type { Sprint, IndividualCheck } from '../types';

export const sprintsService = {
  list: (quarterId: string) => api.get<Sprint[]>(`/sprints?quarter_id=${quarterId}`),
  get: (id: string) => api.get<Sprint>(`/sprints/${id}`),
  getIndividualChecks: (sprintId: string, workspaceId: string) =>
    api.get<IndividualCheck[]>(`/sprints/individual-checks?sprint_id=${sprintId}&workspace_id=${workspaceId}`),
  upsertIndividualCheck: (data: Omit<IndividualCheck, 'id'>) =>
    api.put<IndividualCheck>('/sprints/individual-checks', data),
  lock: (id: string) => api.post(`/sprints/${id}/lock`),
  unlock: (id: string) => api.post(`/sprints/${id}/unlock`),
};
