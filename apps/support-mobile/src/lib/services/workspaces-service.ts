import { api } from '@mobile/lib/api'
import type { AssignableMember } from '@helpin-ai/support-core'
import type { Workspace, WorkspaceAccess, WorkspaceSettingsSummary } from '@mobile/lib/types'

export const workspacesService = {
  list: () => api.get<Workspace[]>('/workspaces'),
  getBySlug: (slug: string) => api.get<Workspace>(`/workspaces/by-slug/${slug}`),
  getMe: (id: string) => api.get<WorkspaceAccess>(`/workspaces/${id}/me`),
  listAssignableMembers: (id: string) => api.get<AssignableMember[]>(`/workspaces/${id}/assignable-members`),
  getSettings: (id: string) => api.get<WorkspaceSettingsSummary>(`/settings?workspace_id=${encodeURIComponent(id)}`),
  updateSupportTaskPreferences: (id: string, data: {
    support_default_team_id?: string
    support_task_dialog_dismissed?: boolean
  }) => api.patch<{ message: string }>(`/workspaces/${id}/me/support-task-preferences`, data),
}
