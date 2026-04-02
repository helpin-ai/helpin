import { api } from '../api';
import type { CreateTaskTemplateRequest, TaskTemplate, UpdateTaskTemplateRequest } from '../pmTypes';

const qs = (workspaceId: string, opts?: { teamId?: string; includeShared?: boolean; archived?: boolean }) => {
  const params = new URLSearchParams();
  params.set('workspace_id', workspaceId);
  if (opts?.teamId) params.set('team_id', opts.teamId);
  if (opts?.includeShared !== undefined) params.set('include_shared', String(opts.includeShared));
  if (opts?.archived !== undefined) params.set('archived', String(opts.archived));
  return `?${params.toString()}`;
};

export const pmTaskTemplateService = {
  list: (workspaceId: string, opts?: { teamId?: string; includeShared?: boolean; archived?: boolean }) =>
    api.get<TaskTemplate[]>(`/pm/task-templates${qs(workspaceId, opts)}`),
  get: (workspaceId: string, id: string) =>
    api.get<TaskTemplate>(`/pm/task-templates/${id}${qs(workspaceId)}`),
  create: (payload: CreateTaskTemplateRequest) =>
    api.post<TaskTemplate>(`/pm/task-templates${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateTaskTemplateRequest) =>
    api.put<TaskTemplate>(`/pm/task-templates/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/pm/task-templates/${id}${qs(workspaceId)}`),
};
