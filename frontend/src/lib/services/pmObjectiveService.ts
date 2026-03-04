import { api } from '../api';
import type {
  CreateKeyResultRequest,
  CreateObjectiveRequest,
  KeyResult,
  ObjectiveWithDetails,
  UpdateKeyResultRequest,
  UpdateObjectiveRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

const toRFC3339 = (v: string | undefined): string | undefined =>
  v && /^\d{4}-\d{2}-\d{2}$/.test(v) ? `${v}T00:00:00Z` : v;

const filterQuery = (filters: Record<string, string | boolean | undefined>) => {
  const search = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') return;
    search.set(key, String(value));
  });
  const str = search.toString();
  return str ? `&${str}` : '';
};

export const pmObjectiveService = {
  list: (
    workspaceId: string,
    filters?: {
      team_id?: string;
      label_id?: string;
      objective_type?: string;
      state?: string;
      archived?: boolean;
    }
  ) => api.get<ObjectiveWithDetails[]>(`/pm/objectives${qs(workspaceId)}${filterQuery(filters ?? {})}`),

  get: (workspaceId: string, id: string) =>
    api.get<ObjectiveWithDetails>(`/pm/objectives/${id}${qs(workspaceId)}`),

  create: (payload: CreateObjectiveRequest) =>
    api.post<ObjectiveWithDetails>(`/pm/objectives${qs(payload.workspace_id)}`, {
      ...payload,
      planned_start_date: toRFC3339(payload.planned_start_date),
      deadline: toRFC3339(payload.deadline),
    }),

  update: (workspaceId: string, id: string, payload: UpdateObjectiveRequest) =>
    api.put<ObjectiveWithDetails>(`/pm/objectives/${id}${qs(workspaceId)}`, {
      ...payload,
      planned_start_date: toRFC3339(payload.planned_start_date),
      deadline: toRFC3339(payload.deadline),
    }),

  remove: (workspaceId: string, id: string) =>
    api.del(`/pm/objectives/${id}${qs(workspaceId)}`),

  addTeam: (workspaceId: string, id: string, teamId: string) =>
    api.post(`/pm/objectives/${id}/teams${qs(workspaceId)}`, { team_id: teamId }),

  removeTeam: (workspaceId: string, id: string, teamId: string) =>
    api.del(`/pm/objectives/${id}/teams/${teamId}${qs(workspaceId)}`),

  addOwner: (workspaceId: string, id: string, userId: string) =>
    api.post(`/pm/objectives/${id}/owners${qs(workspaceId)}`, { user_id: userId }),

  removeOwner: (workspaceId: string, id: string, userId: string) =>
    api.del(`/pm/objectives/${id}/owners/${userId}${qs(workspaceId)}`),

  addEpic: (workspaceId: string, id: string, epicId: string) =>
    api.post(`/pm/objectives/${id}/epics${qs(workspaceId)}`, { epic_id: epicId }),

  removeEpic: (workspaceId: string, id: string, epicId: string) =>
    api.del(`/pm/objectives/${id}/epics/${epicId}${qs(workspaceId)}`),

  createKeyResult: (workspaceId: string, objectiveId: string, payload: CreateKeyResultRequest) =>
    api.post<KeyResult>(`/pm/objectives/${objectiveId}/key-results${qs(workspaceId)}`, payload),

  updateKeyResult: (workspaceId: string, id: string, payload: UpdateKeyResultRequest) =>
    api.put<KeyResult>(`/pm/key-results/${id}${qs(workspaceId)}`, payload),

  deleteKeyResult: (workspaceId: string, id: string) =>
    api.del(`/pm/key-results/${id}${qs(workspaceId)}`),
};
