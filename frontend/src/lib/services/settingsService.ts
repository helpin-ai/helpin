import { api } from '../api';
import type { WorkspaceSettings, WorkspaceTeam, WorkspacePerson, BonusTierConfig, JobRoleCriteria } from '../types';

interface RawWorkspaceSettings extends Omit<WorkspaceSettings, 'job_role_criteria'> {
  job_role_criteria?: JobRoleCriteria[];
  job_roles?: JobRoleCriteria[];
}

const normalizeWorkspaceSettings = (raw: RawWorkspaceSettings): WorkspaceSettings => ({
  settings: raw.settings ?? {
    id: '',
    workspace_id: '',
    quarter_start_date: '',
    sprint_duration_weeks: 2,
    notifications_enabled: true,
    auto_calculate_bonuses: false,
    team_weight: 50,
  },
  teams: raw.teams ?? [],
  people: raw.people ?? [],
  memberships: raw.memberships ?? [],
  managers: raw.managers ?? [],
  job_role_criteria: raw.job_role_criteria ?? raw.job_roles ?? [],
  bonus_tiers: raw.bonus_tiers ?? [],
});

export const settingsService = {
  getAll: async (workspaceId: string) => {
    const primary = await api.get<RawWorkspaceSettings>(`/settings?workspace_id=${encodeURIComponent(workspaceId)}`);
    if (primary.data) {
      return { data: normalizeWorkspaceSettings(primary.data), error: null };
    }

    // Backward-compatibility fallback for older API route shape.
    const fallback = await api.get<RawWorkspaceSettings>(`/settings/${workspaceId}`);
    if (fallback.data) {
      return { data: normalizeWorkspaceSettings(fallback.data), error: null };
    }

    return { data: null, error: primary.error ?? fallback.error };
  },
  initialize: (workspaceId: string) => api.post('/settings/initialize', { workspace_id: workspaceId }),
  createTeam: (data: { workspace_id: string; name: string; description?: string; manager_id?: string }) =>
    api.post<WorkspaceTeam>('/settings/teams', data),
  updateTeam: (id: string, data: Partial<WorkspaceTeam>) =>
    api.put<WorkspaceTeam>(`/settings/teams/${id}`, data),
  deleteTeam: (id: string) => api.del(`/settings/teams/${id}`),
  createPerson: (data: Omit<WorkspacePerson, 'id'>) =>
    api.post<WorkspacePerson>('/settings/people', data),
  updatePerson: (id: string, data: Partial<WorkspacePerson>) =>
    api.put<WorkspacePerson>(`/settings/people/${id}`, data),
  deletePerson: (id: string) => api.del(`/settings/people/${id}`),
  updateBonusTiers: (workspaceId: string, tiers: Omit<BonusTierConfig, 'id' | 'workspace_id'>[]) =>
    api.put('/settings/bonus-tiers', { workspace_id: workspaceId, tiers }),
  updateJobRoleCriteria: (workspaceId: string, jobRole: string, criteria: Omit<JobRoleCriteria, 'id' | 'workspace_id'>[]) =>
    api.put('/settings/job-roles', { workspace_id: workspaceId, job_role: jobRole, criteria }),
  deleteJobRole: (workspaceId: string, jobRole: string) =>
    api.del(`/settings/job-roles?workspace_id=${encodeURIComponent(workspaceId)}&job_role=${encodeURIComponent(jobRole)}`),
  updateSystem: (workspaceId: string, data: { team_weight?: number; sprint_duration_weeks?: number; notifications_enabled?: boolean; auto_calculate_bonuses?: boolean }) =>
    api.put(`/settings/system?workspace_id=${encodeURIComponent(workspaceId)}`, data),
};
