import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ArrowReloadHorizontalIcon, ArrowRight01Icon, Message01Icon, PencilEdit01Icon, ArrowTurnBackwardIcon, SmilePlusIcon, Delete01Icon, Cancel01Icon, PlayCircleIcon, CheckmarkCircle02Icon, MoreHorizontalIcon } from '@/lib/icons';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { CommentEditor } from '@/components/pm/CommentEditor';
import { CommentBody } from '@/components/pm/CommentBody';
import { ImageLightbox } from '@/components/pm/ImageLightbox';
import { extractInlineAttachmentIds } from '@/components/pm/editorImageAttachments';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { formatDistanceToNowStrict, parseISO } from 'date-fns';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadToS3 } from '@/lib/api';
import type { Comment, CommentWithAuthor, ReactionSummary, AttachmentResponse } from '@/lib/pmTypes';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';

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
    // Tight format suitable for narrow rails: "11m", "2h", "3d", "1mo"
    const raw = formatDistanceToNowStrict(parseISO(dateStr), { addSuffix: false });
    return raw
      .replace(/ seconds?/, 's')
      .replace(/ minutes?/, 'm')
      .replace(/ hours?/, 'h')
      .replace(/ days?/, 'd')
      .replace(/ weeks?/, 'w')
      .replace(/ months?/, 'mo')
      .replace(/ years?/, 'y');
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

