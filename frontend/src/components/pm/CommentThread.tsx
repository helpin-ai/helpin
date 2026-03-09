import { useState } from 'react';
import { ChevronRight, MessageSquare, Pencil, Reply, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { CommentEditor } from '@/components/pm/CommentEditor';
import { MentionText } from '@/components/pm/MentionText';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { pmCommentService } from '@/lib/services/pmCommentService';
import type { CommentWithAuthor } from '@/lib/pmTypes';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';

function formatRelativeTime(dateStr: string): string {
  try {
    return formatDistanceToNow(parseISO(dateStr), { addSuffix: true });
  } catch {
    return dateStr;
  }
}

interface CommentThreadProps {
  workspaceId: string;
  entityType: 'story' | 'epic' | 'doc';
  entityId: string;
  comments: CommentWithAuthor[];
  currentUserId?: string;
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[];
  members?: AssignableMember[];
  onCommentsChange: (comments: CommentWithAuthor[]) => void;
}

export function CommentThread({
  workspaceId,
  entityType,
  entityId,
  comments,
  currentUserId,
  teams = [],
  members = [],
  onCommentsChange,
}: CommentThreadProps) {
  const [commentLoading, setCommentLoading] = useState(false);
  const [editingCommentId, setEditingCommentId] = useState<string | null>(null);
  const [editingCommentBody, setEditingCommentBody] = useState('');
  const [replyLoading, setReplyLoading] = useState(false);
  const [expandedThreads, setExpandedThreads] = useState<Set<string>>(new Set());

  const addComment = async (body: string) => {
    if (!body.trim()) return;
    setCommentLoading(true);
    const { data, error } = await pmCommentService.create(workspaceId, {
      entity_type: entityType,
      entity_id: entityId,
      body: body.trim(),
    });
    setCommentLoading(false);
    if (error || !data) return;
    onCommentsChange([...comments, data]);
  };

  const addReply = async (parentId: string, body: string) => {
    if (!body.trim()) return;
    setReplyLoading(true);
    const { data, error } = await pmCommentService.create(workspaceId, {
      entity_type: entityType,
      entity_id: entityId,
      body: body.trim(),
      parent_id: parentId,
    });
    setReplyLoading(false);
    if (error || !data) return;
    onCommentsChange(
      comments.map((c) =>
        c.comment.id === parentId
          ? { ...c, replies: [...(c.replies ?? []), data], reply_count: (c.reply_count || 0) + 1 }
          : c,
      ),
    );
    setExpandedThreads((prev) => new Set(prev).add(parentId));
  };

  const startEditComment = (entry: CommentWithAuthor) => {
    setEditingCommentId(entry.comment.id);
    setEditingCommentBody(entry.comment.body);
  };

  const cancelEditComment = () => {
    setEditingCommentId(null);
    setEditingCommentBody('');
  };

  const saveEditComment = async () => {
    if (!editingCommentId || !editingCommentBody.trim()) return;
    const { error } = await pmCommentService.update(workspaceId, editingCommentId, {
      body: editingCommentBody.trim(),
    });
    if (error) return;
    const updateBody = (list: CommentWithAuthor[]) =>
      list.map((c) =>
        c.comment.id === editingCommentId
          ? { ...c, comment: { ...c.comment, body: editingCommentBody.trim() } }
          : c,
      );
    onCommentsChange(
      updateBody(comments).map((c) => ({
        ...c,
        replies: c.replies ? updateBody(c.replies) : c.replies,
      })),
    );
    setEditingCommentId(null);
    setEditingCommentBody('');
  };

  const deleteComment = async (id: string) => {
    const { error } = await pmCommentService.remove(workspaceId, id);
    if (error) return;
    onCommentsChange(
      comments
        .filter((c) => c.comment.id !== id)
        .map((c) => ({
          ...c,
          replies: c.replies?.filter((r) => r.comment.id !== id),
          reply_count: (c.replies?.filter((r) => r.comment.id !== id) ?? []).length,
        })),
    );
  };

  const toggleThread = (commentId: string) => {
    setExpandedThreads((prev) => {
      const next = new Set(prev);
      if (next.has(commentId)) next.delete(commentId);
      else next.add(commentId);
      return next;
    });
  };

  const renderEditForm = (indent: string) => (
    <div className={`mt-1.5 ${indent}`}>
      <textarea
        value={editingCommentBody}
        rows={2}
        className="w-full resize-none rounded-md border border-border/60 bg-transparent px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
        onChange={(e) => setEditingCommentBody(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
            e.preventDefault();
            saveEditComment();
          }
          if (e.key === 'Escape') cancelEditComment();
        }}
      />
      <div className="mt-1 flex items-center gap-1.5">
        <Button variant="default" size="sm" className="h-6 px-2 text-xs" onClick={saveEditComment}>
          Save
        </Button>
        <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={cancelEditComment}>
          Cancel
        </Button>
      </div>
    </div>
  );

  return (
    <div className="rounded-lg border border-border/60">
      {comments.map((entry, idx) => {
        const isOwn = currentUserId === entry.comment.author_id;
        const isEditing = editingCommentId === entry.comment.id;
        const hasReplies = (entry.reply_count ?? 0) > 0;
        const isExpanded = expandedThreads.has(entry.comment.id);
        return (
          <div key={entry.comment.id}>
            {idx > 0 && <Separator />}
            <div className="group px-4 py-3">
              <div className="flex items-center gap-2">
                <UserAvatar
                  name={entry.author.full_name || entry.author.email}
                  avatarUrl={entry.author.avatar_url}
                  className="h-6 w-6 text-[9px]"
                />
                <span className="text-xs font-semibold">{entry.author.full_name || entry.author.email}</span>
                <span className="text-[11px] text-muted-foreground">{formatRelativeTime(entry.comment.created_at)}</span>
                <div className="ml-auto flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    type="button"
                    className="h-6 w-6 flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer"
                    title="Reply"
                    onClick={() => toggleThread(entry.comment.id)}
                  >
                    <Reply className="h-3 w-3" />
                  </button>
                  {isOwn && !isEditing && (
                    <>
                      <button
                        type="button"
                        className="h-6 w-6 flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer"
                        onClick={() => startEditComment(entry)}
                      >
                        <Pencil className="h-3 w-3" />
                      </button>
                      <button
                        type="button"
                        className="h-6 w-6 flex items-center justify-center rounded text-muted-foreground hover:text-destructive hover:bg-accent transition-colors cursor-pointer"
                        onClick={() => deleteComment(entry.comment.id)}
                      >
                        <Trash2 className="h-3 w-3" />
                      </button>
                    </>
                  )}
                </div>
              </div>
              {isEditing ? (
                renderEditForm('pl-8')
              ) : (
                <p className="mt-1.5 pl-8 text-sm">
                  <MentionText text={entry.comment.body} members={members} />
                </p>
              )}

              {/* Thread toggle */}
              {hasReplies && (
                <button
                  type="button"
                  className="mt-2 ml-8 flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                  onClick={() => toggleThread(entry.comment.id)}
                >
                  <MessageSquare className="h-3 w-3" />
                  <span>
                    {entry.reply_count} {entry.reply_count === 1 ? 'reply' : 'replies'}
                  </span>
                  <ChevronRight className={`h-3 w-3 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                </button>
              )}

              {/* Nested replies */}
              {isExpanded && (
                <div className="mt-2 ml-8 border-l-2 border-border/40 pl-4 space-y-3">
                  {entry.replies?.map((reply) => {
                    const isReplyOwn = currentUserId === reply.comment.author_id;
                    const isReplyEditing = editingCommentId === reply.comment.id;
                    return (
                      <div key={reply.comment.id} className="group/reply">
                        <div className="flex items-center gap-2">
                          <UserAvatar
                            name={reply.author.full_name || reply.author.email}
                            avatarUrl={reply.author.avatar_url}
                            className="h-5 w-5 text-[8px]"
                          />
                          <span className="text-xs font-semibold">{reply.author.full_name || reply.author.email}</span>
                          <span className="text-[11px] text-muted-foreground">{formatRelativeTime(reply.comment.created_at)}</span>
                          {isReplyOwn && !isReplyEditing && (
                            <div className="ml-auto flex items-center gap-0.5 opacity-0 group-hover/reply:opacity-100 transition-opacity">
                              <button
                                type="button"
                                className="h-5 w-5 flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer"
                                onClick={() => startEditComment(reply)}
                              >
                                <Pencil className="h-2.5 w-2.5" />
                              </button>
                              <button
                                type="button"
                                className="h-5 w-5 flex items-center justify-center rounded text-muted-foreground hover:text-destructive hover:bg-accent transition-colors cursor-pointer"
                                onClick={() => deleteComment(reply.comment.id)}
                              >
                                <Trash2 className="h-2.5 w-2.5" />
                              </button>
                            </div>
                          )}
                        </div>
                        {isReplyEditing ? (
                          renderEditForm('pl-7')
                        ) : (
                          <p className="mt-1 pl-7 text-sm">
                            <MentionText text={reply.comment.body} members={members} />
                          </p>
                        )}
                      </div>
                    );
                  })}
                  {/* Inline reply editor at bottom of thread */}
                  <div className="pt-1">
                    <CommentEditor
                      onSubmit={(body) => addReply(entry.comment.id, body)}
                      loading={replyLoading}
                      placeholder="Write a reply..."
                      teams={teams}
                      members={members}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
        );
      })}

      {/* Comment input */}
      {comments.length > 0 && <Separator />}
      <div className="px-4 py-3">
        <CommentEditor
          onSubmit={addComment}
          loading={commentLoading}
          placeholder="Leave a comment... (type @ to mention)"
          teams={teams}
          members={members}
        />
      </div>
    </div>
  );
}
