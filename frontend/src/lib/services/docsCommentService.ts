import { api } from '../api'
import type {
  CommentWithAuthor,
  Comment,
  CreateCommentRequest,
  ReactionSummary,
  UpdateCommentRequest,
} from '../pmTypes'

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`

export const docsCommentService = {
  list: (workspaceId: string, _entityType: 'task' | 'epic' | 'doc', entityId: string) =>
    api.get<CommentWithAuthor[]>(`/docs/documents/${entityId}/comments?${qs(workspaceId)}`),
  create: (workspaceId: string, payload: CreateCommentRequest) =>
    api.post<CommentWithAuthor>(`/docs/documents/${payload.entity_id}/comments?${qs(workspaceId)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCommentRequest) =>
    api.put(`/docs/comments/${id}?${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/docs/comments/${id}?${qs(workspaceId)}`),
  resolve: (workspaceId: string, id: string) =>
    api.post<Comment>(`/docs/comments/${id}/resolve?${qs(workspaceId)}`),
  reopen: (workspaceId: string, id: string) =>
    api.post<Comment>(`/docs/comments/${id}/reopen?${qs(workspaceId)}`),
  toggleReaction: (workspaceId: string, commentId: string, emoji: string) =>
    api.post<ReactionSummary[]>(`/docs/comments/${commentId}/reactions?${qs(workspaceId)}`, { emoji }),
}
