import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { ContactAvatarImage } from '@/components/ui/contact-avatar-image';
import { SupportAIActivity } from './SupportAIActivity';
import { getSupportAIActivity } from './supportAIActivity';
import { PendingSendStatus } from './PendingSendStatus';
import { memo, useCallback, useMemo, useState, type ComponentPropsWithoutRef, type ReactNode } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { TickDouble01Icon, CheckmarkCircle02Icon, ArrowDown01Icon, LinkSquare01Icon, File01Icon, RotateLeft01Icon, StickyNote01Icon, CancelCircleIcon, Mail01Icon, AlertCircleIcon, UserIcon, ZapIcon, BubbleChatIcon } from '@/lib/icons';
import { useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { EmailDetailModal } from './EmailDetailModal';
import { MessageActionsContextMenu, MessageActionsMenu } from './MessageActionsMenu';
import { MessageDeleteDialog } from './MessageDeleteDialog';
import { MessageInfoDialog } from './MessageInfoDialog';
import { SupportAttachmentGallery } from './SupportAttachmentGallery';
import { useShortcutComposerStore } from './shortcutDialogStore';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useDeleteSupportMessage } from '@/hooks/queries/useSupport';
import { useAuthStore } from '@/stores/authStore';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import type { AIMessageMetadata, SupportForwardedAttribution, SupportLinkPreview, SupportLinkSecurity, SupportMessage, TicketSource } from '@/lib/pmTypes';
import { EmailBodyRenderer } from './EmailBodyRenderer';
import { findSupportLinkSecurity, formatMessageTime, formatTimestamp, getInitial, getAvatarColor, getEffectiveSenderType, getExplicitEmailDeliveryState, HELPIN_AI_DISPLAY_NAME, isExternalSupportEmailReply, parseAIMessageMetadata, parseSupportLinkPreviews, parseSupportLinkSecurity, type SupportReceiptStatus } from './helpers';
import { getReplyEmailSubject, getReplyDeliveryMode, REPLY_DELIVERY_LABELS } from './replyDelivery';
import { cleanForwardedDisplayContent, hasForwardedHeaderMarker } from './forwardedEmailDisplay';
import { timeAgo } from '@/lib/utils';
import { toast } from 'sonner';
import { buildTaskPath } from '@/lib/pmTaskLinks';
import { SupportLink } from './SupportLink';

const MARKDOWN_REMARK_PLUGINS = [remarkGfm];
const RESTORE_SUPPORT_DRAFT_EVENT = 'support:restore-draft';

export function sanitizeSupportShortcutSeed(content: string): string {
  return content.replace(/\\(\r?\n)/g, '$1');
}

type MentionNode = {
  type: string;
  tagName?: string;
  value?: string;
  properties?: Record<string, unknown>;
  children?: MentionNode[];
};

/** Highlight parsed note text without changing Markdown, links, or code. */
function rehypeNoteMentions() {
  return (tree: MentionNode) => {
    function visit(node: MentionNode) {
      if (!node.children || ['a', 'code', 'pre'].includes(node.tagName ?? '')) return;
      node.children = node.children.flatMap((child): MentionNode[] => {
        if (child.type !== 'text' || !child.value) {
          visit(child);
          return [child];
        }
        const parts: MentionNode[] = [];
        let offset = 0;
        // A mention starts at a word boundary, never inside an email address.
        for (const match of child.value.matchAll(/(?<![\w.@/+-])@[a-zA-Z0-9][a-zA-Z0-9._-]*/g)) {
          const index = match.index;
          if (index > offset) parts.push({ type: 'text', value: child.value.slice(offset, index) });
          parts.push({
            type: 'element', tagName: 'span', properties: { className: ['mention-highlight'] },
            children: [{ type: 'text', value: match[0] }],
          });
          offset = index + match[0].length;
        }
        if (!parts.length) return [child];
        if (offset < child.value.length) parts.push({ type: 'text', value: child.value.slice(offset) });
        return parts;
      });
    }
    visit(tree);
  };
}

const NOTE_REHYPE_PLUGINS = [rehypeNoteMentions];

function containsMarkdownTable(content: string): boolean {
  return /\|(?:[^\n|]+\|){1,}[^\n]*\n\|(?:\s*[-:]+\s*\|){1,}/m.test(content) || /<table[\s>]/i.test(content);
}

function firstDisplayNamePart(name?: string | null): string {
  return name?.trim().split(/\s+/)[0] ?? '';
}

function emailAddressFromHeader(value?: string | null): string {
  const trimmed = value?.trim() ?? '';
  if (!trimmed) return '';
  const match = trimmed.match(/<([^>]+)>/);
  return (match?.[1] ?? trimmed).trim();
}

function supportSystemEventDisplayContent(eventType: string | undefined, content: string, senderName: string): string {
  const actor = firstDisplayNamePart(senderName);
  switch (eventType) {
    case 'assigned':
      return content.trim() || (actor ? `${actor} assigned this conversation.` : 'Conversation assigned.');
    case 'agent_assigned':
      return actor ? `${actor} assigned this conversation to an AI agent.` : 'Assigned to an AI agent.';
    case 'unassigned':
      return actor ? `${actor} moved this conversation to unassigned.` : 'Moved to unassigned.';
    case 'took':
      return actor ? `${actor} took this conversation.` : 'A teammate took this conversation.';
    default:
      return content;
  }
}

