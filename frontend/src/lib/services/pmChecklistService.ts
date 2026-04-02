import { api } from '../api';
import type {
  ChecklistItem,
  CreateChecklistItemRequest,
  UpdateChecklistItemRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmChecklistService = {
  list: (workspaceId: string, storyId: string) =>
    api.get<ChecklistItem[]>(`/pm/tasks/${storyId}/checklist?${qs(workspaceId)}`),

  create: (workspaceId: string, storyId: string, payload: CreateChecklistItemRequest) =>
    api.post<ChecklistItem>(`/pm/tasks/${storyId}/checklist?${qs(workspaceId)}`, payload),

  update: (workspaceId: string, id: string, payload: UpdateChecklistItemRequest) =>
    api.put<ChecklistItem>(`/pm/checklist-items/${id}?${qs(workspaceId)}`, payload),

  remove: (workspaceId: string, id: string) =>
    api.del(`/pm/checklist-items/${id}?${qs(workspaceId)}`),
};
