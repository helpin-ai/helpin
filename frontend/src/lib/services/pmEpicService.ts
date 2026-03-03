import { api } from '../api';
import type {
  CreateEpicRequest,
  EpicWithStats,
  Story,
  UpdateEpicHealthRequest,
  UpdateEpicRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

const filterQuery = (filters: Record<string, string | boolean | undefined>) => {
  const search = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') return;
    search.set(key, String(value));
  });
  const str = search.toString();
  return str ? `&${str}` : '';
};

export const pmEpicService = {
  list: (
    workspaceId: string,
    filters?: {
      team_id?: string;
      state_id?: string;
      label_id?: string;
      archived?: boolean;
    }
  ) => api.get<EpicWithStats[]>(`/pm/epics${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  create: (payload: CreateEpicRequest) => api.post<EpicWithStats>(`/pm/epics${qs(payload.workspace_id)}`, payload),
  get: (workspaceId: string, id: string) => api.get<EpicWithStats>(`/pm/epics/${id}${qs(workspaceId)}`),
  update: (workspaceId: string, id: string, payload: UpdateEpicRequest) =>
    api.put<EpicWithStats>(`/pm/epics/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/epics/${id}${qs(workspaceId)}`),
  listStories: (workspaceId: string, id: string) => api.get<Story[]>(`/pm/epics/${id}/stories${qs(workspaceId)}`),
  updateHealth: (workspaceId: string, id: string, payload: UpdateEpicHealthRequest) =>
    api.put(`/pm/epics/${id}/health${qs(workspaceId)}`, payload),
};
