import { api } from '../api';
import type { WorkspaceSettings, WorkspaceTeam, WorkspacePerson, BonusTierConfig, JobRoleCriteria } from '../types';

export const settingsService = {
  getAll: (workspaceId: string) => api.get<WorkspaceSettings>(`/settings/${workspaceId}`),
  initialize: (workspaceId: string) => api.post(`/settings/${workspaceId}/initialize`),
  createTeam: (data: { workspace_id: string; name: string; description?: string; manager_id?: string }) =>
    api.post<WorkspaceTeam>('/settings/teams', data),
  updateTeam: (id: string, data: Partial<WorkspaceTeam>) =>
    api.patch<WorkspaceTeam>(`/settings/teams/${id}`, data),
  deleteTeam: (id: string) => api.del(`/settings/teams/${id}`),
  createPerson: (data: Omit<WorkspacePerson, 'id'>) =>
    api.post<WorkspacePerson>('/settings/people', data),
  updatePerson: (id: string, data: Partial<WorkspacePerson>) =>
    api.patch<WorkspacePerson>(`/settings/people/${id}`, data),
  deletePerson: (id: string) => api.del(`/settings/people/${id}`),
  updateBonusTiers: (workspaceId: string, tiers: Omit<BonusTierConfig, 'id' | 'workspace_id'>[]) =>
    api.put(`/settings/${workspaceId}/bonus-tiers`, { tiers }),
  updateJobRoleCriteria: (workspaceId: string, jobRole: string, criteria: Omit<JobRoleCriteria, 'id' | 'workspace_id'>[]) =>
    api.put(`/settings/${workspaceId}/job-roles/${encodeURIComponent(jobRole)}`, { criteria }),
  deleteJobRole: (workspaceId: string, jobRole: string) =>
    api.del(`/settings/${workspaceId}/job-roles/${encodeURIComponent(jobRole)}`),
  updateSystem: (workspaceId: string, data: { team_weight?: number; sprint_duration_weeks?: number; notifications_enabled?: boolean; auto_calculate_bonuses?: boolean }) =>
    api.patch(`/settings/${workspaceId}/system`, data),
};
