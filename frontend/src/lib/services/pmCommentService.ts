import { api } from '../api';
import type {
  CommentWithAuthor,
  Comment,
  CreateCommentRequest,
  ReactionSummary,
  UpdateCommentRequest,
} from '../pmTypes';
import type { ConversationRewriteOperation } from '@/components/design-system/conversation-composer';

const qs = (workspaceId: string) => `workspace_id=${encodeURIComponent(workspaceId)}`;

export const pmCommentService = {
  list: (workspaceId: string, entityType: 'task' | 'epic' | 'doc', entityId: string) =>
    api.get<CommentWithAuthor[]>(
      `/pm/comments?${qs(workspaceId)}&entity_type=${encodeURIComponent(entityType)}&entity_id=${encodeURIComponent(entityId)}`
    ),
  create: (workspaceId: string, payload: CreateCommentRequest) => api.post<CommentWithAuthor>(`/pm/comments?${qs(workspaceId)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCommentRequest) =>
    api.put(`/pm/comments/${id}?${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/pm/comments/${id}?${qs(workspaceId)}`),
  resolve: (workspaceId: string, id: string) =>
    api.post<Comment>(`/pm/comments/${id}/resolve?${qs(workspaceId)}`),
  reopen: (workspaceId: string, id: string) =>
    api.post<Comment>(`/pm/comments/${id}/reopen?${qs(workspaceId)}`),
  toggleReaction: (workspaceId: string, commentId: string, emoji: string) =>
    api.post<ReactionSummary[]>(`/pm/comments/${commentId}/reactions?${qs(workspaceId)}`, { emoji }),
};

export const rewritePMCommentDraft = (workspaceId: string, content: string, operation: ConversationRewriteOperation) =>
  api.post<{ content: string; operation: ConversationRewriteOperation; provider: string; model: string }>(
    `/pm/rewrite-draft?${qs(workspaceId)}`,
    { content, operation },
  );
