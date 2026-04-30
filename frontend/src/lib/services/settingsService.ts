import { api } from '../api';
import type {
  WorkspaceSettings,
  WorkspaceTeam,
  WorkspacePerson,
  JobRoleCriteria,
  InvitationTeamPreassignment,
  TeamEstimateSettings,
  TeamFieldVisibility,
  EstimateScale,
  TeamRepoDefault,
  WorkspaceModuleAccessSettings,
  WorkspaceModuleGrant,
} from '../types';

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
    team_weight: 50,
  },
  teams: raw.teams ?? [],
  people: raw.people ?? [],
  memberships: raw.memberships ?? [],
  user_memberships: raw.user_memberships ?? [],
  managers: raw.managers ?? [],
  job_role_criteria: raw.job_role_criteria ?? raw.job_roles ?? [],
  invitation_team_preassignments: raw.invitation_team_preassignments ?? [],
  team_estimate_settings: raw.team_estimate_settings ?? [],
  team_field_visibility: raw.team_field_visibility ?? [],
  team_repo_defaults: raw.team_repo_defaults ?? [],
});

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const settingsService = {
  getAll: async (workspaceId: string) => {
    const primary = await api.get<RawWorkspaceSettings>(`/settings${qs(workspaceId)}`);
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
  initialize: (workspaceId: string) => api.post(`/settings/initialize${qs(workspaceId)}`, { workspace_id: workspaceId }),
  createTeam: (data: { workspace_id: string; name: string; handle?: string; description?: string; manager_id?: string; team_type?: WorkspaceTeam['team_type']; default_task_type?: WorkspaceTeam['default_task_type'] }) =>
    api.post<WorkspaceTeam>(`/settings/teams${qs(data.workspace_id)}`, data),
  ensureDefaultTeam: (data: { workspace_id: string; team_type: WorkspaceTeam['team_type'] }) =>
    api.post<WorkspaceTeam>(`/settings/teams/ensure-default${qs(data.workspace_id)}`, data),
  updateTeam: (workspaceId: string, id: string, data: Partial<WorkspaceTeam>) =>
    api.put<WorkspaceTeam>(`/settings/teams/${id}${qs(workspaceId)}`, data),
  deleteTeam: (workspaceId: string, id: string) => api.del(`/settings/teams/${id}${qs(workspaceId)}`),
  addTeamMember: (workspaceId: string, teamId: string, data: { user_id: string; role?: 'owner' | 'member' }) =>
    api.post(`/settings/teams/${teamId}/members${qs(workspaceId)}`, data),
  updateTeamMember: (workspaceId: string, teamId: string, userId: string, data: { role?: 'owner' | 'member' }) =>
    api.put(`/settings/teams/${teamId}/members/${userId}${qs(workspaceId)}`, data),
  removeTeamMember: (workspaceId: string, teamId: string, userId: string) =>
    api.del(`/settings/teams/${teamId}/members/${userId}${qs(workspaceId)}`),
  createPerson: (workspaceId: string, data: Omit<WorkspacePerson, 'id'>) =>
    api.post<WorkspacePerson>(`/settings/people${qs(workspaceId)}`, data),
  updatePerson: (workspaceId: string, id: string, data: Partial<WorkspacePerson>) =>
    api.put<WorkspacePerson>(`/settings/people/${id}${qs(workspaceId)}`, data),
  deletePerson: (workspaceId: string, id: string) => api.del(`/settings/people/${id}${qs(workspaceId)}`),
  updateJobRoleCriteria: (workspaceId: string, jobRole: string, criteria: Omit<JobRoleCriteria, 'id' | 'workspace_id'>[]) =>
    api.put(`/settings/job-roles${qs(workspaceId)}`, { workspace_id: workspaceId, job_role: jobRole, criteria }),
  deleteJobRole: (workspaceId: string, jobRole: string) =>
    api.del(`/settings/job-roles${qs(workspaceId)}&job_role=${encodeURIComponent(jobRole)}`),
  updateSystem: (workspaceId: string, data: { team_weight?: number; sprint_duration_weeks?: number; notifications_enabled?: boolean }) =>
    api.put(`/settings/system${qs(workspaceId)}`, data),
  addTeamInvitation: (workspaceId: string, teamId: string, invitationId: string) =>
    api.post<InvitationTeamPreassignment>(`/settings/teams/${teamId}/invitations${qs(workspaceId)}`, { invitation_id: invitationId }),
  removeTeamInvitation: (workspaceId: string, teamId: string, invitationId: string) =>
    api.del(`/settings/teams/${teamId}/invitations/${invitationId}${qs(workspaceId)}`),
  getTeamEstimateSettings: (workspaceId: string, teamId: string) =>
    api.get<TeamEstimateSettings>(`/settings/teams/${teamId}/estimates${qs(workspaceId)}`),
  updateTeamEstimateSettings: (workspaceId: string, teamId: string, data: { enabled?: boolean; scale?: EstimateScale; extended?: boolean; allow_zero?: boolean; count_unestimated_as_one?: boolean }) =>
    api.put<TeamEstimateSettings>(`/settings/teams/${teamId}/estimates${qs(workspaceId)}`, data),
  getTeamFieldVisibility: (workspaceId: string, teamId: string) =>
    api.get<TeamFieldVisibility>(`/settings/teams/${teamId}/field-visibility${qs(workspaceId)}`),
  updateTeamFieldVisibility: (workspaceId: string, teamId: string, data: Partial<Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'>>) =>
    api.put<TeamFieldVisibility>(`/settings/teams/${teamId}/field-visibility${qs(workspaceId)}`, data),
  getTeamRepoDefault: (workspaceId: string, teamId: string) =>
    api.get<TeamRepoDefault>(`/settings/teams/${teamId}/repo-default${qs(workspaceId)}`),
  updateTeamRepoDefault: (
    workspaceId: string,
    teamId: string,
    data: {
      repository_id: string;
      base_branch?: string;
      branch_template?: string;
      auto_sync_states?: boolean;
      review_state_id?: string;
      done_state_id?: string;
      closed_state_id?: string;
    },
  ) => api.put<TeamRepoDefault>(`/settings/teams/${teamId}/repo-default${qs(workspaceId)}`, data),
  getModuleAccess: (workspaceId: string) =>
    api.get<WorkspaceModuleAccessSettings>(`/settings/module-access${qs(workspaceId)}`),
  createModuleGrant: (workspaceId: string, data: { module: 'crm' | 'support'; subject_type: 'team' | 'workspace_member'; subject_id: string }) =>
    api.post<WorkspaceModuleGrant>(`/settings/module-access${qs(workspaceId)}`, { workspace_id: workspaceId, ...data }),
  deleteModuleGrant: (workspaceId: string, grantId: string) =>
    api.del(`/settings/module-access/${grantId}${qs(workspaceId)}`),
};
