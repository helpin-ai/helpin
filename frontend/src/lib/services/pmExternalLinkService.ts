import { api } from '../api';
import type {
  ExternalLink,
  CreateExternalLinkRequest,
  UpdateExternalLinkRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmExternalLinkService = {
  list: (workspaceId: string, storyId: string) =>
    api.get<ExternalLink[]>(`/pm/tasks/${storyId}/links?${qs(workspaceId)}`),

  create: (workspaceId: string, storyId: string, payload: CreateExternalLinkRequest) =>
    api.post<ExternalLink>(`/pm/tasks/${storyId}/links?${qs(workspaceId)}`, payload),

  update: (workspaceId: string, id: string, payload: UpdateExternalLinkRequest) =>
    api.put<ExternalLink>(`/pm/links/${id}?${qs(workspaceId)}`, payload),

  remove: (workspaceId: string, id: string) =>
    api.del(`/pm/links/${id}?${qs(workspaceId)}`),
};
