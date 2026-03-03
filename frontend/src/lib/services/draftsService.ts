import { api } from '../api';
import type { GoalDraft } from '../types';

export const draftsService = {
  list: (workspaceId: string, quarterId: string) =>
    api.get<GoalDraft[]>(`/drafts?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  create: (data: { workspace_id: string; quarter_id: string; draft_data: Record<string, unknown> }) =>
    api.post<GoalDraft>('/drafts', data),
  get: (id: string) => api.get<GoalDraft>(`/drafts/${id}`),
  update: (id: string, data: { draft_data?: Record<string, unknown>; status?: string }) =>
    api.patch<GoalDraft>(`/drafts/${id}`, data),
  delete: (id: string) => api.del(`/drafts/${id}`),
};
