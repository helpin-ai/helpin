import { useCallback, useRef, useState } from 'react';
import { ChevronRight, Download, MessageSquare, Pencil, Reply, SmilePlus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { CommentEditor } from '@/components/pm/CommentEditor';
import { CommentBody } from '@/components/pm/CommentBody';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadToS3 } from '@/lib/api';
import type { CommentWithAuthor, ReactionSummary, AttachmentResponse } from '@/lib/pmTypes';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import type { EditorUploadConfig } from '@/hooks/useEditorImageUpload';

// File type icons
import pdfIcon from '@/assets/attachment/pdf-icon.png';
import csvIcon from '@/assets/attachment/csv-icon.png';
import excelIcon from '@/assets/attachment/excel-icon.png';
import docIcon from '@/assets/attachment/doc-icon.png';
import videoIcon from '@/assets/attachment/video-icon.png';
import audioIcon from '@/assets/attachment/audio-icon.png';
import zipIcon from '@/assets/attachment/zip-icon.png';
import defaultIcon from '@/assets/attachment/default-icon.png';

function formatRelativeTime(dateStr: string): string {
  try {
    return formatDistanceToNow(parseISO(dateStr), { addSuffix: true });
  } catch {
    return dateStr;
  }
}

// ── Curated emoji set ──
const REACTION_EMOJIS = [
  { emoji: '👍', label: 'Thumbs up' },
  { emoji: '❤️', label: 'Heart' },
  { emoji: '🎉', label: 'Celebrate' },
  { emoji: '😄', label: 'Smile' },
  { emoji: '👀', label: 'Eyes' },
  { emoji: '🚀', label: 'Rocket' },
];

// ── File helpers ──
function getFileExtension(filename: string): string {
  const parts = filename.split('.');
  return parts.length > 1 ? parts.pop()!.toLowerCase() : '';
}

