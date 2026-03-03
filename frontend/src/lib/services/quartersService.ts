import { api } from '../api';
import type { Quarter } from '../types';

export const quartersService = {
  list: (workspaceId: string) => api.get<Quarter[]>(`/quarters?workspace_id=${workspaceId}`),
  create: (data: { workspace_id: string; name: string; start_date: string; end_date: string }) =>
    api.post<Quarter>('/quarters', data),
  get: (id: string) => api.get<Quarter>(`/quarters/${id}`),
  updateStatus: (id: string, status: string) =>
    api.patch<Quarter>(`/quarters/${id}/status`, { status }),
};
