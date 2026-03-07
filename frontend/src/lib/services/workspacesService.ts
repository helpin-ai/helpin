import { api } from '../api';
import type { AssignableMember, Workspace, WorkspaceMember, MemberWithUser } from '../types';

export const workspacesService = {
  list: (organizationId?: string) =>
    api.get<Workspace[]>(organizationId ? `/workspaces?organization_id=${organizationId}` : '/workspaces'),
  create: (data: { name: string; slug: string; organization_id: string; description?: string; timezone?: string }) =>
    api.post<Workspace>('/workspaces', data),
  getBySlug: (slug: string) => api.get<Workspace>(`/workspaces/by-slug/${slug}`),
  update: (id: string, data: Partial<Workspace>) =>
    api.put<Workspace>(`/workspaces/${id}`, data),
  delete: (id: string) => api.del(`/workspaces/${id}`),
  getMyRole: (id: string) => api.get<{ role: string }>(`/workspaces/${id}/my-role`),
  getMyMembership: (id: string) => api.get<WorkspaceMember>(`/workspaces/${id}/my-membership`),
  listMembers: (id: string) => api.get<MemberWithUser[]>(`/workspaces/${id}/members`),
  listAssignableMembers: (id: string) => api.get<AssignableMember[]>(`/workspaces/${id}/assignable-members`),
};