function isVideoAttachment(contentType: string, fileName: string): boolean {
  if (contentType.startsWith('video/')) return true;
  return ['mp4', 'mov', 'webm', 'mkv', 'wmv', 'avi', 'mpeg', 'mpg'].includes(getFileExtension(fileName));
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
            <SmilePlusIcon className="h-3.5 w-3.5" />
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
function CommentAttachments({
  attachments,
  body,
  editable = false,
  onDelete,
}: {
  attachments: AttachmentResponse[];
  body: string;
  editable?: boolean;
  onDelete?: (attachmentId: string) => void;
}) {
  const [lightbox, setLightbox] = useState<{ src: string; name: string; kind: 'image' | 'video' } | null>(null);

  if (!attachments || attachments.length === 0) return null;

  const inlineAttachmentIds = new Set(extractInlineAttachmentIds(body));
  const visibleAttachments = attachments.filter(({ attachment }) => {
    const isInlineImage =
      attachment.content_type.startsWith('image/') &&
      inlineAttachmentIds.has(attachment.id);
    return !isInlineImage;
  });
  if (visibleAttachments.length === 0) return null;

  const resolveUrl = (a: AttachmentResponse) => a.public_url || a.url;

  return (
    <>
      <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3">
        {visibleAttachments.map((entry) => {
          const ext = getFileExtension(entry.attachment.file_name);
          const isImage = entry.attachment.content_type.startsWith('image/') && !entry.attachment.content_type.includes('svg');
          const isVideo = isVideoAttachment(entry.attachment.content_type, entry.attachment.file_name);
          const url = resolveUrl(entry);

          const inner = isImage ? (
            <img
              src={url}
              alt={entry.attachment.file_name}
              className="h-20 w-full object-cover transition-transform group-hover:scale-105"
              loading="lazy"
            />
          ) : isVideo ? (
            <div className="relative h-20 w-full overflow-hidden bg-black">
              <video
                src={url}
                preload="metadata"
                muted
                className="h-20 w-full object-cover opacity-80"
              />
              <div className="absolute inset-0 flex items-center justify-center bg-black/20">
                <PlayCircleIcon className="h-8 w-8 text-white drop-shadow" />
              </div>
            </div>
          ) : (
            <div className="flex h-20 flex-col items-center justify-center gap-1.5 bg-muted/30">
              <img src={getFileTypeIcon(ext)} alt={ext} className="h-8 w-8" />
              <span className="text-[9px] font-medium uppercase text-muted-foreground tracking-wide">{ext || 'FILE'}</span>
            </div>
          );

          const label = (
            <p className="truncate px-1.5 py-1 text-[10px] text-muted-foreground" title={entry.attachment.file_name}>
              {entry.attachment.file_name}
            </p>
          );

          const tile = (isImage || isVideo) && url ? (
            <button
              type="button"
              onClick={() => setLightbox({ src: url, name: entry.attachment.file_name, kind: isVideo ? 'video' : 'image' })}
              className="group block w-full overflow-hidden rounded-lg border border-border/60 transition-colors hover:border-border cursor-pointer text-left"
            >
              {inner}
              {label}
            </button>
          ) : (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="group block overflow-hidden rounded-lg border border-border/60 transition-colors hover:border-border"
            >
              {inner}
              {label}
            </a>
          );

          if (!editable || !onDelete) {
            return <div key={entry.attachment.id}>{tile}</div>;
          }

          return (
            <div key={entry.attachment.id} className="relative group/tile">
              {tile}
              <button
                type="button"
                aria-label="Remove attachment"
                onClick={(e) => {
                  e.stopPropagation();
                  onDelete(entry.attachment.id);
                }}
                className="absolute top-1 right-1 inline-flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 group-hover/tile:opacity-100 hover:bg-black/80 transition-opacity cursor-pointer"
              >
                <Cancel01Icon className="h-3 w-3" />
              </button>
            </div>
          );
        })}
      </div>
      {lightbox && (
        <ImageLightbox src={lightbox.src} alt={lightbox.name} kind={lightbox.kind} onClose={() => setLightbox(null)} />
      )}
    </>
  );
}


interface CommentThreadProps {
  workspaceId: string;
  entityType: 'task' | 'epic' | 'doc';
  entityId: string;
  comments: CommentWithAuthor[];
  currentUserId?: string;
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[];
  members?: AssignableMember[];
  commentService?: typeof pmCommentService;
  attachmentsEnabled?: boolean;
  commentAnchor?: {
    block_id?: string;
    range?: Record<string, unknown>;
    anchor_text?: string;
  } | null;
  activeCommentId?: string | null;
  onCommentAnchorConsumed?: () => void;
  onCommentsChange: (comments: CommentWithAuthor[]) => void;
  /** Hide the bottom-of-list "Add a comment…" composer (used when this thread is rendered inside an inline side card). */
  hideTopLevelComposer?: boolean;
  /** Hide all empty-state copy and the empty card itself (used when many threads render side-by-side). */
  hideEmptyState?: boolean;
}

const MAX_FILE_SIZE = 50 * 1024 * 1024; // 50 MB

export function CommentThread({
  workspaceId,
  entityType,
  entityId,
  comments,
  currentUserId,
  teams = [],
  members = [],
  commentService = pmCommentService,
  attachmentsEnabled = true,
  commentAnchor = null,
  activeCommentId = null,
  onCommentAnchorConsumed,
  onCommentsChange,
  hideTopLevelComposer = false,
  hideEmptyState = false,
}: CommentThreadProps) {
  const [commentLoading, setCommentLoading] = useState(false);
  const [editingCommentId, setEditingCommentId] = useState<string | null>(null);
  const [editingCommentBody, setEditingCommentBody] = useState('');
  const [editSaving, setEditSaving] = useState(false);
  const [replyLoading, setReplyLoading] = useState(false);
  const [expandedThreads, setExpandedThreads] = useState<Set<string>>(new Set());

  type PendingFile = { id: string; name: string; url?: string };
  // Uploaded attachment IDs for new comment, edit, and per-reply
  const [pendingAttachments, setPendingAttachments] = useState<PendingFile[]>([]);
  const [editPendingAttachments, setEditPendingAttachments] = useState<PendingFile[]>([]);
  const [replyPendingAttachments, setReplyPendingAttachments] = useState<Map<string, PendingFile[]>>(new Map());
  const pendingAttachmentsRef = useRef(pendingAttachments);
  const editPendingAttachmentsRef = useRef(editPendingAttachments);
  const replyPendingAttachmentsRef = useRef(replyPendingAttachments);

  // Member name map for reaction tooltips
  const memberNameMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const m of members) {
      map.set(m.user_id || m.id, m.display_name);
    }
    return map;
  }, [members]);

  useEffect(() => {
    pendingAttachmentsRef.current = pendingAttachments;
  }, [pendingAttachments]);

  useEffect(() => {
    editPendingAttachmentsRef.current = editPendingAttachments;
  }, [editPendingAttachments]);

  useEffect(() => {
    replyPendingAttachmentsRef.current = replyPendingAttachments;
  }, [replyPendingAttachments]);

  // Upload a file immediately and return the attachment ID + public URL
  const uploadFileImmediately = useCallback(async (file: File): Promise<PendingFile | null> => {
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
    return { id: initData.attachment.id, name: file.name, url: initData.public_url || initData.url };
  }, [workspaceId]);

  useEffect(
    () => () => {
      const attachmentIds = [
        ...pendingAttachmentsRef.current.map((attachment) => attachment.id),
        ...editPendingAttachmentsRef.current.map((attachment) => attachment.id),
        ...Array.from(replyPendingAttachmentsRef.current.values()).flatMap((attachments) =>
          attachments.map((attachment) => attachment.id),
        ),
      ];
      if (attachmentIds.length === 0) {
        return;
      }
      void Promise.allSettled(
        attachmentIds.map((attachmentId) =>
          pmAttachmentService.remove(workspaceId, attachmentId, { pendingOnly: true }),
        ),
      );
    },
    [workspaceId],
  );

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

  const handleImageUpload = useCallback(async (files: File[], parentId?: string) => {
    for (const file of files) {
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
  }, [uploadFileImmediately]);

  const removePendingAttachment = useCallback(async (attachmentId: string, parentId?: string) => {
    // Delete from S3/DB
    await pmAttachmentService.remove(workspaceId, attachmentId, { pendingOnly: true });
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

  const handleEditImageUpload = useCallback(async (files: File[]) => {
    for (const file of files) {
      const result = await uploadFileImmediately(file);
      if (!result) continue;
      setEditPendingAttachments((prev) => [...prev, result]);
    }
  }, [uploadFileImmediately]);

  const handleEditFileUpload = useCallback(async () => {
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = true;
    input.onchange = async () => {
      const files = input.files;
      if (!files?.length) return;
      for (const file of Array.from(files)) {
        const result = await uploadFileImmediately(file);
        if (!result) continue;
        setEditPendingAttachments((prev) => [...prev, result]);
      }
    };
    input.click();
  }, [uploadFileImmediately]);

  const removeEditPendingAttachment = useCallback(async (attachmentId: string) => {
    await pmAttachmentService.remove(workspaceId, attachmentId, { pendingOnly: true });
    setEditPendingAttachments((prev) => prev.filter((f) => f.id !== attachmentId));
  }, [workspaceId]);

  const deleteExistingAttachment = useCallback(async (commentId: string, attachmentId: string) => {
    const { error } = await pmAttachmentService.remove(workspaceId, attachmentId);
    if (error) return;
    const stripAttachment = (list: CommentWithAuthor[]): CommentWithAuthor[] =>
      list.map((c) =>
        c.comment.id === commentId
          ? { ...c, attachments: (c.attachments ?? []).filter((a) => a.attachment.id !== attachmentId) }
          : c,
      );
    onCommentsChange(
      stripAttachment(comments).map((c) => ({
        ...c,
        replies: c.replies ? stripAttachment(c.replies) : c.replies,
      })),
    );
  }, [workspaceId, comments, onCommentsChange]);

  const addComment = async (body: string) => {
    if (!body.trim() && pendingAttachments.length === 0) return;
    setCommentLoading(true);

    const attachmentIds = pendingAttachments.map((a) => a.id);
    const { data, error } = await commentService.create(workspaceId, {
      entity_type: entityType,
      entity_id: entityId,
      body: body.trim() || '(attachment)',
      block_id: commentAnchor?.block_id,
      range: commentAnchor?.range,
      anchor_text: commentAnchor?.anchor_text,
      attachment_ids: attachmentIds.length > 0 ? attachmentIds : undefined,
    });
    if (error || !data) {
      setCommentLoading(false);
      return;
    }
    setPendingAttachments([]);
    onCommentAnchorConsumed?.();

    // Reload comments to get attachments in response
    if (attachmentIds.length > 0) {
      const { data: refreshed } = await commentService.list(workspaceId, entityType, entityId);
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
    const { data, error } = await commentService.create(workspaceId, {
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
      const { data: refreshed } = await commentService.list(workspaceId, entityType, entityId);
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
    setEditPendingAttachments([]);
  };

  const cancelEditComment = useCallback(async () => {
    const orphanIds = editPendingAttachments.map((a) => a.id);
    setEditingCommentId(null);
    setEditingCommentBody('');
    setEditPendingAttachments([]);
    if (orphanIds.length > 0) {
      await Promise.allSettled(
        orphanIds.map((id) => pmAttachmentService.remove(workspaceId, id, { pendingOnly: true })),
      );
    }
  }, [editPendingAttachments, workspaceId]);

  const saveEditComment = async (bodyOverride?: string) => {
    const body = (bodyOverride ?? editingCommentBody).trim();
    if (!editingCommentId) return;
    if (!body && editPendingAttachments.length === 0) return;
    setEditSaving(true);
    const attachmentIds = editPendingAttachments.map((a) => a.id);
    const { error } = await commentService.update(workspaceId, editingCommentId, {
      body: body || '(attachment)',
      attachment_ids: attachmentIds.length > 0 ? attachmentIds : undefined,
    });
    if (error) {
      setEditSaving(false);
      return;
    }

    // Always reload so attachment tiles (existing + newly linked) stay in sync.
    const { data: refreshed } = await commentService.list(workspaceId, entityType, entityId);
    if (refreshed) onCommentsChange(refreshed);

    setEditingCommentId(null);
    setEditingCommentBody('');
    setEditPendingAttachments([]);
    setEditSaving(false);
  };

  const deleteComment = async (id: string) => {
    const attachmentIds = comments
      .flatMap((comment) => [comment, ...(comment.replies ?? [])])
      .find((comment) => comment.comment.id === id)
      ? extractInlineAttachmentIds(
          comments
            .flatMap((comment) => [comment, ...(comment.replies ?? [])])
            .find((comment) => comment.comment.id === id)?.comment.body ?? '',
        )
      : [];
    const { error } = await commentService.remove(workspaceId, id);
    if (error) return;
    if (attachmentIds.length > 0) {
      await Promise.allSettled(
        attachmentIds.map((attachmentId) => pmAttachmentService.remove(workspaceId, attachmentId)),
      );
    }
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
    const { data, error } = await commentService.toggleReaction(workspaceId, commentId, emoji);
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

  const setCommentResolved = async (commentId: string, resolved: boolean) => {
    const action = resolved ? commentService.resolve : commentService.reopen;
    const { data, error } = await action(workspaceId, commentId);
    if (error || !data) return;

    const updatedComment = data as Comment;
    const updateComment = (list: CommentWithAuthor[]): CommentWithAuthor[] =>
      list.map((c) => {
        if (c.comment.id === commentId) {
          return { ...c, comment: updatedComment };
        }
        if (c.replies) {
          return { ...c, replies: updateComment(c.replies) };
        }
        return c;
      });
    onCommentsChange(updateComment(comments));
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
      <div className="rounded-lg border border-border/60">
        <CommentEditor
          key={editingCommentId ?? 'edit'}
          onSubmit={(html) => { void saveEditComment(html); }}
          loading={editSaving}
          placeholder="Edit comment..."
          teams={teams}
          members={members}
          onImageSelect={attachmentsEnabled ? (files) => void handleEditImageUpload(files) : undefined}
          onFileSelect={attachmentsEnabled ? () => void handleEditFileUpload() : undefined}
          uploadedFiles={editPendingAttachments}
          onRemoveUploadedFile={(id) => void removeEditPendingAttachment(id)}
          initialContent={editingCommentBody}
          onCancel={() => void cancelEditComment()}
          autoFocus
        />
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
    const avatarSize = isReply ? 'h-6 w-6 text-[9px]' : 'h-7 w-7 text-[10px]';
    const groupClass = isReply ? 'group/reply' : 'group';
    const hoverClass = isReply ? 'group-hover/reply:opacity-100' : 'group-hover:opacity-100';
    const isResolved = Boolean(entry.comment.resolved_at);
    const authorName = entry.author.full_name || entry.author.email;

    return (
      <div className={`${groupClass} ${isResolved ? 'opacity-75' : ''} flex gap-2`}>
        <UserAvatar
          name={authorName}
          avatarUrl={entry.author.avatar_url}
          avatarStyle={entry.author.avatar_style}
          avatarSeed={entry.author.avatar_seed}
          avatarBackgroundMode={entry.author.avatar_background_mode}
          avatarBackgroundColor={entry.author.avatar_background_color}
          className={`${avatarSize} shrink-0 mt-0.5`}
        />
        <div className="min-w-0 flex-1">
          {/* Header row: name, time, hover actions */}
          <div className="flex items-center gap-1.5">
            <span className="truncate text-xs font-semibold">{authorName}</span>
            <span className="shrink-0 text-[11px] text-muted-foreground">{formatRelativeTime(entry.comment.created_at)}</span>
            {isResolved && (
              <span className="ml-1 inline-flex items-center gap-1 rounded-full bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">
                <CheckmarkCircle02Icon className="h-3 w-3" />
                Resolved
              </span>
            )}
            {!isEditing && (
              <div className={`ml-auto flex items-center gap-0.5 opacity-0 transition-opacity ${hoverClass}`}>
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                      aria-label="React"
                    >
                      <SmilePlusIcon className="h-3.5 w-3.5" />
                    </button>
                  </PopoverTrigger>
                  <PopoverContent side="top" align="end" className="w-auto p-1">
                    <ReactionPicker onPick={(emoji) => toggleReaction(entry.comment.id, emoji)} />
                  </PopoverContent>
                </Popover>
                {!isReply && (
                  <QuickTooltip label="Reply">
                    <button
                      type="button"
                      className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                      onClick={() => toggleThread(entry.comment.id)}
                      aria-label="Reply"
                    >
                      <ArrowTurnBackwardIcon className="h-3.5 w-3.5" />
                    </button>
                  </QuickTooltip>
                )}
                {!isReply && (
                  <QuickTooltip label={isResolved ? 'Reopen' : 'Resolve'}>
                    <button
                      type="button"
                      className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                      onClick={() => void setCommentResolved(entry.comment.id, !isResolved)}
                      aria-label={isResolved ? 'Reopen' : 'Resolve'}
                    >
                      {isResolved ? (
                        <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />
                      ) : (
                        <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                      )}
                    </button>
                  </QuickTooltip>
                )}
                {isOwn && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <button
                        type="button"
                        className="flex h-6 w-6 items-center justify-center rounded text-foreground/60 hover:bg-accent hover:text-foreground"
                        aria-label="More"
                      >
                        <MoreHorizontalIcon className="h-3.5 w-3.5" />
                      </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-36">
                      <DropdownMenuItem onSelect={() => startEditComment(entry)}>
                        <PencilEdit01Icon className="h-3.5 w-3.5" />
                        Edit
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        onSelect={() => deleteComment(entry.comment.id)}
                        className="text-destructive focus:text-destructive"
                      >
                        <Delete01Icon className="h-3.5 w-3.5" />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
            )}
          </div>

          {isEditing ? (
            renderEditForm('')
          ) : (
            <>
              {/* Anchor: slim italic blockquote line */}
              {(entry.comment.anchor_text || entry.comment.block_id) && (
                <div className="mt-1 truncate border-l-2 border-border/60 pl-2 py-0.5 text-[11px] italic text-muted-foreground">
                  {entry.comment.anchor_text
                    ? `“${entry.comment.anchor_text}”`
                    : `Block ${entry.comment.block_id}`}
                </div>
              )}
              <CommentBody
                body={entry.comment.body}
                members={members}
                teams={teams}
                className="mt-1"
              />
            </>
          )}

          {/* Attachments */}
          {entry.attachments && entry.attachments.length > 0 && (
            <div className="mt-1.5">
              <CommentAttachments
                attachments={entry.attachments}
                body={entry.comment.body}
                editable={isEditing}
                onDelete={(attachmentId) => void deleteExistingAttachment(entry.comment.id, attachmentId)}
              />
            </div>
          )}

          {/* Reactions */}
          {hasReactions && (
            <div className="mt-1.5">
              <CommentReactions
                reactions={entry.reactions ?? []}
                currentUserId={currentUserId}
                memberNameMap={memberNameMap}
                onToggle={(emoji) => toggleReaction(entry.comment.id, emoji)}
              />
            </div>
          )}

        </div>
      </div>
    );
  };

  return (
    <div className="space-y-3">
      {/* Empty state */}
      {comments.length === 0 && !hideEmptyState && (
        <div className="rounded-lg border border-dashed border-border/60 px-4 py-8 text-center">
          <Message01Icon className="mx-auto mb-2 h-6 w-6 text-muted-foreground/40" />
          <p className="text-sm font-medium text-foreground">No comments yet</p>
          <p className="mt-1 text-[11px] text-muted-foreground">
            Highlight text in the document to comment on a specific passage.
          </p>
        </div>
      )}

      {/* Thread list */}
      {comments.length > 0 && (
        <div className="divide-y divide-border/50">
          {comments.map((entry) => {
            const hasReplies = (entry.reply_count ?? 0) > 0;
            const isExpanded = expandedThreads.has(entry.comment.id);
            const isActive = activeCommentId === entry.comment.id;
            return (
              <div
                key={entry.comment.id}
                data-comment-thread-id={entry.comment.id}
                className={`rounded-lg px-3 py-3 transition-colors ${
                  isActive ? 'bg-amber-500/10 ring-1 ring-inset ring-amber-500/20' : ''
                }`}
              >
                {renderComment(entry, false)}

                {/* Thread toggle */}
                {hasReplies && (
                  <button
                    type="button"
                    className="mt-2 ml-9 flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                    onClick={() => toggleThread(entry.comment.id)}
                  >
                    <Message01Icon className="h-3 w-3" />
                    <span>
                      {entry.reply_count} {entry.reply_count === 1 ? 'reply' : 'replies'}
                    </span>
                    <ArrowRight01Icon className={`h-3 w-3 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                  </button>
                )}

                {/* Nested replies */}
                {isExpanded && (
                  <div className="mt-3 ml-4 border-l border-border/40 pl-3 space-y-3">
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
                        placeholder="Reply…"
                        variant="reply"
                        teams={teams}
                        members={members}
                        onImageSelect={attachmentsEnabled ? (files) => handleImageUpload(files, entry.comment.id) : undefined}
                        onFileSelect={attachmentsEnabled ? () => handleFileUpload(entry.comment.id) : undefined}
                        uploadedFiles={replyPendingAttachments.get(entry.comment.id) ?? []}
                        onRemoveUploadedFile={(id) => removePendingAttachment(id, entry.comment.id)}
                      />
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Top-level composer (creates a new doc/entity-scoped comment) */}
      {!hideTopLevelComposer && (
      <div>
        {commentAnchor && (
          <div className="mb-1.5 flex items-start gap-2 rounded-md border border-border/60 bg-muted/30 px-2 py-1.5 text-[11px] text-muted-foreground">
            <span className="flex-1 truncate">
              Commenting on {commentAnchor.anchor_text ? `“${commentAnchor.anchor_text}”` : 'selected block'}
            </span>
            <button
              type="button"
              className="shrink-0 rounded p-0.5 hover:bg-accent hover:text-foreground"
              onClick={() => onCommentAnchorConsumed?.()}
              aria-label="Clear anchor"
            >
              <Cancel01Icon className="h-3 w-3" />
            </button>
          </div>
        )}
        <CommentEditor
          onSubmit={addComment}
          loading={commentLoading}
          placeholder="Add a comment…"
          variant="primary"
          teams={teams}
          members={members}
          onImageSelect={attachmentsEnabled ? (files) => handleImageUpload(files) : undefined}
          onFileSelect={attachmentsEnabled ? () => handleFileUpload() : undefined}
          uploadedFiles={pendingAttachments}
          onRemoveUploadedFile={(id) => removePendingAttachment(id)}
        />
      </div>
      )}
    </div>
  );
}
