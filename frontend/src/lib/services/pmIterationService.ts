import { api } from '../api';
import type {
  CreateIterationRequest,
  IterationWithStats,
  Story,
  UpdateIterationRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

/** Convert date-only "YYYY-MM-DD" to RFC 3339 "YYYY-MM-DDT00:00:00Z" for Go's time.Time. */
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

export const pmIterationService = {
  list: (
    workspaceId: string,
    filters?: {
      team_id?: string;
      status?: string;
      archived?: boolean;
    }
  ) => api.get<IterationWithStats[]>(`/pm/iterations${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  create: (payload: CreateIterationRequest) =>
    api.post<IterationWithStats>(`/pm/iterations${qs(payload.workspace_id)}`, {
      ...payload,
      start_date: toRFC3339(payload.start_date),
      end_date: toRFC3339(payload.end_date),
    }),
  get: (workspaceId: string, id: string) => api.get<IterationWithStats>(`/pm/iterations/${id}${qs(workspaceId)}`),
  update: (workspaceId: string, id: string, payload: UpdateIterationRequest) =>
    api.put<IterationWithStats>(`/pm/iterations/${id}${qs(workspaceId)}`, {
      ...payload,
      start_date: toRFC3339(payload.start_date),
      end_date: toRFC3339(payload.end_date),
    }),
  remove: (workspaceId: string, id: string) => api.del(`/pm/iterations/${id}${qs(workspaceId)}`),
  listStories: (workspaceId: string, id: string) => api.get<Story[]>(`/pm/iterations/${id}/stories${qs(workspaceId)}`),
};
