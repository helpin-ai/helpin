import { memo, useCallback, useEffect, useMemo, useState, type ComponentPropsWithoutRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { TickDouble01Icon, CheckmarkCircle02Icon, ArrowDown01Icon, Download04Icon, LinkSquare01Icon, File01Icon, AttachmentIcon, RotateLeft01Icon, StickyNote01Icon, Cancel01Icon, CancelCircleIcon, Mail01Icon, AlertCircleIcon, BotIcon, UserIcon } from '@/lib/icons';
import { EmailDetailModal } from './EmailDetailModal';
import { MessageActionsContextMenu, MessageActionsMenu } from './MessageActionsMenu';
import { MessageDeleteDialog } from './MessageDeleteDialog';
import { MessageInfoDialog } from './MessageInfoDialog';
import { useShortcutComposerStore } from './shortcutDialogStore';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useDeleteSupportMessage } from '@/hooks/queries/useSupport';
import { useAuthStore } from '@/stores/authStore';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import type { AIMessageMetadata, SupportForwardedAttribution, SupportLinkPreview, SupportMessage, TicketSource } from '@/lib/pmTypes';
import { EmailBodyRenderer } from './EmailBodyRenderer';
import { formatMessageTime, formatTimestamp, getInitial, getAvatarColor, getEffectiveSenderType, HELPIN_AI_DISPLAY_NAME, parseAIMessageMetadata, parseSupportLinkPreviews } from './helpers';
import { timeAgo } from '@/lib/utils';
import { toast } from 'sonner';

const MARKDOWN_REMARK_PLUGINS = [remarkGfm];
const RESTORE_SUPPORT_DRAFT_EVENT = 'support:restore-draft';

export function sanitizeSupportShortcutSeed(content: string): string {
  return content.replace(/\\(\r?\n)/g, '$1');
}

/** Splits text on @mention patterns and wraps them in highlight spans. */
function renderMentionHighlights(content: string): ReactNode[] | null {
  const regex = /@([a-zA-Z0-9][a-zA-Z0-9._-]*)/g;
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let key = 0;
  while ((match = regex.exec(content)) !== null) {
    if (match.index > lastIndex) {
      parts.push(content.slice(lastIndex, match.index));
    }
    parts.push(
      <span key={key++} className="mention-highlight">{match[0]}</span>
    );
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < content.length) {
    parts.push(content.slice(lastIndex));
  }
  return parts.length > 1 ? parts : null;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function containsMarkdownTable(content: string): boolean {
  return /\|(?:[^\n|]+\|){1,}[^\n]*\n\|(?:\s*[-:]+\s*\|){1,}/m.test(content) || /<table[\s>]/i.test(content);
}

const markdownComponents = {
  a: ({ href, children }: ComponentPropsWithoutRef<'a'>) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="break-all [overflow-wrap:anywhere]"
    >
      {children}
    </a>
  ),
  table: ({ children }: ComponentPropsWithoutRef<'table'>) => (
    <div className="chat-markdown-table-wrap">
      <table>{children}</table>
    </div>
  ),
  img: ({ className, loading, ...props }: ComponentPropsWithoutRef<'img'>) => (
    <img
      {...props}
      loading={loading ?? 'lazy'}
      className={`max-h-60 max-w-full rounded-lg object-cover ${className ?? ''}`.trim()}
    />
  ),
};

const SOURCE_LABELS: Record<string, string> = {
  widget: 'Chat',
  email: 'Email',
  internal: 'Internal',
  api: 'API',
};

function previewHostLabel(preview: SupportLinkPreview): string {
  try {
    return new URL(preview.url).hostname.replace(/^www\./, '') || preview.host;
  } catch {
    return preview.host.replace(/^www\./, '');
  }
}

function parseForwardedAttributionMetadata(metadata?: string): SupportForwardedAttribution | null {
  if (!metadata) return null;
  try {
    const parsed = JSON.parse(metadata) as Record<string, unknown>;
    const originalEmail = typeof parsed.original_sender_email === 'string' ? parsed.original_sender_email.trim() : '';
    const forwardedByEmail = typeof parsed.forwarded_by_email === 'string' ? parsed.forwarded_by_email.trim() : '';
    if (!originalEmail || !forwardedByEmail) return null;
    return {
      original_sender_email: originalEmail,
      original_sender_name: typeof parsed.original_sender_name === 'string' ? parsed.original_sender_name.trim() : undefined,
      forwarded_by_email: forwardedByEmail,
      forwarded_by_name: typeof parsed.forwarded_by_name === 'string' ? parsed.forwarded_by_name.trim() : undefined,
      confidence: typeof parsed.sender_attribution_confidence === 'number' ? parsed.sender_attribution_confidence : 0,
      confidence_level: typeof parsed.sender_attribution_confidence_level === 'string' ? parsed.sender_attribution_confidence_level : '',
      source: typeof parsed.sender_attribution_source === 'string' ? parsed.sender_attribution_source : 'forwarded_body',
    };
  } catch {
    return null;
  }
}

