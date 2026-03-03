import { api } from '../api';
import type { Workspace, WorkspaceMember } from '../types';

export const workspacesService = {
  list: () => api.get<Workspace[]>('/workspaces'),
  create: (data: { name: string; slug: string; description?: string }) =>
    api.post<Workspace>('/workspaces', data),
  getBySlug: (slug: string) => api.get<Workspace>(`/workspaces/by-slug/${slug}`),
  update: (id: string, data: Partial<Workspace>) =>
    api.put<Workspace>(`/workspaces/${id}`, data),
  delete: (id: string) => api.del(`/workspaces/${id}`),
  getMyRole: (id: string) => api.get<{ role: string }>(`/workspaces/${id}/my-role`),
  getMyMembership: (id: string) => api.get<WorkspaceMember>(`/workspaces/${id}/my-membership`),
};
