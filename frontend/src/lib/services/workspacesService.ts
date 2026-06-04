import { api, API_BASE, fetchWithSessionAuth } from '../api';
import type {
  AssignableMember,
  MemberWithUser,
  Workspace,
  WorkspaceAccess,
  WorkspaceMember,
  WorkspaceMemberPresenceStatus,
} from '../types';

export const workspacesService = {
  list: (organizationId?: string) =>
    api.get<Workspace[]>(organizationId ? `/workspaces?organization_id=${organizationId}` : '/workspaces'),
  create: (data: { name: string; slug: string; workspace_key: string; organization_id: string; description?: string; website_url?: string; timezone?: string }) =>
    api.post<Workspace>('/workspaces', data),
  getBySlug: (slug: string) => api.get<Workspace>(`/workspaces/by-slug/${slug}`),
  update: (id: string, data: Partial<Workspace>) =>
    api.put<Workspace>(`/workspaces/${id}`, data),
  delete: (id: string) => api.del(`/workspaces/${id}`),
  getMyRole: (id: string) => api.get<{ role: string }>(`/workspaces/${id}/my-role`),
  getMyMembership: (id: string) => api.get<WorkspaceMember>(`/workspaces/${id}/my-membership`),
  getMe: (id: string) => api.get<WorkspaceAccess>(`/workspaces/${id}/me`),
  listMembers: (id: string) => api.get<MemberWithUser[]>(`/workspaces/${id}/members`),
  listMemberPresence: (id: string) => api.get<WorkspaceMemberPresenceStatus[]>(`/workspaces/${id}/members/presence`),
  updateMemberRole: (id: string, memberId: string, data: { role: WorkspaceMember['role'] }) =>
    api.put(`/workspaces/${id}/members/${memberId}`, data),
  removeMember: (id: string, memberId: string) =>
    api.del(`/workspaces/${id}/members/${memberId}`),
  listAssignableMembers: (id: string) => api.get<AssignableMember[]>(`/workspaces/${id}/assignable-members`),
  getKeyHistory: (id: string) => api.get<{ id: string; workspace_id: string; old_key: string; new_key: string; changed_at: string; changed_by: string }[]>(`/workspaces/${id}/key-history`),

  uploadLogo: async (id: string, file: File): Promise<{ data: Workspace | null; error: string | null }> => {
    const formData = new FormData();
    formData.append('logo', file);
    try {
      const res = await fetchWithSessionAuth(API_BASE, `/workspaces/${id}/logo`, {
        method: 'POST',
        body: formData,
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null, error: err.error || res.statusText };
      }
      const data = await res.json();
      return { data, error: null };
    } catch (e) {
      return { data: null, error: e instanceof Error ? e.message : 'Upload failed' };
    }
  },

  deleteLogo: (id: string) => api.del<Workspace>(`/workspaces/${id}/logo`),

  updateSupportTaskPreferences: (workspaceId: string, data: {
    support_default_team_id?: string;
    support_task_dialog_dismissed?: boolean;
  }) => api.patch<{ status: string }>(`/workspaces/${workspaceId}/me/support-task-preferences`, data),
};
