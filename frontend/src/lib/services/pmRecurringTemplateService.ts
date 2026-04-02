import { api } from '../api';
import type {
  CreateRecurringTemplateRequest,
  RecurringTemplateDetail,
  TaskRecurringSummary,
  UpdateRecurringTemplateRequest,
} from '../pmTypes';

const qs = (
  workspaceId: string,
  opts?: { status?: string; teamId?: string; search?: string },
) => {
  const params = new URLSearchParams();
  params.set('workspace_id', workspaceId);
  if (opts?.status) params.set('status', opts.status);
  if (opts?.teamId) params.set('team_id', opts.teamId);
  if (opts?.search) params.set('search', opts.search);
  return `?${params.toString()}`;
};

export const pmRecurringTemplateService = {
  list: (workspaceId: string, opts?: { status?: string; teamId?: string; search?: string }) =>
    api.get<RecurringTemplateDetail[]>(`/pm/recurring-templates${qs(workspaceId, opts)}`),
  get: (workspaceId: string, id: string) =>
    api.get<RecurringTemplateDetail>(`/pm/recurring-templates/${id}${qs(workspaceId)}`),
  getByTask: (workspaceId: string, taskId: string) =>
    api.get<TaskRecurringSummary>(`/pm/tasks/${taskId}/recurring-template${qs(workspaceId)}`),
  create: (payload: CreateRecurringTemplateRequest) =>
    api.post<RecurringTemplateDetail>(`/pm/recurring-templates${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateRecurringTemplateRequest) =>
    api.put<RecurringTemplateDetail>(`/pm/recurring-templates/${id}${qs(workspaceId)}`, payload),
  pause: (workspaceId: string, id: string) =>
    api.post<RecurringTemplateDetail>(`/pm/recurring-templates/${id}/pause${qs(workspaceId)}`),
  resume: (workspaceId: string, id: string) =>
    api.post<RecurringTemplateDetail>(`/pm/recurring-templates/${id}/resume${qs(workspaceId)}`),
  stop: (workspaceId: string, id: string) =>
    api.post<RecurringTemplateDetail>(`/pm/recurring-templates/${id}/stop${qs(workspaceId)}`),
  skipNext: (workspaceId: string, id: string) =>
    api.post<RecurringTemplateDetail>(`/pm/recurring-templates/${id}/skip-next${qs(workspaceId)}`),
  generateNow: (workspaceId: string, id: string) =>
    api.post<RecurringTemplateDetail>(`/pm/recurring-templates/${id}/generate-now${qs(workspaceId)}`),
  duplicate: (workspaceId: string, id: string) =>
    api.post<RecurringTemplateDetail>(`/pm/recurring-templates/${id}/duplicate${qs(workspaceId)}`),
};