function renderAssignedSystemEventContent(content: string): ReactNode {
  const match = content.match(/^(.*\bassigned this conversation to\s+)([^.]+)(\.)$/);
  if (!match) return content;

  const [, prefix, targetName, suffix] = match;
  return (
    <>
      {prefix}
      <strong className="font-semibold text-foreground">{targetName}</strong>
      {suffix}
    </>
  );
}

function boldSupportSystemValue(value: string): ReactNode {
  return <strong className="font-semibold text-foreground">{value}</strong>;
}

function renderBoldedSupportMatches(content: string, pattern: RegExp): ReactNode {
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  let key = 0;
  for (const match of content.matchAll(pattern)) {
    if (match.index === undefined) continue;
    const [fullMatch, prefix, value, suffix] = match;
    const valueIndex = match.index + prefix.length;
    if (valueIndex > lastIndex) {
      parts.push(content.slice(lastIndex, valueIndex));
    }
    parts.push(<strong key={key++} className="font-semibold text-foreground">{value}</strong>);
    lastIndex = match.index + fullMatch.length - suffix.length;
  }
  if (lastIndex < content.length) {
    parts.push(content.slice(lastIndex));
  }
  return parts.length > 1 ? <>{parts}</> : content;
}

function supportTaskIDFromMetadata(metadata?: string): string | null {
  if (!metadata?.trim()) return null;
  try {
    const parsed = JSON.parse(metadata) as { task_id?: unknown };
    return typeof parsed.task_id === 'string' && parsed.task_id.trim() ? parsed.task_id.trim() : null;
  } catch {
    return null;
  }
}

