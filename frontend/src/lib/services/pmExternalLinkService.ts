import { api } from '../api';
import type {
  ExternalLink,
  CreateExternalLinkRequest,
  UpdateExternalLinkRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmExternalLinkService = {
  list: (workspaceId: string, taskId: string) =>
    api.get<ExternalLink[]>(`/pm/tasks/${taskId}/links?${qs(workspaceId)}`),

  create: (workspaceId: string, taskId: string, payload: CreateExternalLinkRequest) =>
    api.post<ExternalLink>(`/pm/tasks/${taskId}/links?${qs(workspaceId)}`, payload),

  update: (workspaceId: string, id: string, payload: UpdateExternalLinkRequest) =>
    api.put<ExternalLink>(`/pm/links/${id}?${qs(workspaceId)}`, payload),

  remove: (workspaceId: string, id: string) =>
    api.del(`/pm/links/${id}?${qs(workspaceId)}`),

  listByEntity: (workspaceId: string, entityType: string, entityId: string) =>
    api.get<ExternalLink[]>(`/pm/entity-links/${entityType}/${entityId}?${qs(workspaceId)}`),

  createForEntity: (workspaceId: string, entityType: string, entityId: string, payload: CreateExternalLinkRequest) =>
    api.post<ExternalLink>(`/pm/entity-links/${entityType}/${entityId}?${qs(workspaceId)}`, payload),
};
