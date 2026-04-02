import { api } from '../api';
import type {
  ActivityLogEntry,
  ColumnTasksResponse,
  CreateTaskRequest,
  MoveTaskRequest,
  PaginatedResponse,
  ReorderTaskRequest,
  Task,
  TaskDetail,
  TaskLabelLinkRequest,
  TaskMemberColumn,
  TaskStateColumn,
  TaskStateCount,
  TaskUserLinkRequest,
  UpdateTaskRequest,
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

export const pmTaskService = {
  list: (
    workspaceId: string,
    filters?: {
      page?: number;
      per_page?: number;
      team_id?: string;
      epic_id?: string;
      sprint_id?: string;
      contact_id?: string;
      company_id?: string;
      deal_id?: string;
      support_conversation_id?: string;
      include_contacts?: boolean;
      include_companies?: boolean;
      include_deals?: boolean;
      include_support?: boolean;
      workflow_id?: string;
      state_id?: string;
      task_type?: string;
      owner_member_id?: string;
      requester_member_id?: string;
      label_id?: string;
      priority?: string;
      severity?: string;
      blocked?: string;
      blocking?: string;
      archived?: boolean;
    }
  ) =>
    api.get<PaginatedResponse<Task[]>>(
      withFilters('/pm/tasks', {
        workspace_id: workspaceId,
        ...(filters ?? {}),
      })
    ),
  listBoard: (
    workspaceId: string,
    workflowId: string,
    filters?: Record<string, string | undefined>,
    perStateLimit?: number,
    includeOptions?: {
      include_contacts?: boolean;
      include_companies?: boolean;
      include_deals?: boolean;
      include_support?: boolean;
    }
  ) => {
    const params = new URLSearchParams();
    params.set('workspace_id', workspaceId);
    params.set('workflow_id', workflowId);
    if (perStateLimit && perStateLimit > 0) {
      params.set('per_state_limit', String(perStateLimit));
    }
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value) params.set(key, value);
      });
    }
    if (includeOptions?.include_contacts) params.set('include_contacts', 'true');
    if (includeOptions?.include_companies) params.set('include_companies', 'true');
    if (includeOptions?.include_deals) params.set('include_deals', 'true');
    if (includeOptions?.include_support) params.set('include_support', 'true');
    return api.get<TaskStateColumn[]>(`/pm/tasks/board?${params.toString()}`);
  },
  listBoardColumn: (
    workspaceId: string,
    stateId: string,
    offset: number,
    limit: number,
    filters?: Record<string, string | undefined>,
    includeOptions?: {
      include_contacts?: boolean;
      include_companies?: boolean;
      include_deals?: boolean;
      include_support?: boolean;
    }
  ) => {
    const params = new URLSearchParams();
    params.set('workspace_id', workspaceId);
    params.set('state_id', stateId);
    params.set('offset', String(offset));
    params.set('limit', String(limit));
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value) params.set(key, value);
      });
    }
    if (includeOptions?.include_contacts) params.set('include_contacts', 'true');
    if (includeOptions?.include_companies) params.set('include_companies', 'true');
    if (includeOptions?.include_deals) params.set('include_deals', 'true');
    if (includeOptions?.include_support) params.set('include_support', 'true');
    return api.get<ColumnTasksResponse>(`/pm/tasks/board/column?${params.toString()}`);
  },
  listBoardByMember: (
    workspaceId: string,
    workflowId: string,
    filters?: Record<string, string | undefined>,
    perMemberLimit?: number,
    includeEmpty?: boolean,
    memberIds?: string[]
  ) => {
    const params = new URLSearchParams();
    params.set('workspace_id', workspaceId);
    params.set('workflow_id', workflowId);
    if (perMemberLimit && perMemberLimit > 0) params.set('per_member_limit', String(perMemberLimit));
    if (includeEmpty) params.set('include_empty', 'true');
    if (memberIds?.length) params.set('member_ids', memberIds.join(','));
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value) params.set(key, value);
      });
    }
    return api.get<TaskMemberColumn[]>(`/pm/tasks/board/members?${params.toString()}`);
  },
  listBoardMemberColumn: (
    workspaceId: string,
    workflowId: string,
    memberId: string | null,
    offset: number,
    limit: number,
    filters?: Record<string, string | undefined>
  ) => {
    const params = new URLSearchParams();
    params.set('workspace_id', workspaceId);
    params.set('workflow_id', workflowId);
    if (memberId) params.set('member_id', memberId);
    params.set('offset', String(offset));
    params.set('limit', String(limit));
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value) params.set(key, value);
      });
    }
    return api.get<ColumnTasksResponse>(`/pm/tasks/board/members/column?${params.toString()}`);
  },
  countByState: (workspaceId: string, workflowId: string) =>
    api.get<TaskStateCount[]>(`/pm/tasks/counts?${qs(workspaceId)}&workflow_id=${encodeURIComponent(workflowId)}`),
  create: (payload: CreateTaskRequest) =>
    api.post<TaskDetail>(`/pm/tasks?${qs(payload.workspace_id)}`, {
      ...payload,
      deadline: toRFC3339(payload.deadline),
    }),
  get: (workspaceId: string, id: string) => api.get<TaskDetail>(`/pm/tasks/${id}?${qs(workspaceId)}`),
  getByDisplayId: (workspaceId: string, displayId: number) =>
    api.get<TaskDetail>(`/pm/tasks/display/${displayId}?${qs(workspaceId)}`),
  update: (workspaceId: string, id: string, payload: UpdateTaskRequest) =>
    api.put<TaskDetail>(`/pm/tasks/${id}?${qs(workspaceId)}`, {
      ...payload,
      deadline: toRFC3339(payload.deadline),
    }),
  remove: (workspaceId: string, id: string) => api.del(`/pm/tasks/${id}?${qs(workspaceId)}`),
  move: (workspaceId: string, id: string, payload: MoveTaskRequest) =>
    api.put<TaskDetail>(`/pm/tasks/${id}/move?${qs(workspaceId)}`, payload),
  reorder: (workspaceId: string, id: string, payload: ReorderTaskRequest) =>
    api.put(`/pm/tasks/${id}/reorder?${qs(workspaceId)}`, payload),
  addOwner: (workspaceId: string, id: string, payload: TaskUserLinkRequest) =>
    api.post(`/pm/tasks/${id}/owners?${qs(workspaceId)}`, payload),
  removeOwner: (workspaceId: string, id: string, userId: string) =>
    api.del(`/pm/tasks/${id}/owners/${userId}?${qs(workspaceId)}`),
  addFollower: (workspaceId: string, id: string, payload: TaskUserLinkRequest) =>
    api.post(`/pm/tasks/${id}/followers?${qs(workspaceId)}`, payload),
  removeFollower: (workspaceId: string, id: string, userId?: string) =>
    api.del(`/pm/tasks/${id}/followers?${qs(workspaceId)}${userId ? `&user_id=${encodeURIComponent(userId)}` : ''}`),
  addLabel: (workspaceId: string, id: string, payload: TaskLabelLinkRequest) =>
    api.post(`/pm/tasks/${id}/labels?${qs(workspaceId)}`, payload),
  removeLabel: (workspaceId: string, id: string, labelId: string) =>
    api.del(`/pm/tasks/${id}/labels/${labelId}?${qs(workspaceId)}`),
  syncLabels: (workspaceId: string, taskId: string, currentIds: string[], nextIds: string[]) => {
    const current = new Set(currentIds);
    const next = new Set(nextIds);
    return Promise.all([
      ...nextIds.filter((id) => !current.has(id)).map((id) =>
        api.post(`/pm/tasks/${taskId}/labels?${qs(workspaceId)}`, { label_id: id }),
      ),
      ...[...current].filter((id) => !next.has(id)).map((id) =>
        api.del(`/pm/tasks/${taskId}/labels/${id}?${qs(workspaceId)}`),
      ),
    ]);
  },
  listActivity: (workspaceId: string, id: string, page = 1, perPage = 50) =>
    api.get<PaginatedResponse<ActivityLogEntry[]>>(
      `/pm/tasks/${id}/activity?${qs(workspaceId)}&page=${page}&per_page=${perPage}`
    ),
};
