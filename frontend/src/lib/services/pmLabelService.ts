import { api } from '../api';
import type { CreateLabelRequest, Label, UpdateLabelRequest } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmLabelService = {
  list: (workspaceId: string) => api.get<Label[]>(`/pm/labels${qs(workspaceId)}`),
  create: (payload: CreateLabelRequest) => api.post<Label>('/pm/labels', payload),
  update: (workspaceId: string, id: string, payload: UpdateLabelRequest) =>
    api.put<Label>(`/pm/labels/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/labels/${id}${qs(workspaceId)}`),
};
