import { api } from '../api';
import type { CreateLabelRequest, Label, LabelWithStats, UpdateLabelRequest } from '../pmTypes';

const qs = (workspaceId: string, opts?: { teamId?: string; includeShared?: boolean; archived?: boolean }) => {
  const params = new URLSearchParams();
  params.set('workspace_id', workspaceId);
  if (opts?.teamId) params.set('team_id', opts.teamId);
  if (opts?.includeShared !== undefined) params.set('include_shared', String(opts.includeShared));
  if (opts?.archived !== undefined) params.set('archived', String(opts.archived));
  return `?${params.toString()}`;
};

export const pmLabelService = {
  list: (workspaceId: string, opts?: { teamId?: string; includeShared?: boolean }) =>
    api.get<Label[]>(`/pm/labels${qs(workspaceId, opts)}`),
  listWithStats: (workspaceId: string, opts?: { teamId?: string; includeShared?: boolean; archived?: boolean }) =>
    api.get<LabelWithStats[]>(`/pm/labels/stats${qs(workspaceId, opts)}`),
  create: (payload: CreateLabelRequest) => api.post<Label>(`/pm/labels${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateLabelRequest) =>
    api.put<Label>(`/pm/labels/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/labels/${id}${qs(workspaceId)}`),
};