function renderSupportAuditSystemEventContent(
  eventType: string | undefined,
  content: string,
  taskHref?: string | null,
): ReactNode {
  if (eventType === 'assigned') {
    return renderAssignedSystemEventContent(content);
  }

  if (eventType === 'mailbox_moved' || eventType === 'triage_routed') {
    return renderBoldedSupportMatches(content, /(\bmoved to inbox ')(.+)('\.)$/g);
  }

  if (eventType === 'email_recipients_updated') {
    return renderBoldedSupportMatches(
      content,
      /(\b(?:made|added|removed)\s+)(\S+@\S+?)(\s+(?:the primary recipient|to Cc|from Cc)\.)/g,
    );
  }

  if (eventType === 'tag_added' || eventType === 'tag_removed') {
    return renderBoldedSupportMatches(content, /(\b(?:added|removed) tag\s+)([^.]+)(\.)/g);
  }

  if (eventType === 'task_created') {
    const taskMatch = content.match(/^(.*\bcreated task\s+)(#[^:\s]+)(?::\s+(.+))?(\.)$/);
    if (taskMatch) {
      const [, prefix, taskKey, taskName, suffix] = taskMatch;
      return (
        <span data-task-created-event className="flex min-w-0 max-w-full items-center whitespace-nowrap">
          <span data-task-created-prefix className="mr-1 shrink-0">{prefix.trimEnd()}</span>
          {taskHref ? (
            <a data-task-created-link href={taskHref} title={`${taskKey}${taskName ? `: ${taskName}` : ''}`} className="-mx-1 flex min-w-0 items-center rounded-md px-1 font-semibold text-foreground underline decoration-border underline-offset-2 transition-[color,background-color,text-decoration-color] duration-150 hover:bg-muted hover:text-primary hover:decoration-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50">
              <strong className="shrink-0 font-semibold text-foreground">{taskKey}</strong>
              {taskName ? <><span className="shrink-0">:&nbsp;</span><strong data-task-created-title className="truncate font-semibold text-foreground">{taskName}</strong></> : null}
            </a>
          ) : (
            <span className="flex min-w-0 items-center">
              {boldSupportSystemValue(taskKey)}
              {taskName ? <><span className="shrink-0">:&nbsp;</span><strong data-task-created-title className="truncate font-semibold text-foreground">{taskName}</strong></> : null}
            </span>
          )}
          <span className="shrink-0">{suffix}</span>
        </span>
      );
    }
  }

  return content;
}

const markdownBaseComponents = {
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

function LinkPreviewCard({ preview, security }: { preview: SupportLinkPreview; security?: SupportLinkSecurity }) {
  // Both incoming (`bg-muted`) and outgoing (`bg-blue-50`) bubbles are light,
  // so foreground/muted-foreground tokens read well on either. We dropped the
  // separate isOutgoing styling that assumed a dark/saturated outgoing bubble.
  return (
    <SupportLink
      href={preview.url}
      security={security}
      showInlineIndicator={false}
      className="block overflow-hidden rounded-xl border border-border bg-background text-foreground transition-colors hover:opacity-95"
    >
      <div className="flex items-center gap-2.5 px-3 py-2">
        <div className="min-w-0 flex-1 space-y-0.5">
          <div className="flex min-w-0 items-center gap-1.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
            <span className="truncate">{preview.site_name || previewHostLabel(preview)}</span>
            {security?.status === 'malicious' ? <span className="shrink-0 text-red-600 dark:text-red-400">Potentially harmful</span> : preview.url.toLowerCase().startsWith('http://') ? <span className="shrink-0 text-amber-600 dark:text-amber-400">Not secure</span> : null}
          </div>
          <div className="truncate text-sm font-semibold leading-snug">{preview.title}</div>
        </div>
        <LinkSquare01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
      </div>
    </SupportLink>
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

export interface MessageBubbleProps {
 translatedContent?: string;
 translationFooter?: ReactNode;
 translationStatus?: ReactNode;
  message: SupportMessage;
  isConsecutive?: boolean;
  isLastInGroup?: boolean;
  source?: TicketSource;
  receiptStatus?: SupportReceiptStatus;
  contactLastSeenAt?: string;
  fallbackAvatarUrl?: string;
  teammateDisplayName?: string;
  customerDisplayName?: string;
  customerEmail?: string | null;
  workspaceSlug?: string;
  linkedTaskId?: string | null;
}

export const MessageBubble = memo(function MessageBubble({
  translatedContent,
  translationFooter,
  translationStatus,
  message,
  isConsecutive,
  isLastInGroup = true,
  source,
  receiptStatus,
  contactLastSeenAt,
  fallbackAvatarUrl,
  teammateDisplayName,
  customerDisplayName,
  customerEmail,
  workspaceSlug,
  linkedTaskId,
}: MessageBubbleProps) {
  const queryClient = useQueryClient();
  const inboundIdentity = useMemo(() => {
    try { return JSON.parse(message.metadata || '{}') as { email_sender?: string; email_unknown_sender?: boolean; email_participant_sender?: boolean }; }
    catch { return {}; }
  }, [message.metadata]);
  const retryAttachment = useCallback(async (id: string) => {
    await api.post(`/support/inbox/messages/${message.id}/attachments/${id}/retry?workspace_id=${encodeURIComponent(message.workspace_id)}`, {});
    await queryClient.invalidateQueries({ queryKey: ['support', message.workspace_id, 'conversations', message.conversation_id] });
  }, [message.id, message.workspace_id, message.conversation_id, queryClient]);
  const currentUser = useAuthStore((s) => s.user);
  const aiMeta = useMemo<AIMessageMetadata | null>(() => parseAIMessageMetadata(message.metadata), [message.metadata]);
  const aiConfidence = typeof aiMeta?.ai_confidence === 'number' && Number.isFinite(aiMeta.ai_confidence)
    && aiMeta.ai_confidence >= 0 && aiMeta.ai_confidence <= 1 ? aiMeta.ai_confidence : null;
  const linkPreviews = useMemo<SupportLinkPreview[]>(() => parseSupportLinkPreviews(message.metadata), [message.metadata]);
  const linkSecurity = useMemo<SupportLinkSecurity[]>(() => parseSupportLinkSecurity(message.metadata), [message.metadata]);
  const displayedReceiptStatus: SupportReceiptStatus = isExternalSupportEmailReply(message.metadata)
    ? 'sent_outside_helpin'
    : receiptStatus ?? null;
  const markdownComponents = useMemo(() => ({
    ...markdownBaseComponents,
    a: ({ href, children }: ComponentPropsWithoutRef<'a'>) => (
      <SupportLink
        href={href}
        security={href ? findSupportLinkSecurity(linkSecurity, href) : undefined}
        className="break-all [overflow-wrap:anywhere]"
      >
        {children}
      </SupportLink>
    ),
  }), [linkSecurity]);
  const forwardedAttribution = useMemo(() => parseForwardedAttributionMetadata(message.metadata), [message.metadata]);
  const effectiveSenderType = getEffectiveSenderType(message);
  const isCustomer = effectiveSenderType === 'customer';
  const isAI = effectiveSenderType === 'ai';
  const isAgent = effectiveSenderType === 'agent';
  const isInternal = message.is_internal;
  const visitorFeedback = useMemo<boolean | undefined>(() => {
    if (!isAI || isInternal) return undefined;
    try {
      const metadata = JSON.parse(message.metadata || '{}');
      return typeof metadata.visitor_feedback?.helpful === 'boolean'
        ? metadata.visitor_feedback.helpful : undefined;
    } catch { return undefined; }
  }, [isAI, isInternal, message.metadata]);
  const feedbackLabel = visitorFeedback ? 'Visitor marked helpful' : 'Visitor marked unhelpful';
  const deliveryMode = !isCustomer && !isInternal ? getReplyDeliveryMode(message.metadata) : undefined;
  const explicitEmailState = deliveryMode && deliveryMode !== 'chat_only' ? getExplicitEmailDeliveryState(message) : undefined;
  const chatSeen = receiptStatus === 'read' || (source === 'widget' && !!contactLastSeenAt && Date.parse(contactLastSeenAt) >= Date.parse(message.created_at));
  const chatDeliveryLabel = message.id.startsWith('optimistic-') ? 'Sending' : chatSeen ? 'Seen' : 'Sent';
  const isOwnMessage = !!message.sender_user_id && message.sender_user_id === currentUser?.id;
  const senderName = message.sender_display_name
    ?? (isCustomer ? (customerDisplayName || 'Customer') : isAI ? HELPIN_AI_DISPLAY_NAME : isAgent ? 'Agent' : isOwnMessage ? currentUser?.full_name ?? 'You' : 'Teammate');
  const resolvedSenderName = isAI ? HELPIN_AI_DISPLAY_NAME : senderName;
  const showAvatar = isLastInGroup;
  const fullTimestamp = formatTimestamp(message.created_at);
  const bubbleTime = new Date(message.created_at).toLocaleTimeString(undefined, {
    hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
  });
  const renderBubbleTime = (className = '') => (
    <time
      dateTime={message.created_at}
      title={fullTimestamp}
      className={`select-none whitespace-nowrap text-[10px] leading-4 tabular-nums ${className}`}
    >
      {bubbleTime}
    </time>
  );
  const messageSource = message.via_channel ?? source;
  const sourceLabel = isInternal ? null : deliveryMode ? REPLY_DELIVERY_LABELS[deliveryMode]
    : messageSource ? SOURCE_LABELS[messageSource] ?? messageSource : null;

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
  const forwardedDisplayContent = useMemo(() => {
    if (!forwardedAttribution) return '';
    if (!hasForwardedHeaderMarker(displayContent)) return '';
    return cleanForwardedDisplayContent(displayContent).trim();
  }, [displayContent, forwardedAttribution]);
  const projectedEmailVisibleContent = message.via_channel === 'email'
    ? message.email_visible_text?.trim() ?? ''
    : '';
  const visibleContent = translatedContent ?? (forwardedDisplayContent || projectedEmailVisibleContent || displayContent);
  const hasTableContent = useMemo(() => containsMarkdownTable(visibleContent), [visibleContent]);

  const [sourcesOpen, setSourcesOpen] = useState(false);
  const [emailDetailOpen, setEmailDetailOpen] = useState(false);
  const [infoOpen, setInfoOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const deleteMutation = useDeleteSupportMessage(message.workspace_id, message.conversation_id);

  const imageAttachments = message.attachments?.filter(a => a.file_type.startsWith('image/')) ?? [];
  const fileAttachments = message.attachments?.filter(a => !a.file_type.startsWith('image/')) ?? [];
  const hasDisplayContent = visibleContent.trim().length > 0;
  const showBubble = !!visibleContent || fileAttachments.length > 0 || imageAttachments.length > 0 || linkPreviews.length > 0;
  const hasEmailBody = translatedContent === undefined && message.via_channel === 'email' && !!message.html_body;
  const renderEmailBodyAsForwardedText = hasEmailBody && !!forwardedDisplayContent;

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
  const cancellableActive = canMutateOwnReply && Number.isFinite(cancellableUntilMs) && cancellableUntilMs > Date.now();
  const hasCancellableFooter = cancellableActive;

  const restoreComposerDraft = useCallback((markdown: string) => {
    window.dispatchEvent(new CustomEvent(RESTORE_SUPPORT_DRAFT_EVENT, {
      detail: { conversationId: message.conversation_id, markdown, attachments: message.attachments ?? [], deliveryMode, emailSubject: getReplyEmailSubject(message.metadata) },
    }));
  }, [message.attachments, message.conversation_id, message.metadata, deliveryMode]);

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
    void navigator.clipboard?.writeText(visibleContent);
    toast.success('Message copied');
  }, [visibleContent]);

  const canSaveAsShortcut = visibleContent.trim().length > 0 && message.message_type !== 'system';
  const openShortcutComposer = useShortcutComposerStore((s) => s.openCreate);
  const handleSaveAsShortcut = useCallback(
    () => openShortcutComposer({ seedContent: sanitizeSupportShortcutSeed(visibleContent) }),
    [openShortcutComposer, visibleContent],
  );

  const handleQuoteReply = useCallback(() => {
    const quoted = visibleContent
      .split('\n')
      .map((line) => `> ${line}`)
      .join('\n');
    restoreComposerDraft(`${quoted}\n\n`);
  }, [visibleContent, restoreComposerDraft]);

  const renderFileAttachments = (tone: 'default' | 'note' = 'default', className = '') => {
    if (fileAttachments.length === 0) return null;

    return <SupportAttachmentGallery attachments={fileAttachments} onRetry={retryAttachment} tone={tone} className={className} />;
  };

  const renderImageAttachments = (className = '') => {
    if (imageAttachments.length === 0) return null;

    return <SupportAttachmentGallery attachments={imageAttachments} onRetry={retryAttachment} className={className} />;
  };

  const resolvedAvatarUrl = message.sender_avatar_url
    ?? (inboundIdentity.email_participant_sender || (inboundIdentity.email_sender && customerEmail && inboundIdentity.email_sender.toLowerCase() !== customerEmail.toLowerCase()) ? undefined : fallbackAvatarUrl)
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
  const avatarSeed = message.sender_user_id || message.sender_agent_id || inboundIdentity.email_sender || resolvedSenderName;
  const fallbackAvatar = (
    <div
      className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[10.5px] font-semibold leading-none shadow-sm ${getAvatarColor(avatarSeed)}`}
    >
      {getInitial(resolvedSenderName)}
    </div>
  );

  const aiActivity = getSupportAIActivity(message);
  if (aiActivity) {
    return <SupportAIActivity
      message={message}
      activity={aiActivity}
      teammateName={teammateDisplayName}
      avatarUrl={resolvedAvatarUrl}
      detailsContent={<Markdown remarkPlugins={MARKDOWN_REMARK_PLUGINS} components={markdownComponents}>{message.content}</Markdown>}
    />;
  }

  // ── System message: centered pill with avatar ──
  if (message.message_type === 'system' && message.system_event_type !== 'delayed_team_reply') {
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
      'reopened',
      'email_recipients_updated',
      'tag_added',
      'tag_removed',
      'task_created',
    ];

    const stateEventTypes: ReadonlyArray<string> = ['resolved', 'closed'];
    const eventType = message.system_event_type;

    const ESCALATION_LABELS: Record<string, string> = {
      ai_escalated: 'AI escalated to a human',
      customer_requested_human: 'Customer requested a human',
    };
    const isEscalationEvent = !!eventType && eventType in ESCALATION_LABELS;
    const escalationLabel = isEscalationEvent ? ESCALATION_LABELS[eventType] : null;
    const isRuleRoutingEvent = eventType === 'triage_routed' && message.content.trim().toLowerCase().startsWith('routing rule ');
    const isAIRoutingEvent = eventType === 'triage_routed' && !isRuleRoutingEvent;
    const showAIAvatar = eventType !== 'customer_requested_human' && !isRuleRoutingEvent
      && (isAI || isAgent || isAIRoutingEvent || eventType === 'ai_escalated');
    const aiStatusAvatar = <AskAgentAvatar plateStyle="solid" radius={50} className="h-5 w-5 shrink-0" decorative={false} label={isAIRoutingEvent ? 'AI routing' : HELPIN_AI_DISPLAY_NAME} />;
    const systemDisplayContent = escalationLabel ?? supportSystemEventDisplayContent(eventType, message.content, resolvedSenderName);
    const taskID = eventType === 'task_created'
      ? supportTaskIDFromMetadata(message.metadata) ?? linkedTaskId
      : null;
    const taskHref = taskID && workspaceSlug ? buildTaskPath(workspaceSlug, taskID) : null;
    const systemDisplayNode = renderSupportAuditSystemEventContent(eventType, systemDisplayContent, taskHref);

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

    const statusIcon = stateEventKind === 'resolved' ? (
      <span aria-label="Resolved" className="flex shrink-0 text-emerald-600 dark:text-emerald-300">
        <CheckmarkCircle02Icon className="h-4 w-4" />
      </span>
    )
      : stateEventKind === 'reopened' ? (
        <span aria-label="Reopened" className="flex shrink-0">
          <RotateLeft01Icon className="h-3.5 w-3.5" />
        </span>
      )
        : stateEventKind === 'closed' ? (
          <span aria-label="Closed" className="flex shrink-0">
            <CancelCircleIcon className="h-4 w-4" />
          </span>
        )
      : null;
    const statePillClass = stateEventKind === 'resolved'
      ? 'rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-800 shadow-sm dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200'
      : 'rounded-full bg-slate-700 px-4 py-2 text-white shadow-sm';
    const resolvedActorAvatar = stateEventKind === 'resolved' ? (
      showAIAvatar ? aiStatusAvatar : resolvedAvatarUrl ? (
        <img src={resolvedAvatarUrl} alt={resolvedSenderName} className="h-5 w-5 rounded-full object-cover" />
      ) : (
        <div aria-label={resolvedSenderName} className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[9px] font-semibold leading-none ${getAvatarColor(avatarSeed)}`}>
          {getInitial(resolvedSenderName)}
        </div>
      )
    ) : null;

    // Routing events use a neutral muted style with leading avatar; state
    // transitions keep the stronger slate pill so they stay visually distinct.
    if (isRoutingEvent) {
      const escalationPillClass = 'rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200';
      const defaultPillClass = 'rounded-full px-3 py-1 text-xs text-muted-foreground';
      return (
        <div className="my-5 flex items-center justify-center gap-2 animate-in fade-in duration-300">
          <Tooltip>
            <TooltipTrigger asChild>
              <div data-support-system-callout className={`flex min-w-0 items-center gap-2 ${eventType === 'task_created' ? 'max-w-[70%]' : 'max-w-full'} ${isEscalationEvent ? escalationPillClass : defaultPillClass}`}>
                {showAIAvatar ? aiStatusAvatar : isEscalationEvent ? (
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                    <UserIcon className="h-3 w-3" />
                  </span>
                ) : isRuleRoutingEvent ? (
                  <span aria-label="Routing rule" className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                    <ZapIcon className="h-3 w-3" />
                  </span>
                ) : resolvedAvatarUrl ? (
                  <img src={resolvedAvatarUrl} alt={resolvedSenderName} className="h-5 w-5 rounded-full object-cover" />
                ) : (
                  <div className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[9px] font-semibold leading-none ${getAvatarColor(avatarSeed)}`}>
                    {getInitial(resolvedSenderName)}
                  </div>
                )}
                <span className="min-w-0 max-w-full">{systemDisplayNode}</span>
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
            <div data-support-system-callout className={`flex min-w-0 max-w-full items-center gap-2.5 [overflow-wrap:anywhere] ${statePillClass}`}>
              {statusIcon ?? <CheckmarkCircle02Icon className="h-4 w-4 shrink-0" />}
              {resolvedActorAvatar ?? (showAIAvatar ? aiStatusAvatar : null)}
              <span className={`min-w-0 ${stateEventKind === 'resolved' ? 'font-medium' : 'text-sm font-medium'}`}>{systemDisplayNode}</span>
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

  const avatarEl = isCustomer ? (
    <Tooltip>
      <TooltipTrigger asChild>
        <Avatar className="h-7 w-7 shadow-sm">
          <ContactAvatarImage
            email={inboundIdentity.email_sender || emailAddressFromHeader(message.email_from) || (inboundIdentity.email_participant_sender ? undefined : customerEmail)}
            src={resolvedAvatarUrl}
            alt={resolvedSenderName}
          />
          <AvatarFallback className={`text-[10.5px] font-semibold leading-none ${getAvatarColor(avatarSeed)}`}>
            {getInitial(resolvedSenderName)}
          </AvatarFallback>
        </Avatar>
      </TooltipTrigger>
      <TooltipContent side="left"><span className="text-xs font-medium">{resolvedSenderName}</span></TooltipContent>
    </Tooltip>
  ) : isAI || isAgent ? (
    <Tooltip>
      <TooltipTrigger asChild>
        <AskAgentAvatar plateStyle="solid" radius={50} className="h-7 w-7" decorative={false} label={resolvedSenderName} />
      </TooltipTrigger>
      <TooltipContent side="right"><span className="text-xs font-medium">{resolvedSenderName}</span></TooltipContent>
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

  const bubbleWidthClass = hasEmailBody && !renderEmailBodyAsForwardedText
    ? 'min-w-0 w-[min(92%,64rem)] max-w-[calc(100%-2.25rem)]'
    : hasTableContent
      ? 'min-w-0 max-w-[min(85%,46rem)] lg:max-w-[min(85%,48rem)]'
      : 'min-w-0 max-w-[min(85%,42rem)]';

  // ── Internal note: team bubble with the same author avatar as replies ──
  if (isInternal) {
    return (
      <>
        <div className={`flex min-w-0 max-w-full justify-end ${isConsecutive ? 'mt-1' : 'mt-5'}`}>
          <div className={bubbleWidthClass}>
            <Tooltip>
              <TooltipTrigger asChild>
                <div data-support-internal-note className={`flow-root rounded-2xl border border-border/40 bg-amber-50 px-3.5 py-2 [overflow-wrap:anywhere] dark:bg-amber-950/20 ${isLastInGroup ? 'rounded-br-sm' : ''}`}>
                  <div className="mb-1.5 flex items-center gap-1.5">
                    <StickyNote01Icon className="h-3 w-3 text-amber-500 dark:text-amber-400" />
                    <span className="text-[11px] text-amber-600 dark:text-amber-400">
                      {inboundIdentity.email_unknown_sender ? 'Team only' : 'Internal note'}
                    </span>
                  </div>
                  {hasDisplayContent && (
                    <div className="prose-chat inline text-sm leading-relaxed text-amber-900 [&>p:last-child]:inline dark:text-amber-200">
                      <Markdown remarkPlugins={MARKDOWN_REMARK_PLUGINS} rehypePlugins={NOTE_REHYPE_PLUGINS} components={markdownComponents}>{visibleContent}</Markdown>
                    </div>
                  )}
                  {renderFileAttachments('note', hasDisplayContent ? 'mt-2' : 'mt-1.5')}
                  {renderImageAttachments(hasDisplayContent || fileAttachments.length > 0 ? 'mt-2' : 'mt-1.5')}
                  {renderBubbleTime('float-right ml-2 mt-1 text-amber-700/70 dark:text-amber-300/70')}
                </div>
              </TooltipTrigger>
              <TooltipContent side="top">{inboundIdentity.email_unknown_sender ? "This sender is not a participant. Only your team can see this message; no automatic reply is sent." : tooltipContent}</TooltipContent>
            </Tooltip>
          </div>
          <div className="ml-2 flex w-7 shrink-0 flex-col justify-end">
            {showAvatar && avatarEl}
          </div>
        </div>
      </>
    );
  }

  // ── Chat bubble ──
  const hasEmailBadge = message.via_channel === 'email';
  const inboundFromEmail = isCustomer ? emailAddressFromHeader(message.email_from) : '';
  const inboundReplyToEmail = isCustomer ? emailAddressFromHeader(message.email_reply_to) : '';
  const inboundCustomerEmail = customerEmail?.trim() ?? '';
  const inboundFromMatchesKnownCustomerEmail = inboundFromEmail !== '' && (
    (inboundReplyToEmail !== '' && inboundFromEmail.toLowerCase() === inboundReplyToEmail.toLowerCase())
    || (inboundCustomerEmail !== '' && inboundFromEmail.toLowerCase() === inboundCustomerEmail.toLowerCase())
  );
  const inboundEmailBadgeLabel = inboundFromEmail
    ? inboundFromMatchesKnownCustomerEmail
      ? 'Received by email'
      : `Received by email from ${inboundFromEmail}`
    : 'Received via email';
  const hasEmailReceiptStatus = displayedReceiptStatus === 'sending_email' || displayedReceiptStatus === 'sent_email' || displayedReceiptStatus === 'delivered_email' || displayedReceiptStatus === 'read_email' || displayedReceiptStatus === 'sent_outside_helpin';
  const showStandaloneEmailBadge = !deliveryMode && hasEmailBadge && !(hasEmailReceiptStatus && !isCustomer);
  const hasStatusBelow = !!deliveryMode || !!displayedReceiptStatus || aiConfidence !== null || !!aiMeta?.ai_sources?.length || hasEmailBadge;
  const messageActionsMenu = (
    <MessageActionsMenu
      alignSide={isCustomer ? 'right' : 'left'}
      canEdit={!message.pending_send && cancellableActive}
      canDelete={!message.pending_send && canMutateOwnReply}
      onEdit={handleUndoOrEdit}
      onCopy={handleCopy}
      onReply={handleQuoteReply}
      onDelete={() => setDeleteDialogOpen(true)}
      onInfo={() => setInfoOpen(true)}
      onSaveAsShortcut={canSaveAsShortcut ? handleSaveAsShortcut : undefined}
    />
  );

  return (
    <div className={`${isConsecutive ? 'mt-1' : 'mt-5'} ${!isConsecutive ? (isCustomer ? 'animate-in fade-in slide-in-from-left-2 duration-200' : 'animate-in fade-in slide-in-from-right-2 duration-200') : ''}`}>
      {/* Bubble row: avatar + bubble aligned together */}
      <div className={`flex min-w-0 max-w-full ${isCustomer ? 'justify-start' : 'justify-end'}`}>
        {/* Left side: avatar or spacer (customer messages) */}
        {isCustomer && (
          <div className="mr-2 flex w-7 shrink-0 flex-col justify-end">
            {showAvatar && avatarEl}
          </div>
        )}

        <MessageActionsContextMenu
          canEdit={!message.pending_send && cancellableActive}
          canDelete={!message.pending_send && canMutateOwnReply}
          onEdit={handleUndoOrEdit}
          onCopy={handleCopy}
          onReply={handleQuoteReply}
          onDelete={() => setDeleteDialogOpen(true)}
          onInfo={() => setInfoOpen(true)}
          onSaveAsShortcut={canSaveAsShortcut ? handleSaveAsShortcut : undefined}
        >
        <div
          data-slot="support-message-bubble"
          className={`${message.pending_send && message.pending_send !== 'failed' ? 'opacity-70' : ''} ${bubbleWidthClass} group/message ${showBubble ? '' : 'relative'}`}
        >
          {showBubble ? (
            <div data-slot="support-message-bubble-frame" className="relative">
              {!message.pending_send && messageActionsMenu}
              <Tooltip>
                <TooltipTrigger asChild>
                  <div
                    className={`flow-root rounded-2xl border border-border/40 px-3.5 py-2 text-sm leading-relaxed [overflow-wrap:anywhere] ${
                      isCustomer
                        ? `bg-muted text-foreground/85 dark:text-foreground ${isLastInGroup ? 'rounded-bl-sm' : ''}`
                        : `bg-blue-50 text-foreground/85 dark:bg-blue-950/40 dark:text-foreground ${isLastInGroup ? 'rounded-br-sm' : ''}`
                    } ${hasTableContent || (hasEmailBody && !renderEmailBodyAsForwardedText) ? 'overflow-hidden' : ''}`}
                  >
                    {hasEmailBody && !renderEmailBodyAsForwardedText ? (
                      <div className="-mx-1" data-chat-tone={isCustomer ? 'customer' : 'agent'}>
                        <EmailBodyRenderer html={message.html_body ?? ''} collapsedByDefault />
                      </div>
                    ) : (
                      visibleContent && (
                        <div
                          className="prose-chat inline [&>p:last-child]:inline"
                          data-chat-tone={isCustomer ? 'customer' : 'agent'}
                          data-has-table={hasTableContent ? 'true' : 'false'}
                        >
                          <Markdown remarkPlugins={MARKDOWN_REMARK_PLUGINS} components={markdownComponents}>{visibleContent}</Markdown>
                        </div>
                      )
                    )}
                    {fileAttachments.length > 0 && (
                      renderFileAttachments('default', visibleContent ? 'mt-2' : '')
                    )}
                    {linkPreviews.length > 0 && (
                      <div className={`${visibleContent || fileAttachments.length > 0 ? 'mt-2' : ''} space-y-2`}>
                        {linkPreviews.map((preview) => (
                          <LinkPreviewCard
                            key={`${message.id}:${preview.url}`}
                            preview={preview}
                            security={findSupportLinkSecurity(linkSecurity, preview.url)}
                          />
                        ))}
                      </div>
                    )}
                    {renderImageAttachments(hasDisplayContent || fileAttachments.length > 0 || linkPreviews.length > 0 ? 'mt-2' : '')}
                    {translationFooter ? <div data-slot="support-message-footer" className="mt-1 flex items-end justify-between gap-3">
                      {translationFooter}
                      {renderBubbleTime('ml-auto shrink-0 text-muted-foreground')}
                    </div> : renderBubbleTime('float-right ml-2 mt-1 text-muted-foreground')}
                  </div>
                </TooltipTrigger>
                <TooltipContent side="top">
                  {tooltipContent}
                </TooltipContent>
              </Tooltip>
            </div>
          ) : (
            messageActionsMenu
          )}
          {visitorFeedback !== undefined && (
            <div data-slot="support-answer-feedback" className="mt-1 flex h-5 justify-start">
              <Tooltip>
                <TooltipTrigger asChild>
                  <span role="img" tabIndex={0} aria-label={feedbackLabel}
                    className="inline-flex h-5 min-w-6 items-center justify-center rounded-full border border-border/60 bg-background px-1 text-sm leading-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                    {visitorFeedback ? '👍' : '👎'}
                  </span>
                </TooltipTrigger>
                <TooltipContent side="bottom">{feedbackLabel}</TooltipContent>
              </Tooltip>
            </div>
          )}
        </div>
        </MessageActionsContextMenu>

        {/* Right side: avatar or spacer (agent/user messages) */}
        {!isCustomer && (
          <div className={`ml-2 flex w-7 shrink-0 flex-col justify-end ${visitorFeedback !== undefined ? 'pb-6' : ''}`}>
            {showAvatar && avatarEl}
          </div>
        )}
      </div>

      {/* Email detail modal — rendered via Radix portal */}
      {(hasEmailBadge || emailDetailOpen) && (
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
        onViewEmail={() => { setInfoOpen(false); setEmailDetailOpen(true); }}
      />
      <MessageDeleteDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        onConfirm={handleDelete}
        isPending={deleteMutation.isPending}
      />
      {/* Status below the bubble row — outside the avatar alignment */}
      {translationStatus && <div data-slot="support-message-translation-status" className={`mt-1 flex text-xs text-muted-foreground ${isCustomer ? 'justify-start pl-9' : 'justify-end pr-9'}`}>{translationStatus}</div>}
      {message.pending_send && <PendingSendStatus message={message}/>}
      {!message.pending_send && (hasStatusBelow || hasCancellableFooter) && (
        <div className={`mt-0.5 ${isCustomer ? 'pl-9' : 'pr-9'}`}>
          {showStandaloneEmailBadge && (
            <div className={`mb-0.5 space-y-0.5 ${isCustomer ? '' : 'text-right'}`}>
              <div className={`flex ${isCustomer ? '' : 'justify-end'}`}>
                <button
                  type="button"
                  onClick={() => setEmailDetailOpen(true)}
                  className="inline-flex items-center gap-1 text-[11px] text-muted-foreground transition-colors hover:text-foreground hover:underline"
                >
                  <Mail01Icon className="h-3 w-3" />
                  {forwardedAttribution && isCustomer
                    ? `Forwarded by ${forwardedAttribution.forwarded_by_name || forwardedAttribution.forwarded_by_email}`
                    : isCustomer ? inboundEmailBadgeLabel : 'Sent via email'}
                </button>
              </div>
            </div>
          )}

          {/* Delivery failure indicator — supersedes the read receipt when the outbound email bounced or was marked spam. */}
          {deliveryMode ? (
            <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
              {deliveryMode !== 'email_only' && (
                <span className="inline-flex items-center gap-1">
                  <BubbleChatIcon className="h-3.5 w-3.5" aria-hidden="true" />
                  {deliveryMode === 'chat_only' ? 'Chat only' : 'Chat'} · {chatDeliveryLabel}
                </span>
              )}
              {explicitEmailState && (
                <button
                  type="button"
                  onClick={() => setInfoOpen(true)}
                  className={`inline-flex items-center gap-1 text-left transition-colors hover:underline ${explicitEmailState.failed ? 'text-quiet-accent' : 'hover:text-foreground'}`}
                >
                  <Mail01Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span>
                    {deliveryMode === 'email_only' ? 'Email only' : 'Email'} · {explicitEmailState.label}
                    {explicitEmailState.error ? ` · ${explicitEmailState.error}` : ''}
                  </span>
                </button>
              )}
              {cancellableActive && (
                <button
                  type="button"
                  className="font-medium text-foreground transition-colors hover:text-primary hover:underline"
                  onClick={handleUndoOrEdit}
                  disabled={deleteMutation.isPending}
                >
                  Undo
                </button>
              )}
            </div>
          ) : (message.email_delivery_status === 'bounced' || message.email_delivery_status === 'spam_complaint') ? (
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
                  <span>Queued for email</span>
                  <span>·</span>
                  <button
                    type="button"
                    className="font-medium text-foreground transition-colors hover:text-primary hover:underline"
                    onClick={handleUndoOrEdit}
                    disabled={deleteMutation.isPending}
                  >
                    Undo
                  </button>
                </>
              ) : (
                <span>Delivered to email</span>
              )}
            </div>
          ) : aiMeta && (aiConfidence !== null || !!aiMeta.ai_sources?.length || !!displayedReceiptStatus) ? (
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
                  {aiConfidence !== null && (
                    <span className="inline-flex items-center gap-1 rounded-full border border-primary/20 bg-primary/10 px-2 py-0.5 font-medium text-primary">
                      <CheckmarkCircle02Icon className="h-3 w-3" />
                      {(aiConfidence * 100).toFixed(0)}% confident
                    </span>
                  )}
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
                {displayedReceiptStatus && (
                  <button
                    type="button"
                    onClick={() => setInfoOpen(true)}
                    className="inline-flex shrink-0 items-center gap-1 text-[11px] text-muted-foreground transition-colors hover:text-foreground hover:underline"
                  >
                    {displayedReceiptStatus === 'sent_outside_helpin' ? (
                      <>
                        <Mail01Icon className="h-3.5 w-3.5" />
                        Sent outside Helpin
                      </>
                    ) : displayedReceiptStatus === 'read' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                        Read in chat
                      </>
                    ) : displayedReceiptStatus === 'read_email' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                        Read via email
                      </>
                    ) : displayedReceiptStatus === 'delivered_email' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5" />
                        Delivered via email
                      </>
                    ) : displayedReceiptStatus === 'sending_email' ? (
                      <>
                        <TickDouble01Icon className="h-3.5 w-3.5" />
                        Sending email
                      </>
                    ) : displayedReceiptStatus === 'sent_email' ? (
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
                  </button>
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
          ) : displayedReceiptStatus && (
            <div className={`flex items-center gap-1 ${isCustomer ? '' : 'justify-end'}`}>
              <button
                type="button"
                onClick={() => setInfoOpen(true)}
                className="inline-flex items-center gap-1 text-[11px] text-muted-foreground transition-colors hover:text-foreground hover:underline"
              >
                {displayedReceiptStatus === 'sent_outside_helpin' ? (
                  <>
                    <Mail01Icon className="h-3.5 w-3.5" />
                    Sent outside Helpin
                  </>
                ) : displayedReceiptStatus === 'read' ? (
                  <>
                    <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                    Read in chat
                  </>
                ) : displayedReceiptStatus === 'read_email' ? (
                  <>
                    <TickDouble01Icon className="h-3.5 w-3.5 text-blue-500" />
                    Read via email
                  </>
                ) : displayedReceiptStatus === 'delivered_email' ? (
                  <>
                    <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                    Delivered via email
                  </>
                ) : displayedReceiptStatus === 'sending_email' ? (
                  <>
                    <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                    Sending email
                  </>
                ) : displayedReceiptStatus === 'sent_email' ? (
                  <>
                    <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                    Sent via email
                  </>
                ) : (
                  <>
                    <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                    Delivered
                  </>
                )}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
});
