import { api } from '../api';
import type {
  ActivityLogEntry,
  CreateEpicRequest,
  EpicWithStats,
  PaginatedResponse,
  Task,
  UpdateEpicHealthRequest,
  UpdateEpicRequest,
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
  create: (payload: CreateEpicRequest) =>
    api.post<EpicWithStats>(`/pm/epics${qs(payload.workspace_id)}`, {
      ...payload,
      planned_start_date: toRFC3339(payload.planned_start_date),
      deadline: toRFC3339(payload.deadline),
    }),
  get: (workspaceId: string, id: string) => api.get<EpicWithStats>(`/pm/epics/${id}${qs(workspaceId)}`),
  listActivity: (workspaceId: string, id: string, page = 1, perPage = 50) =>
    api.get<PaginatedResponse<ActivityLogEntry[]>>(
      `/pm/epics/${id}/activity${qs(workspaceId)}&page=${page}&per_page=${perPage}`
    ),
  update: (workspaceId: string, id: string, payload: UpdateEpicRequest) =>
    api.put<EpicWithStats>(`/pm/epics/${id}${qs(workspaceId)}`, {
      ...payload,
      planned_start_date: toRFC3339(payload.planned_start_date),
      deadline: toRFC3339(payload.deadline),
    }),
  remove: (workspaceId: string, id: string) => api.del(`/pm/epics/${id}${qs(workspaceId)}`),
  listTasks: (workspaceId: string, id: string) => api.get<Task[]>(`/pm/epics/${id}/tasks${qs(workspaceId)}`),
  updateHealth: (workspaceId: string, id: string, payload: UpdateEpicHealthRequest) =>
    api.put(`/pm/epics/${id}/health${qs(workspaceId)}`, payload),
};
