import { api } from '../api';
import type {
  CommentWithAuthor,
  CreateCommentRequest,
  UpdateCommentRequest,
} from '../pmTypes';

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmCommentService = {
  list: (workspaceId: string, entityType: 'story' | 'epic' | 'doc', entityId: string) =>
    api.get<CommentWithAuthor[]>(
      `/pm/comments?${qs(workspaceId)}&entity_type=${encodeURIComponent(entityType)}&entity_id=${encodeURIComponent(entityId)}`
    ),
  create: (workspaceId: string, payload: CreateCommentRequest) => api.post<CommentWithAuthor>(`/pm/comments?${qs(workspaceId)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCommentRequest) =>
    api.put(`/pm/comments/${id}?${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/comments/${id}?${qs(workspaceId)}`),
};