function getFileTypeIcon(ext: string): string {
  const map: Record<string, string> = {
    pdf: pdfIcon, csv: csvIcon, xlsx: excelIcon, xls: excelIcon,
    doc: docIcon, docx: docIcon, mp4: videoIcon, mkv: videoIcon,
    webm: videoIcon, mp3: audioIcon, wav: audioIcon,
    zip: zipIcon, gz: zipIcon, tar: zipIcon, rar: zipIcon,
  };
  return map[ext] || defaultIcon;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// ── Reaction picker (shared between popover & inline) ──
function ReactionPicker({ onPick }: { onPick: (emoji: string) => void }) {
  return (
    <div className="flex items-center gap-0.5">
      {REACTION_EMOJIS.map(({ emoji, label }) => (
        <QuickTooltip key={emoji} label={label}>
          <button
            type="button"
            className="flex h-8 w-8 items-center justify-center rounded-md text-base transition-colors hover:bg-accent cursor-pointer"
            onClick={() => onPick(emoji)}
          >
            {emoji}
          </button>
        </QuickTooltip>
      ))}
    </div>
  );
}

// ── Reaction display ──
function CommentReactions({
  reactions,
  currentUserId,
  memberNameMap,
  onToggle,
}: {
  reactions: ReactionSummary[];
  currentUserId?: string;
  memberNameMap: Map<string, string>;
  onToggle: (emoji: string) => void;
}) {
  const [pickerOpen, setPickerOpen] = useState(false);

  return (
    <div className="flex items-center gap-1 flex-wrap mt-1.5">
      {reactions.map((r) => {
        const isOwn = currentUserId ? r.user_ids.includes(currentUserId) : false;
        const names = r.user_ids
          .map((id) => memberNameMap.get(id) ?? id)
          .join(', ');
        return (
          <QuickTooltip key={r.emoji} label={names}>
            <button
              type="button"
              onClick={() => onToggle(r.emoji)}
              className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs transition-colors cursor-pointer ${
                isOwn
                  ? 'border-primary/40 bg-primary/10 text-foreground'
                  : 'border-border/60 bg-muted/30 text-muted-foreground hover:bg-muted/50'
              }`}
            >
              <span>{r.emoji}</span>
              <span className="font-medium">{r.count}</span>
            </button>
          </QuickTooltip>
        );
      })}
      <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="inline-flex h-6 w-6 items-center justify-center rounded-full text-muted-foreground/50 transition-colors hover:bg-muted/50 hover:text-muted-foreground cursor-pointer"
          >
            <SmilePlus className="h-3.5 w-3.5" />
          </button>
        </PopoverTrigger>
        <PopoverContent side="top" align="start" className="w-auto p-1.5">
          <ReactionPicker onPick={(emoji) => { onToggle(emoji); setPickerOpen(false); }} />
        </PopoverContent>
      </Popover>
    </div>
  );
}

// ── Comment attachment display (compact inline) ──
function CommentAttachments({ attachments }: { attachments: AttachmentResponse[] }) {
  if (!attachments || attachments.length === 0) return null;

  const resolveUrl = (a: AttachmentResponse) => a.public_url || a.url;

  return (
    <div className="mt-2 flex flex-wrap gap-1.5">
      {attachments.map((entry) => {
        const ext = getFileExtension(entry.attachment.file_name);
        const isImage = entry.attachment.content_type.startsWith('image/') && !entry.attachment.content_type.includes('svg');
        const url = resolveUrl(entry);

        if (isImage) {
          return (
            <a
              key={entry.attachment.id}
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="block overflow-hidden rounded-md border border-border/60 transition-colors hover:border-border"
            >
              <img
                src={url}
                alt={entry.attachment.file_name}
                className="h-20 w-auto max-w-[160px] object-cover"
                loading="lazy"
              />
            </a>
          );
        }

        return (
          <a
            key={entry.attachment.id}
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 rounded-md border border-border/60 bg-muted/20 px-2.5 py-1.5 text-xs transition-colors hover:bg-muted/40"
          >
            <img src={getFileTypeIcon(ext)} alt={ext} className="h-5 w-5 shrink-0" />
            <div className="min-w-0">
              <p className="truncate font-medium text-foreground max-w-[140px]">{entry.attachment.file_name}</p>
              <p className="text-muted-foreground">{formatFileSize(entry.attachment.file_size)}</p>
            </div>
            <Download className="h-3 w-3 shrink-0 text-muted-foreground" />
          </a>
        );
      })}
    </div>
  );
}

// ── Uploaded file chip (shown in editor area while composing) ──
function UploadedFileChips({ files, onRemove }: {
  files: { id: string; name: string }[];
  onRemove: (id: string) => void;
}) {
  if (files.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1.5">
      {files.map((f) => {
        const ext = getFileExtension(f.name);
        return (
          <div
            key={f.id}
            className="flex items-center gap-1.5 rounded-md border border-border/60 bg-muted/20 pl-2 pr-1 py-1 text-xs"
          >
            <img src={getFileTypeIcon(ext)} alt={ext} className="h-4 w-4 shrink-0" />
            <span className="truncate max-w-[120px] text-muted-foreground">{f.name}</span>
            <button
              type="button"
              onClick={() => onRemove(f.id)}
              className="flex h-4 w-4 items-center justify-center rounded text-muted-foreground hover:text-foreground cursor-pointer"
            >
              <Trash2 className="h-3 w-3" />
            </button>
          </div>
        );
      })}
    </div>
  );
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
  uploadConfig?: EditorUploadConfig;
}

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10 MB

export function CommentThread({
  workspaceId,
  entityType,
  entityId,
  comments,
  currentUserId,
  teams = [],
  members = [],
  onCommentsChange,
  uploadConfig,
}: CommentThreadProps) {
  const [commentLoading, setCommentLoading] = useState(false);
  const [editingCommentId, setEditingCommentId] = useState<string | null>(null);
  const [editingCommentBody, setEditingCommentBody] = useState('');
  const [replyLoading, setReplyLoading] = useState(false);
  const [expandedThreads, setExpandedThreads] = useState<Set<string>>(new Set());

  // Uploaded attachment IDs for new comment and per-reply
  const [pendingAttachments, setPendingAttachments] = useState<{ id: string; name: string }[]>([]);
  const [replyPendingAttachments, setReplyPendingAttachments] = useState<Map<string, { id: string; name: string }[]>>(new Map());

  // Member name map for reaction tooltips
  const memberNameMap = useRef(new Map<string, string>());
  memberNameMap.current.clear();
  for (const m of members) {
    memberNameMap.current.set(m.user_id || m.id, m.display_name);
  }

  // Upload a file immediately and return the attachment ID
  const uploadFileImmediately = useCallback(async (file: File): Promise<{ id: string; name: string } | null> => {
    if (file.size > MAX_FILE_SIZE) return null;

    const { data: initData, error: initError } = await pmAttachmentService.initiateUpload(workspaceId, {
      entity_type: 'editor_upload',
      entity_id: workspaceId, // temporary — will be reassigned on comment creation
      file_name: file.name,
      file_size: file.size,
      content_type: file.type || 'application/octet-stream',
    });
    if (initError || !initData) return null;

    const uploadResult = await uploadToS3(initData.url, file, () => {}, { 'x-amz-acl': 'public-read' });
    if (!uploadResult.ok) return null;

    await pmAttachmentService.confirmUpload(workspaceId, initData.attachment.id);
    return { id: initData.attachment.id, name: file.name };
  }, [workspaceId]);

  const handleFileUpload = useCallback(async (parentId?: string) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = true;
    input.onchange = async () => {
      const files = input.files;
      if (!files?.length) return;
      for (const file of Array.from(files)) {
        const result = await uploadFileImmediately(file);
        if (!result) continue;
        if (parentId) {
          setReplyPendingAttachments((prev) => {
            const next = new Map(prev);
            next.set(parentId, [...(prev.get(parentId) ?? []), result]);
            return next;
          });
        } else {
          setPendingAttachments((prev) => [...prev, result]);
        }
      }
    };
    input.click();
  }, [uploadFileImmediately]);

  const removePendingAttachment = useCallback(async (attachmentId: string, parentId?: string) => {
    // Delete from S3/DB
    await pmAttachmentService.remove(workspaceId, attachmentId);
    if (parentId) {
      setReplyPendingAttachments((prev) => {
        const next = new Map(prev);
        const files = (prev.get(parentId) ?? []).filter((f) => f.id !== attachmentId);
        if (files.length === 0) next.delete(parentId);
        else next.set(parentId, files);
        return next;
      });
    } else {
      setPendingAttachments((prev) => prev.filter((f) => f.id !== attachmentId));
    }
  }, [workspaceId]);

  const addComment = async (body: string) => {
    if (!body.trim() && pendingAttachments.length === 0) return;
    setCommentLoading(true);

    const attachmentIds = pendingAttachments.map((a) => a.id);
    const { data, error } = await pmCommentService.create(workspaceId, {
      entity_type: entityType,
      entity_id: entityId,
      body: body.trim() || '(attachment)',
      attachment_ids: attachmentIds.length > 0 ? attachmentIds : undefined,
    });
    if (error || !data) {
      setCommentLoading(false);
      return;
    }
    setPendingAttachments([]);

    // Reload comments to get attachments in response
    if (attachmentIds.length > 0) {
      const { data: refreshed } = await pmCommentService.list(workspaceId, entityType, entityId);
      if (refreshed) {
        onCommentsChange(refreshed);
        setCommentLoading(false);
        return;
      }
    }

    onCommentsChange([...comments, data]);
    setCommentLoading(false);
  };

  const addReply = async (parentId: string, body: string) => {
    const replyFiles = replyPendingAttachments.get(parentId) ?? [];
    if (!body.trim() && replyFiles.length === 0) return;
    setReplyLoading(true);

    const attachmentIds = replyFiles.map((a) => a.id);
    const { data, error } = await pmCommentService.create(workspaceId, {
      entity_type: entityType,
      entity_id: entityId,
      body: body.trim() || '(attachment)',
      parent_id: parentId,
      attachment_ids: attachmentIds.length > 0 ? attachmentIds : undefined,
    });
    if (error || !data) {
      setReplyLoading(false);
      return;
    }
    setReplyPendingAttachments((prev) => {
      const next = new Map(prev);
      next.delete(parentId);
      return next;
    });

    // Reload if attachments were included
    if (attachmentIds.length > 0) {
      const { data: refreshed } = await pmCommentService.list(workspaceId, entityType, entityId);
      if (refreshed) {
        onCommentsChange(refreshed);
        setReplyLoading(false);
        setExpandedThreads((prev) => new Set(prev).add(parentId));
        return;
      }
    }

    onCommentsChange(
      comments.map((c) =>
        c.comment.id === parentId
          ? { ...c, replies: [...(c.replies ?? []), data], reply_count: (c.reply_count || 0) + 1 }
          : c,
      ),
    );
    setExpandedThreads((prev) => new Set(prev).add(parentId));
    setReplyLoading(false);
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

  const toggleReaction = async (commentId: string, emoji: string) => {
    const { data, error } = await pmCommentService.toggleReaction(workspaceId, commentId, emoji);
    if (error || !data) return;

    const updateReactions = (list: CommentWithAuthor[]): CommentWithAuthor[] =>
      list.map((c) => {
        if (c.comment.id === commentId) {
          return { ...c, reactions: data };
        }
        if (c.replies) {
          return { ...c, replies: updateReactions(c.replies) };
        }
        return c;
      });
    onCommentsChange(updateReactions(comments));
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

  const renderComment = (
    entry: CommentWithAuthor,
    isReply: boolean,
  ) => {
    const isOwn = currentUserId === entry.comment.author_id;
    const isEditing = editingCommentId === entry.comment.id;
    const hasReactions = (entry.reactions?.length ?? 0) > 0;
    const indent = isReply ? 'pl-7' : 'pl-8';
    const avatarSize = isReply ? 'h-5 w-5 text-[8px]' : 'h-6 w-6 text-[9px]';
    const groupClass = isReply ? 'group/reply' : 'group';
    const hoverClass = isReply ? 'group-hover/reply:opacity-100' : 'group-hover:opacity-100';
    const btnSize = isReply ? 'h-5 w-5' : 'h-6 w-6';
    const iconSize = isReply ? 'h-2.5 w-2.5' : 'h-3 w-3';

    return (
      <div className={groupClass}>
        <div className="flex items-center gap-2">
          <UserAvatar
            name={entry.author.full_name || entry.author.email}
            avatarUrl={entry.author.avatar_url}
            className={avatarSize}
          />
          <span className="text-xs font-semibold">{entry.author.full_name || entry.author.email}</span>
          <span className="text-[11px] text-muted-foreground">{formatRelativeTime(entry.comment.created_at)}</span>
          <div className={`ml-auto flex items-center gap-0.5 opacity-0 ${hoverClass} transition-opacity`}>
            <Popover>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className={`${btnSize} flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer`}
                  title="React"
                >
                  <SmilePlus className={iconSize} />
                </button>
              </PopoverTrigger>
              <PopoverContent side="top" align="end" className="w-auto p-1.5">
                <ReactionPicker onPick={(emoji) => toggleReaction(entry.comment.id, emoji)} />
              </PopoverContent>
            </Popover>
            {!isReply && (
              <button
                type="button"
                className={`${btnSize} flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer`}
                title="Reply"
                onClick={() => toggleThread(entry.comment.id)}
              >
                <Reply className={iconSize} />
              </button>
            )}
            {isOwn && !isEditing && (
              <>
                <button
                  type="button"
                  className={`${btnSize} flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer`}
                  onClick={() => startEditComment(entry)}
                >
                  <Pencil className={iconSize} />
                </button>
                <button
                  type="button"
                  className={`${btnSize} flex items-center justify-center rounded text-muted-foreground hover:text-destructive hover:bg-accent transition-colors cursor-pointer`}
                  onClick={() => deleteComment(entry.comment.id)}
                >
                  <Trash2 className={iconSize} />
                </button>
              </>
            )}
          </div>
        </div>
        {isEditing ? (
          renderEditForm(indent)
        ) : (
          <CommentBody body={entry.comment.body} members={members} className={`mt-1.5 ${indent}`} />
        )}
        {/* Attachments */}
        {entry.attachments && entry.attachments.length > 0 && (
          <div className={indent}>
            <CommentAttachments attachments={entry.attachments} />
          </div>
        )}
        {/* Reactions */}
        {hasReactions && (
          <div className={indent}>
            <CommentReactions
              reactions={entry.reactions ?? []}
              currentUserId={currentUserId}
              memberNameMap={memberNameMap.current}
              onToggle={(emoji) => toggleReaction(entry.comment.id, emoji)}
            />
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="rounded-lg border border-border/60">
      {comments.map((entry, idx) => {
        const hasReplies = (entry.reply_count ?? 0) > 0;
        const isExpanded = expandedThreads.has(entry.comment.id);
        return (
          <div key={entry.comment.id}>
            {idx > 0 && <Separator />}
            <div className="group px-4 py-3">
              {renderComment(entry, false)}

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
                  {entry.replies?.map((reply) => (
                    <div key={reply.comment.id}>
                      {renderComment(reply, true)}
                    </div>
                  ))}
                  {/* Inline reply editor */}
                  <div className="pt-1">
                    <CommentEditor
                      onSubmit={(body) => addReply(entry.comment.id, body)}
                      loading={replyLoading}
                      placeholder="Write a reply..."
                      teams={teams}
                      members={members}
                      uploadConfig={uploadConfig}
                      onFileSelect={() => handleFileUpload(entry.comment.id)}
                      uploadedFiles={replyPendingAttachments.get(entry.comment.id) ?? []}
                      onRemoveUploadedFile={(id) => removePendingAttachment(id, entry.comment.id)}
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
          uploadConfig={uploadConfig}
          onFileSelect={() => handleFileUpload()}
          uploadedFiles={pendingAttachments}
          onRemoveUploadedFile={(id) => removePendingAttachment(id)}
        />
      </div>
    </div>
  );
}