function formatCountdown(ms: number): string {
  const totalSeconds = Math.max(0, Math.ceil(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

function LinkPreviewCard({ preview }: { preview: SupportLinkPreview }) {
  // Both incoming (`bg-muted`) and outgoing (`bg-blue-50`) bubbles are light,
  // so foreground/muted-foreground tokens read well on either. We dropped the
  // separate isOutgoing styling that assumed a dark/saturated outgoing bubble.
  return (
    <a
      href={preview.url}
      target="_blank"
      rel="noopener noreferrer"
      className="block overflow-hidden rounded-xl border border-border bg-background text-foreground transition-colors hover:opacity-95"
    >
      {preview.image_url ? (
        <img
          src={preview.image_url}
          alt={preview.title}
          className="h-36 w-full object-cover"
          loading="lazy"
        />
      ) : null}
      <div className="space-y-1.5 p-3">
        <div className="flex items-center gap-1.5 text-[11px] uppercase tracking-wide text-muted-foreground">
          <span className="truncate">{preview.site_name || previewHostLabel(preview)}</span>
          <LinkSquare01Icon className="h-3 w-3 shrink-0" />
        </div>
        <div className="text-sm font-semibold leading-snug">{preview.title}</div>
        {preview.description ? (
          <p className="text-xs leading-relaxed text-muted-foreground">
            {preview.description}
          </p>
        ) : null}
      </div>
    </a>
  );
}

function findTrailingAIContractStart(content: string): number {
  const trimmed = content.trimEnd();
  if (!trimmed.endsWith('}')) return -1;

  let depth = 0;
  let inString = false;
  let escaped = false;

  for (let i = trimmed.length - 1; i >= 0; i -= 1) {
    const ch = trimmed[i];

    if (escaped) {
      escaped = false;
      continue;
    }
    if (ch === '\\' && inString) {
      escaped = true;
      continue;
    }
    if (ch === '"') {
      inString = !inString;
      continue;
    }
    if (inString) continue;

    if (ch === '}') depth += 1;
    if (ch === '{') {
      depth -= 1;
      if (depth === 0) return i;
    }
  }

  return -1;
}

interface MessageBubbleProps {
  message: SupportMessage;
  isConsecutive?: boolean;
  isLastInGroup?: boolean;
  source?: TicketSource;
  receiptStatus?: 'delivered' | 'sent_email' | 'delivered_email' | 'read' | 'read_email' | null;
  fallbackAvatarUrl?: string;
  customerDisplayName?: string;
}

export const MessageBubble = memo(function MessageBubble({
  message,
  isConsecutive,
  isLastInGroup = true,
  source,
  receiptStatus,
  fallbackAvatarUrl,
  customerDisplayName,
}: MessageBubbleProps) {
  const currentUser = useAuthStore((s) => s.user);
  const aiMeta = useMemo<AIMessageMetadata | null>(() => parseAIMessageMetadata(message.metadata), [message.metadata]);
  const linkPreviews = useMemo<SupportLinkPreview[]>(() => parseSupportLinkPreviews(message.metadata), [message.metadata]);
  const forwardedAttribution = useMemo(() => parseForwardedAttributionMetadata(message.metadata), [message.metadata]);
  const effectiveSenderType = getEffectiveSenderType(message);
  const isCustomer = effectiveSenderType === 'customer';
  const isAI = effectiveSenderType === 'ai';
  const isAgent = effectiveSenderType === 'agent';
  const isInternal = message.is_internal;
  const senderName = message.sender_display_name
    ?? (isCustomer ? (customerDisplayName || 'Customer') : isAI ? HELPIN_AI_DISPLAY_NAME : isAgent ? 'Agent' : currentUser?.full_name ?? 'You');
  const resolvedSenderName = isAI ? HELPIN_AI_DISPLAY_NAME : senderName;
  const showAvatar = isLastInGroup;
  const fullTimestamp = formatTimestamp(message.created_at);
  const sourceLabel = source ? SOURCE_LABELS[source] ?? source : null;

  // Strip trailing AI contract JSON blocks that LLM sometimes appends to content.
  // Only strip if the JSON parses as an AI contract (has can_answer + content keys)
  // so legitimate user code blocks with ```json are preserved.
  const displayContent = useMemo(() => {
    const jsonBlockIdx = message.content.indexOf('```json');
    if (jsonBlockIdx > 0) {
      const afterFence = message.content.slice(jsonBlockIdx + 7);
      const closeIdx = afterFence.indexOf('```');
      if (closeIdx > 0) {
        try {
          const parsed = JSON.parse(afterFence.slice(0, closeIdx).trim());
          if (parsed && typeof parsed.can_answer === 'boolean' && typeof parsed.content === 'string') {
            return message.content.slice(0, jsonBlockIdx).trimEnd();
          }
        } catch { /* not an AI contract — keep as-is */ }
      }
    }

    const rawJsonStart = findTrailingAIContractStart(message.content);
    if (rawJsonStart > 0) {
      try {
        const parsed = JSON.parse(message.content.slice(rawJsonStart).trim());
        if (parsed && typeof parsed.can_answer === 'boolean' && typeof parsed.content === 'string') {
          return message.content.slice(0, rawJsonStart).trimEnd();
        }
      } catch { /* not an AI contract — keep as-is */ }
    }

    return message.content;
  }, [message.content]);
  const hasTableContent = useMemo(() => containsMarkdownTable(displayContent), [displayContent]);

  // Highlight @mentions in internal notes
  const mentionParts = useMemo(() => {
    if (!isInternal) return null;
    return renderMentionHighlights(displayContent);
  }, [displayContent, isInternal]);

  const [sourcesOpen, setSourcesOpen] = useState(false);
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null);
  const [emailDetailOpen, setEmailDetailOpen] = useState(false);
  const [infoOpen, setInfoOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const deleteMutation = useDeleteSupportMessage(message.workspace_id, message.conversation_id);

  useEffect(() => {
    if (!message.cancellable_until) return;
    const interval = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(interval);
  }, [message.cancellable_until]);

  const imageAttachments = message.attachments?.filter(a => a.file_type.startsWith('image/')) ?? [];
  const fileAttachments = message.attachments?.filter(a => !a.file_type.startsWith('image/')) ?? [];
  const hasDisplayContent = displayContent.trim().length > 0;
  const showBubble = !!displayContent || fileAttachments.length > 0 || linkPreviews.length > 0;
  const hasEmailBody = message.via_channel === 'email' && !!message.html_body;

  const verb = isCustomer ? 'Received' : 'Sent';
  const relativeTime = timeAgo(message.created_at);
  const isRelativeFormat = relativeTime.endsWith(' ago') || relativeTime === 'Just now';
  const timeLine = isRelativeFormat
    ? `${verb}, ${relativeTime} (${formatMessageTime(message.created_at)})`
    : `${verb}, ${relativeTime}`;
  const tooltipContent = (
    <div className="space-y-0.5 text-center text-xs">
      <div>{timeLine}</div>
      {sourceLabel && <div className="text-background/70">via {sourceLabel}</div>}
    </div>
  );

  const cancellableUntilMs = message.cancellable_until ? Date.parse(message.cancellable_until) : 0;
  const canMutateOwnReply = message.sender_type === 'user'
    && message.sender_user_id === currentUser?.id
    && message.message_type !== 'system'
    && !message.is_internal;
  const cancellableActive = canMutateOwnReply && Number.isFinite(cancellableUntilMs) && cancellableUntilMs > nowMs;
  const hasCancellableFooter = canMutateOwnReply && !!message.cancellable_until;
  const countdown = cancellableActive ? formatCountdown(cancellableUntilMs - nowMs) : '0:00';

  const restoreComposerDraft = useCallback((markdown: string) => {
    window.dispatchEvent(new CustomEvent(RESTORE_SUPPORT_DRAFT_EVENT, {
      detail: { conversationId: message.conversation_id, markdown, attachments: message.attachments ?? [] },
    }));
  }, [message.attachments, message.conversation_id]);

  const handleUndoOrEdit = useCallback(async () => {
    const result = await deleteMutation.mutateAsync({ messageId: message.id, undo: true });
    if (result.markdown) {
      restoreComposerDraft(result.markdown);
    }
  }, [deleteMutation, message.id, restoreComposerDraft]);

  const handleDelete = useCallback(async () => {
    const result = await deleteMutation.mutateAsync({ messageId: message.id, undo: false });
    setDeleteDialogOpen(false);
    if (result.email_already_sent) {
      toast.message('Message removed from chat', { description: 'The email may already have been delivered.' });
    }
  }, [deleteMutation, message.id]);

  const handleCopy = useCallback(() => {
    void navigator.clipboard?.writeText(displayContent);
    toast.success('Message copied');
  }, [displayContent]);

  const canSaveAsShortcut = displayContent.trim().length > 0 && message.message_type !== 'system';
  const openShortcutComposer = useShortcutComposerStore((s) => s.openCreate);
  const handleSaveAsShortcut = useCallback(
    () => openShortcutComposer({ seedContent: sanitizeSupportShortcutSeed(displayContent) }),
    [openShortcutComposer, displayContent],
  );

  const handleQuoteReply = useCallback(() => {
    const quoted = displayContent
      .split('\n')
      .map((line) => `> ${line}`)
      .join('\n');
    restoreComposerDraft(`${quoted}\n\n`);
  }, [displayContent, restoreComposerDraft]);

  const renderFileAttachments = (tone: 'default' | 'note' = 'default', className = '') => {
    if (fileAttachments.length === 0) return null;

    const linkClassName = tone === 'note'
      ? 'flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-100/40 px-3 py-2 text-xs text-amber-900 transition-colors hover:bg-amber-100 dark:border-amber-800/70 dark:bg-amber-950/30 dark:text-amber-100 dark:hover:bg-amber-900/30'
      : 'flex items-center gap-2 rounded-lg border border-border px-3 py-2 text-xs text-foreground transition-colors hover:bg-muted/50';

    return (
      <div className={`${className} space-y-1.5`.trim()}>
        {fileAttachments.map((att) => (
          <a
            key={att.id}
            href={att.url}
            target="_blank"
            rel="noopener noreferrer"
            className={linkClassName}
          >
            <AttachmentIcon className="h-3.5 w-3.5 shrink-0 opacity-60" />
            <span className="truncate font-medium">{att.file_name}</span>
            <span className="shrink-0 opacity-60">{formatFileSize(att.file_size)}</span>
            <Download04Icon className="ml-auto h-3.5 w-3.5 shrink-0 opacity-60" />
          </a>
        ))}
      </div>
    );
  };

  const renderImageAttachments = (className = '') => {
    if (imageAttachments.length === 0) return null;

    return (
      <div className={`${className} space-y-1.5`.trim()}>
        {imageAttachments.map((att) => (
          <button
            key={att.id}
            type="button"
            onClick={() => setLightboxSrc(att.url)}
            className="block cursor-zoom-in overflow-hidden rounded-xl transition-opacity hover:opacity-90"
          >
            <img
              src={att.url}
              alt={att.file_name}
              className="max-h-60 max-w-full rounded-xl object-cover"
              loading="lazy"
            />
          </button>
        ))}
      </div>
    );
  };

  const lightboxPortal = lightboxSrc ? createPortal(
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/85 backdrop-blur-sm animate-in fade-in duration-150"
      onClick={() => setLightboxSrc(null)}
    >
      <button
        onClick={() => setLightboxSrc(null)}
        className="absolute right-4 top-4 flex h-9 w-9 items-center justify-center rounded-full bg-white/15 text-white transition-colors hover:bg-white/25"
      >
        <Cancel01Icon className="h-5 w-5" />
      </button>
      <img
        src={lightboxSrc}
        alt="Preview"
        className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      />
    </div>,
    document.body,
  ) : null;

  const resolvedAvatarUrl = message.sender_avatar_url
    ?? fallbackAvatarUrl
    ?? ((message.sender_user_id && message.sender_user_id === currentUser?.id)
      ? resolveTeamMemberAvatarSrc({
          avatarUrl: currentUser.avatar_url,
          avatarStyle: currentUser.avatar_style,
          avatarSeed: currentUser.avatar_seed,
          avatarBackgroundMode: currentUser.avatar_background_mode,
          avatarBackgroundColor: currentUser.avatar_background_color,
          fallbackSeed: currentUser.full_name ?? currentUser.email,
        })
      : undefined);
  const avatarSeed = message.sender_user_id || message.sender_agent_id || resolvedSenderName;
  const fallbackAvatar = (
    <div
      className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[10.5px] font-semibold leading-none shadow-sm ${getAvatarColor(avatarSeed)}`}
    >
      {getInitial(resolvedSenderName)}
    </div>
  );

  // ── System message: centered pill with avatar (Intercom-style) ──
  if (message.message_type === 'system') {
    // Dispatch on system_event_type set by the backend. The legacy
    // content-keyword branch below is a TRANSITIONAL fallback for
    // pre-migration rows only — tracked in
    // docs/plans/2026-04-15-system-message-event-type-plan.md, slated for
    // removal after the backfill has covered historic rows in prod.
    const routingEventTypes: ReadonlyArray<string> = [
      'teammate_joined',
      'assigned',
      'unassigned',
      'took',
      'agent_assigned',
      'mailbox_moved',
      'triage_routed',
      'triage_dismissed',
      'ai_escalated',
      'customer_requested_human',
    ];

    const stateEventTypes: ReadonlyArray<string> = ['resolved', 'reopened', 'closed'];
    const eventType = message.system_event_type;

    const ESCALATION_LABELS: Record<string, string> = {
      ai_escalated: 'AI escalated to a human',
      customer_requested_human: 'Customer requested a human',
    };
    const isEscalationEvent = !!eventType && eventType in ESCALATION_LABELS;
    const escalationLabel = isEscalationEvent ? ESCALATION_LABELS[eventType] : null;
    const escalationIcon = eventType === 'customer_requested_human'
      ? <UserIcon className="h-3 w-3" />
      : eventType === 'ai_escalated'
        ? <BotIcon className="h-3 w-3" />
        : null;

    let isRoutingEvent: boolean;
    let stateEventKind: 'resolved' | 'reopened' | 'closed' | null;
    if (eventType) {
      isRoutingEvent = routingEventTypes.includes(eventType);
      stateEventKind = stateEventTypes.includes(eventType)
        ? (eventType as 'resolved' | 'reopened' | 'closed')
        : null;
    } else {
      // TODO: remove after system_event_type backfill rollout completes —
      // plan 2026-04-15.
      const lower = message.content.toLowerCase();
      const resolved = lower.includes('resolved');
      const reopened = lower.includes('reopened');
      const closed = lower.includes('closed');
      isRoutingEvent =
        lower.includes('joined') ||
        lower.includes('left') ||
        lower.includes('assigned') ||
        lower.includes('unassigned') ||
        lower.includes('took this conversation');
      stateEventKind = resolved ? 'resolved' : reopened ? 'reopened' : closed ? 'closed' : null;
    }

    const statusIcon = stateEventKind === 'resolved' ? <CheckmarkCircle02Icon className="h-4 w-4 shrink-0" />
      : stateEventKind === 'reopened' ? <RotateLeft01Icon className="h-3.5 w-3.5 shrink-0" />
      : stateEventKind === 'closed' ? <CancelCircleIcon className="h-4 w-4 shrink-0" />
      : null;

    // Routing events use a neutral muted style with leading avatar; state
    // transitions keep the stronger slate pill so they stay visually distinct.
    if (isRoutingEvent) {
      const escalationPillClass = 'rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200';
      const defaultPillClass = 'rounded-full px-3 py-1 text-xs text-muted-foreground';
      return (
        <div className="my-5 flex items-center justify-center gap-2 animate-in fade-in duration-300">
          <Tooltip>
            <TooltipTrigger asChild>
              <div className={`flex items-center gap-2 ${isEscalationEvent ? escalationPillClass : defaultPillClass}`}>
                {isEscalationEvent ? (
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                    {escalationIcon}
                  </span>
                ) : resolvedAvatarUrl ? (
                  <img src={resolvedAvatarUrl} alt={resolvedSenderName} className="h-5 w-5 rounded-full object-cover" />
                ) : (
                  <div className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[9px] font-semibold leading-none ${getAvatarColor(avatarSeed)}`}>
                    {getInitial(resolvedSenderName)}
                  </div>
                )}
                <span>{escalationLabel ?? message.content}</span>
              </div>
            </TooltipTrigger>
            <TooltipContent side="top">
              <div className="text-xs">{fullTimestamp}</div>
            </TooltipContent>
          </Tooltip>
        </div>
      );
    }

    return (
      <div className="my-5 flex items-center justify-center gap-2 animate-in fade-in duration-300">
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="flex items-center gap-2.5 rounded-full bg-slate-700 px-4 py-2 text-white shadow-sm" style={{ border: 'none' }}>
              {statusIcon ?? <CheckmarkCircle02Icon className="h-4 w-4 shrink-0" />}
              <span className="text-sm font-medium">{message.content}</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="top">
            <div className="space-y-0.5 text-xs">
              <div className="font-medium">{resolvedSenderName}</div>
              <div className="text-background/70">{fullTimestamp}</div>
            </div>
          </TooltipContent>
        </Tooltip>
      </div>
    );
  }

  // ── Internal note: right-aligned card with amber accent ──
  if (isInternal) {
    return (
      <>
        <div className={`flex justify-end ${isConsecutive ? 'mt-1' : 'mt-5'}`}>
          <div className="max-w-[85%]">
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="rounded-lg border-r-[3px] border-r-amber-400 bg-amber-50 px-4 py-2.5 [overflow-wrap:anywhere] dark:bg-amber-950/20">
                  <div className="mb-1.5 flex items-center gap-1.5">
                    <StickyNote01Icon className="h-3 w-3 text-amber-500 dark:text-amber-400" />
                    <span className="text-[11px] text-amber-600 dark:text-amber-400">
                      <span className="font-semibold">{resolvedSenderName}</span>
                      <span className="font-normal"> left a private note</span>
                    </span>
                  </div>
                  {hasDisplayContent && (
                    <div className="prose-chat text-sm leading-relaxed text-amber-900 dark:text-amber-200">
                      {mentionParts ? (
                        <p className="whitespace-pre-wrap">{mentionParts}</p>
                      ) : (
                        <Markdown remarkPlugins={MARKDOWN_REMARK_PLUGINS} components={markdownComponents}>{displayContent}</Markdown>
                      )}
                    </div>
                  )}
                  {renderFileAttachments('note', hasDisplayContent ? 'mt-2' : 'mt-1.5')}
                  {renderImageAttachments(hasDisplayContent || fileAttachments.length > 0 ? 'mt-2' : 'mt-1.5')}
                </div>
              </TooltipTrigger>
              <TooltipContent side="top">{tooltipContent}</TooltipContent>
            </Tooltip>
          </div>
        </div>
        {lightboxPortal}
      </>
    );
  }

  // ── Chat bubble ──
  const avatarEl = isCustomer ? (
    <Tooltip>
      <TooltipTrigger asChild>
        {fallbackAvatar}
      </TooltipTrigger>
      <TooltipContent side="left"><span className="text-xs font-medium">{resolvedSenderName}</span></TooltipContent>
    </Tooltip>
  ) : resolvedAvatarUrl ? (
    <Tooltip>
      <TooltipTrigger asChild>
        <img src={resolvedAvatarUrl} alt={resolvedSenderName} className="h-7 w-7 shrink-0 rounded-full object-cover shadow-sm" />
      </TooltipTrigger>
      <TooltipContent side="right"><span className="text-xs font-medium">{resolvedSenderName}</span></TooltipContent>
    </Tooltip>
  ) : (
    <Tooltip>
      <TooltipTrigger asChild>
        {fallbackAvatar}
      </TooltipTrigger>
      <TooltipContent side="right"><span className="text-xs font-medium">{resolvedSenderName}</span></TooltipContent>
    </Tooltip>
  );

  const hasEmailBadge = message.via_channel === 'email';
  const hasStatusBelow = !!receiptStatus || !!aiMeta || hasEmailBadge;
  const bubbleWidthClass = hasEmailBody
    ? 'min-w-0 w-[min(92%,64rem)] max-w-[calc(100%-2.25rem)]'
    : hasTableContent
      ? 'min-w-0 max-w-[min(85%,46rem)] lg:max-w-[min(85%,48rem)]'
      : 'min-w-0 max-w-[min(85%,42rem)]';

  return (
    <div className={`${isConsecutive ? 'mt-1' : 'mt-5'} ${!isConsecutive ? (isCustomer ? 'animate-in fade-in slide-in-from-left-2 duration-200' : 'animate-in fade-in slide-in-from-right-2 duration-200') : ''}`}>
      {/* Bubble row: avatar + bubble aligned together */}
      <div className={`flex ${isCustomer ? 'justify-start' : 'justify-end'}`}>
        {/* Left side: avatar or spacer (customer messages) */}
        {isCustomer && (
          <div className="mr-2 flex w-7 shrink-0 flex-col justify-end">
            {showAvatar && avatarEl}
          </div>
        )}

        <MessageActionsContextMenu
          canEdit={cancellableActive}
          canDelete={canMutateOwnReply}
          onEdit={handleUndoOrEdit}
          onCopy={handleCopy}
          onReply={handleQuoteReply}
          onDelete={() => setDeleteDialogOpen(true)}
          onInfo={() => setInfoOpen(true)}
          onSaveAsShortcut={canSaveAsShortcut ? handleSaveAsShortcut : undefined}
        >
        <div
          data-slot="support-message-bubble"
          className={`${bubbleWidthClass} group/message relative`}
        >
          <MessageActionsMenu
            alignSide={isCustomer ? 'right' : 'left'}
            canEdit={cancellableActive}
            canDelete={canMutateOwnReply}
            onEdit={handleUndoOrEdit}
            onCopy={handleCopy}
            onReply={handleQuoteReply}
            onDelete={() => setDeleteDialogOpen(true)}
            onInfo={() => setInfoOpen(true)}
            onSaveAsShortcut={canSaveAsShortcut ? handleSaveAsShortcut : undefined}
          />
          {showBubble && (
            <Tooltip>
              <TooltipTrigger asChild>
                <div
                  className={`rounded-2xl border border-border/40 px-3.5 py-2 text-sm leading-relaxed [overflow-wrap:anywhere] ${
                    isCustomer
                      ? `bg-muted text-foreground/85 dark:text-foreground ${isLastInGroup ? 'rounded-bl-sm' : ''}`
                      : `bg-blue-50 text-foreground/85 dark:bg-blue-950/40 dark:text-foreground ${isLastInGroup ? 'rounded-br-sm' : ''}`
                  } ${hasTableContent || hasEmailBody ? 'overflow-hidden' : ''}`}
                >
                  {hasEmailBody ? (
                    <div className="-mx-1" data-chat-tone={isCustomer ? 'customer' : 'agent'}>
                      <EmailBodyRenderer html={message.html_body ?? ''} />
                    </div>
                  ) : (
                    displayContent && (
                      <div
                        className="prose-chat"
                        data-chat-tone={isCustomer ? 'customer' : 'agent'}
                        data-has-table={hasTableContent ? 'true' : 'false'}
                      >
                        <Markdown remarkPlugins={MARKDOWN_REMARK_PLUGINS} components={markdownComponents}>{displayContent}</Markdown>
                      </div>
                    )
                  )}
                  {fileAttachments.length > 0 && (
                    renderFileAttachments('default', displayContent ? 'mt-2' : '')
                  )}
                  {linkPreviews.length > 0 && (
                    <div className={`${displayContent || fileAttachments.length > 0 ? 'mt-2' : ''} space-y-2`}>
                      {linkPreviews.map((preview) => (
                        <LinkPreviewCard
                          key={`${message.id}:${preview.url}`}
                          preview={preview}
                        />
                      ))}
                    </div>
                  )}
                </div>
              </TooltipTrigger>
              <TooltipContent side="top">
                {tooltipContent}
              </TooltipContent>
            </Tooltip>
          )}

          {/* Image attachments: outside the bubble, clickable for preview */}
          {imageAttachments.length > 0 && (
            renderImageAttachments(showBubble ? 'mt-1.5' : '')
          )}
        </div>
        </MessageActionsContextMenu>

        {/* Right side: avatar or spacer (agent/user messages) */}
        {!isCustomer && (
          <div className="ml-2 flex w-7 shrink-0 flex-col justify-end">
            {showAvatar && avatarEl}
          </div>
        )}
      </div>

      {/* Email detail modal — rendered via Radix portal */}
      {hasEmailBadge && (
        <EmailDetailModal
          workspaceId={message.workspace_id}
          message={message}
          open={emailDetailOpen}
          onOpenChange={setEmailDetailOpen}
        />
      )}
      <MessageInfoDialog
        workspaceId={message.workspace_id}
        conversationId={message.conversation_id}
        messageId={message.id}
        open={infoOpen}
        onOpenChange={setInfoOpen}
      />
      <MessageDeleteDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        onConfirm={handleDelete}
        isPending={deleteMutation.isPending}
      />

      {/* Lightbox modal — rendered in portal for full-screen overlay */}
      {lightboxPortal}

      {/* Status below the bubble row — outside the avatar alignment */}
      {(hasStatusBelow || hasCancellableFooter) && (
        <div className={`mt-0.5 ${isCustomer ? 'pl-9' : 'pr-9'}`}>
          {hasEmailBadge && (
            <div className={`mb-0.5 space-y-0.5 ${isCustomer ? '' : 'text-right'}`}>
              {forwardedAttribution && isCustomer && (
                <div className="text-[11px] text-muted-foreground">
                  Forwarded by {forwardedAttribution.forwarded_by_name || forwardedAttribution.forwarded_by_email}
                  <span className="opacity-70"> · originally from </span>
                  <span className="font-medium text-foreground/80">
                    {forwardedAttribution.original_sender_name || forwardedAttribution.original_sender_email}
                  </span>
                  {forwardedAttribution.original_sender_name ? (
                    <span className="opacity-70"> &lt;{forwardedAttribution.original_sender_email}&gt;</span>
                  ) : null}
                </div>
              )}
              <div className={`flex ${isCustomer ? '' : 'justify-end'}`}>
                <button
                  type="button"
                  onClick={() => setEmailDetailOpen(true)}
                  className="inline-flex items-center gap-1 text-[11px] text-muted-foreground transition-colors hover:text-foreground hover:underline"
                >
                  <Mail01Icon className="h-3 w-3" />
                  {isCustomer ? 'Received via email' : 'Sent via email'}
                  <span className="opacity-60">· View details</span>
                </button>
              </div>
            </div>
          )}

          {/* Delivery failure indicator — supersedes the read receipt when the outbound email bounced or was marked spam. */}
          {(message.email_delivery_status === 'bounced' || message.email_delivery_status === 'spam_complaint') ? (
            <div className={`flex items-center gap-1 ${isCustomer ? '' : 'justify-end'}`}>
              <AlertCircleIcon className="h-3.5 w-3.5 text-red-500" />
              <span className="text-[11px] text-red-600 dark:text-red-400">
                {message.email_delivery_status === 'spam_complaint' ? 'Marked as spam' : 'Delivery failed'}
                {message.email_delivery_error ? ` · ${message.email_delivery_error}` : ''}
              </span>
            </div>
          ) : hasCancellableFooter ? (
            <div className={`flex items-center gap-1 text-[11px] text-muted-foreground ${isCustomer ? '' : 'justify-end'}`}>
              <TickDouble01Icon className="h-3.5 w-3.5" />
              {cancellableActive ? (
                <>
                  <span>Sent</span>
                  <span>·</span>
                  <button
                    type="button"
                    className="font-medium text-foreground transition-colors hover:text-primary hover:underline"
                    onClick={handleUndoOrEdit}
                    disabled={deleteMutation.isPending}
                  >
                    Undo
                  </button>
                  <span>·</span>
                  <span>{countdown}</span>
                </>
              ) : (
                <span>Delivered to email</span>
              )}
            </div>
          ) : aiMeta ? (
            // AI message: combined footer — confidence + sources cluster + receipt.
            // For agent messages the parent wrapper isn't bubble-width, so
            // justify-between would scatter the chips across the whole row.
            // Cluster everything to the right under the bubble instead.
            <>
              <div
                className={`mt-1.5 flex items-center gap-2 ${
                  isCustomer ? 'justify-between' : 'justify-end'
                }`}
              >
                <div className="inline-flex items-center gap-1.5 text-[11px]">
                  <span className="inline-flex items-center gap-1 rounded-full border border-primary/20 bg-primary/10 px-2 py-0.5 font-medium text-primary">
                    <CheckmarkCircle02Icon className="h-3 w-3" />
                    {(aiMeta.ai_confidence * 100).toFixed(0)}% confident
                  </span>
                  {aiMeta.ai_sources?.length > 0 && (
                    <button
                      type="button"
                      onClick={() => setSourcesOpen(!sourcesOpen)}
                      aria-expanded={sourcesOpen}
                      className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground ${sourcesOpen ? 'border-border bg-background text-foreground' : 'border-border/60 bg-muted/40'}`}
                    >
                      <File01Icon className="h-3 w-3" />
                      {aiMeta.ai_sources.length} source{aiMeta.ai_sources.length > 1 ? 's' : ''}
                      <ArrowDown01Icon className={`h-3 w-3 transition-transform ${sourcesOpen ? 'rotate-180' : ''}`} />
                    </button>
                  )}
                </div>
                {receiptStatus && (
                  <span className="inline-flex shrink-0 items-center gap-1 text-[11px] text-muted-foreground">
                    {receiptStatus === 'read' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                        Read in chat
                      </>
                    ) : receiptStatus === 'read_email' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                        Read via email
                      </>
                    ) : receiptStatus === 'delivered_email' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5" />
                        Delivered via email
                      </>
                    ) : receiptStatus === 'sent_email' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5" />
                        Sent via email
                      </>
                    ) : (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5" />
                        Delivered
                      </>
                    )}
                  </span>
                )}
              </div>
              {sourcesOpen && aiMeta.ai_sources?.length > 0 && (
                <div className="mt-1.5 overflow-hidden rounded-xl border bg-muted/40 p-1 shadow-sm">
                  {aiMeta.ai_sources.map((src, idx) => {
                    const Tag: 'a' | 'div' = src.url ? 'a' : 'div';
                    const linkProps = src.url
                      ? { href: src.url, target: '_blank' as const, rel: 'noopener noreferrer' }
                      : {};
                    return (
                      <Tag
                        key={src.docId}
                        {...linkProps}
                        className={`group flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs ${idx > 0 ? 'border-t border-border/60' : ''} ${src.url ? 'cursor-pointer text-foreground hover:bg-background hover:text-primary' : 'text-foreground'}`}
                      >
                        <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                        <span className={`min-w-0 truncate font-medium ${src.url ? 'group-hover:underline' : ''}`}>{src.title}</span>
                        {src.url && (
                          <LinkSquare01Icon className="h-3 w-3 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" />
                        )}
                      </Tag>
                    );
                  })}
                </div>
              )}
            </>
          ) : receiptStatus && (
            <div className={`flex items-center gap-1 ${isCustomer ? '' : 'justify-end'}`}>
              {receiptStatus === 'read' ? (
                <>
                  <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                  <span className="text-[11px] text-muted-foreground">Read in chat</span>
                </>
              ) : receiptStatus === 'read_email' ? (
                <>
                  <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                  <span className="text-[11px] text-muted-foreground">Read via email</span>
                </>
              ) : receiptStatus === 'delivered_email' ? (
                <>
                  <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="text-[11px] text-muted-foreground">Delivered via email</span>
                </>
              ) : receiptStatus === 'sent_email' ? (
                <>
                  <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="text-[11px] text-muted-foreground">Sent via email</span>
                </>
              ) : (
                <>
                  <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="text-[11px] text-muted-foreground">Delivered</span>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
});
