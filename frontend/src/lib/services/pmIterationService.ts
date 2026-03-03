import { api } from '../api';
import type {
  CreateIterationRequest,
  IterationWithStats,
  Story,
  UpdateIterationRequest,
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

export const pmIterationService = {
  list: (
    workspaceId: string,
    filters?: {
      team_id?: string;
      status?: string;
      archived?: boolean;
    }
  ) => api.get<IterationWithStats[]>(`/pm/iterations${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  create: (payload: CreateIterationRequest) => api.post<IterationWithStats>(`/pm/iterations${qs(payload.workspace_id)}`, payload),
  get: (workspaceId: string, id: string) => api.get<IterationWithStats>(`/pm/iterations/${id}${qs(workspaceId)}`),
  update: (workspaceId: string, id: string, payload: UpdateIterationRequest) =>
    api.put<IterationWithStats>(`/pm/iterations/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/iterations/${id}${qs(workspaceId)}`),
  listStories: (workspaceId: string, id: string) => api.get<Story[]>(`/pm/iterations/${id}/stories${qs(workspaceId)}`),
};
