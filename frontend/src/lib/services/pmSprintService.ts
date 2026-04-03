import { api } from '../api';
import type {
  CreateSprintRequest,
  PaginatedResponse,
  SprintPlanningFilters,
  SprintPlanningTaskPreview,
  SprintPlanningWorkspace,
  SprintWithStats,
  Task,
  UpdateSprintRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

/** Convert date-only "YYYY-MM-DD" to RFC 3339 "YYYY-MM-DDT00:00:00Z" for Go's time.Time. */
const toRFC3339 = (v: string | undefined): string | undefined =>
  v && /^\d{4}-\d{2}-\d{2}$/.test(v) ? `${v}T00:00:00Z` : v;

const filterQuery = (filters: Record<string, string | number | boolean | undefined>) => {
  const search = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') return;
    search.set(key, String(value));
  });
  const str = search.toString();
  return str ? `&${str}` : '';
};

const planningFilterQuery = (filters: SprintPlanningFilters) =>
  filterQuery({
    team_id: filters.team_id,
    include_completed: filters.include_completed,
  });

export const pmSprintService = {
  list: (
    workspaceId: string,
    filters?: {
      team_id?: string;
      status?: string;
      archived?: boolean;
    }
  ) => api.get<SprintWithStats[]>(`/pm/sprints${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  planningWorkspace: (
    workspaceId: string,
    filters?: SprintPlanningFilters,
  ) => api.get<SprintPlanningWorkspace>(`/pm/sprints/planning${qs(workspaceId)}${planningFilterQuery(filters ?? {})}`),
  create: (payload: CreateSprintRequest) =>
    api.post<SprintWithStats>(`/pm/sprints${qs(payload.workspace_id)}`, {
      ...payload,
      start_date: toRFC3339(payload.start_date),
      end_date: toRFC3339(payload.end_date),
    }),
  get: (workspaceId: string, id: string) => api.get<SprintWithStats>(`/pm/sprints/${id}${qs(workspaceId)}`),
  update: (workspaceId: string, id: string, payload: UpdateSprintRequest) =>
    api.put<SprintWithStats>(`/pm/sprints/${id}${qs(workspaceId)}`, {
      ...payload,
      start_date: toRFC3339(payload.start_date),
      end_date: toRFC3339(payload.end_date),
    }),
  remove: (workspaceId: string, id: string) => api.del(`/pm/sprints/${id}${qs(workspaceId)}`),
  listTasks: (workspaceId: string, id: string) => api.get<Task[]>(`/pm/sprints/${id}/tasks${qs(workspaceId)}`),
  listPreviewTasks: (
    workspaceId: string,
    id: string,
    pagination?: { page?: number; per_page?: number },
  ) => api.get<PaginatedResponse<SprintPlanningTaskPreview[]>>(
    `/pm/sprints/${id}/preview-tasks${qs(workspaceId)}${filterQuery(pagination ?? {})}`,
  ),
};
