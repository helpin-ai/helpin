import { api } from '../api';
import type {
  ActivityLogEntry,
  CreateStoryRequest,
  MoveStoryRequest,
  PaginatedResponse,
  ReorderStoryRequest,
  Story,
  StoryDetail,
  StoryLabelLinkRequest,
  StoryStateColumn,
  StoryStateCount,
  StoryUserLinkRequest,
  UpdateStoryRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

/** Convert date-only "YYYY-MM-DD" to RFC 3339 "YYYY-MM-DDT00:00:00Z" for Go's time.Time. */
const toRFC3339 = (v: string | undefined): string | undefined =>
  v && /^\d{4}-\d{2}-\d{2}$/.test(v) ? `${v}T00:00:00Z` : v;

const withFilters = (base: string, filters: Record<string, string | number | boolean | undefined>) => {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') return;
    params.set(key, String(value));
  });
  return `${base}?${params.toString()}`;
};

export const pmStoryService = {
  list: (
    workspaceId: string,
    filters?: {
      page?: number;
      per_page?: number;
      team_id?: string;
      epic_id?: string;
      sprint_id?: string;
      workflow_id?: string;
      state_id?: string;
      story_type?: string;
      owner_id?: string;
      label_id?: string;
      priority?: string;
      archived?: boolean;
    }
  ) =>
    api.get<PaginatedResponse<Story[]>>(
      withFilters('/pm/stories', {
        workspace_id: workspaceId,
        ...(filters ?? {}),
      })
    ),
  listBoard: (
    workspaceId: string,
    workflowId: string,
    filters?: Record<string, string | undefined>
  ) => {
    const params = new URLSearchParams();
    params.set('workspace_id', workspaceId);
    params.set('workflow_id', workflowId);
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value) params.set(key, value);
      });
    }
    return api.get<StoryStateColumn[]>(`/pm/stories/board?${params.toString()}`);
  },
  countByState: (workspaceId: string, workflowId: string) =>
    api.get<StoryStateCount[]>(`/pm/stories/counts?${qs(workspaceId)}&workflow_id=${encodeURIComponent(workflowId)}`),
  create: (payload: CreateStoryRequest) =>
    api.post<StoryDetail>(`/pm/stories?${qs(payload.workspace_id)}`, {
      ...payload,
      deadline: toRFC3339(payload.deadline),
    }),
  get: (workspaceId: string, id: string) => api.get<StoryDetail>(`/pm/stories/${id}?${qs(workspaceId)}`),
  getByDisplayId: (workspaceId: string, displayId: number) =>
    api.get<StoryDetail>(`/pm/stories/display/${displayId}?${qs(workspaceId)}`),
  update: (workspaceId: string, id: string, payload: UpdateStoryRequest) =>
    api.put<StoryDetail>(`/pm/stories/${id}?${qs(workspaceId)}`, {
      ...payload,
      deadline: toRFC3339(payload.deadline),
    }),
  remove: (workspaceId: string, id: string) => api.del(`/pm/stories/${id}?${qs(workspaceId)}`),
  move: (workspaceId: string, id: string, payload: MoveStoryRequest) =>
    api.put<StoryDetail>(`/pm/stories/${id}/move?${qs(workspaceId)}`, payload),
  reorder: (workspaceId: string, id: string, payload: ReorderStoryRequest) =>
    api.put(`/pm/stories/${id}/reorder?${qs(workspaceId)}`, payload),
  addOwner: (workspaceId: string, id: string, payload: StoryUserLinkRequest) =>
    api.post(`/pm/stories/${id}/owners?${qs(workspaceId)}`, payload),
  removeOwner: (workspaceId: string, id: string, userId: string) =>
    api.del(`/pm/stories/${id}/owners/${userId}?${qs(workspaceId)}`),
  addFollower: (workspaceId: string, id: string, payload: StoryUserLinkRequest) =>
    api.post(`/pm/stories/${id}/followers?${qs(workspaceId)}`, payload),
  removeFollower: (workspaceId: string, id: string, userId?: string) =>
    api.del(`/pm/stories/${id}/followers?${qs(workspaceId)}${userId ? `&user_id=${encodeURIComponent(userId)}` : ''}`),
  addLabel: (workspaceId: string, id: string, payload: StoryLabelLinkRequest) =>
    api.post(`/pm/stories/${id}/labels?${qs(workspaceId)}`, payload),
  removeLabel: (workspaceId: string, id: string, labelId: string) =>
    api.del(`/pm/stories/${id}/labels/${labelId}?${qs(workspaceId)}`),
  syncLabels: (workspaceId: string, storyId: string, currentIds: string[], nextIds: string[]) => {
    const current = new Set(currentIds);
    const next = new Set(nextIds);
    return Promise.all([
      ...nextIds.filter((id) => !current.has(id)).map((id) =>
        api.post(`/pm/stories/${storyId}/labels?${qs(workspaceId)}`, { label_id: id }),
      ),
      ...[...current].filter((id) => !next.has(id)).map((id) =>
        api.del(`/pm/stories/${storyId}/labels/${id}?${qs(workspaceId)}`),
      ),
    ]);
  },
  listActivity: (workspaceId: string, id: string, page = 1, perPage = 50) =>
    api.get<PaginatedResponse<ActivityLogEntry[]>>(
      `/pm/stories/${id}/activity?${qs(workspaceId)}&page=${page}&per_page=${perPage}`
    ),
};
