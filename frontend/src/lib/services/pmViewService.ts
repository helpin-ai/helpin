import { api } from '../api';
import type { PMView, CreateViewRequest, UpdateViewRequest } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmViewService = {
  list: (workspaceId: string) => api.get<PMView[]>(`/pm/views${qs(workspaceId)}`),
  create: (workspaceId: string, payload: CreateViewRequest) =>
    api.post<PMView>(`/pm/views${qs(workspaceId)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateViewRequest) =>
    api.put<PMView>(`/pm/views/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/views/${id}${qs(workspaceId)}`),
};
