import { memo, useMemo, useState, type ComponentPropsWithoutRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { TickDouble01Icon, CheckmarkCircle02Icon, ArrowDown01Icon, ArrowUp01Icon, Download04Icon, LinkSquare01Icon, File01Icon, AttachmentIcon, RotateLeft01Icon, StickyNote01Icon, Cancel01Icon, CancelCircleIcon, Mail01Icon } from '@/lib/icons';
import { EmailDetailModal } from './EmailDetailModal';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import type { AIMessageMetadata, SupportLinkPreview, SupportMessage, TicketSource } from '@/lib/pmTypes';
import { formatTimestamp, getInitial, getAvatarColor, getEffectiveSenderType, HELPIN_AI_DISPLAY_NAME, parseAIMessageMetadata, parseSupportLinkPreviews } from './helpers';

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
      className="[overflow-wrap:anywhere] break-words"
    >
      {children}
    </a>
  ),
  table: ({ children }: ComponentPropsWithoutRef<'table'>) => (
    <div className="chat-markdown-table-wrap">
      <table>{children}</table>
    </div>
  ),
};

const SOURCE_LABELS: Record<string, string> = {
  widget: 'Chat Widget',
  email: 'Email',
  internal: 'Internal',
  api: 'API',
};

const SENDER_TYPE_LABELS: Record<string, string> = {
  customer: 'Customer',
  user: 'Agent',
  agent: 'Agent',
  ai: 'AI Agent',
};

function previewHostLabel(preview: SupportLinkPreview): string {
  try {
    return new URL(preview.url).hostname.replace(/^www\./, '') || preview.host;
  } catch {
    return preview.host.replace(/^www\./, '');
  }
}

function LinkPreviewCard({ preview, isOutgoing }: { preview: SupportLinkPreview; isOutgoing: boolean }) {
  return (
    <a
      href={preview.url}
      target="_blank"
      rel="noopener noreferrer"
      className={`block overflow-hidden rounded-xl border transition-colors hover:opacity-95 ${
        isOutgoing
          ? 'border-white/20 bg-white/10 text-white'
          : 'border-border bg-background text-foreground'
      }`}
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
        <div className={`flex items-center gap-1.5 text-[11px] uppercase tracking-wide ${isOutgoing ? 'text-white/70' : 'text-muted-foreground'}`}>
          <span className="truncate">{preview.site_name || previewHostLabel(preview)}</span>
          <LinkSquare01Icon className="h-3 w-3 shrink-0" />
        </div>
        <div className="text-sm font-semibold leading-snug">{preview.title}</div>
        {preview.description ? (
          <p className={`text-xs leading-relaxed ${isOutgoing ? 'text-white/80' : 'text-muted-foreground'}`}>
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
  receiptStatus?: 'delivered' | 'delivered_email' | 'read' | 'read_email' | null;
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
  const senderLabel = SENDER_TYPE_LABELS[effectiveSenderType] ?? effectiveSenderType;
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

  const imageAttachments = message.attachments?.filter(a => a.file_type.startsWith('image/')) ?? [];
  const fileAttachments = message.attachments?.filter(a => !a.file_type.startsWith('image/')) ?? [];
  const showBubble = !!displayContent || fileAttachments.length > 0 || linkPreviews.length > 0;

  const tooltipContent = (
    <div className="space-y-0.5 text-xs">
      <div className="font-medium">{resolvedSenderName}</div>
      <div className="text-muted-foreground">{fullTimestamp}</div>
      <div className="text-muted-foreground">
        {senderLabel}
        {sourceLabel && ` · via ${sourceLabel}`}
      </div>
    </div>
  );

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
    ];
    const stateEventTypes: ReadonlyArray<string> = ['resolved', 'reopened', 'closed'];
    const eventType = message.system_event_type;

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
      return (
        <div className="my-3 flex items-center justify-center gap-2 animate-in fade-in duration-300">
          <Tooltip>
            <TooltipTrigger asChild>
              <div className="flex items-center gap-2 rounded-full px-3 py-1 text-xs text-muted-foreground">
                {resolvedAvatarUrl ? (
                  <img src={resolvedAvatarUrl} alt={resolvedSenderName} className="h-5 w-5 rounded-full object-cover" />
                ) : (
                  <div className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[9px] font-semibold leading-none ${getAvatarColor(avatarSeed)}`}>
                    {getInitial(resolvedSenderName)}
                  </div>
                )}
                <span>{message.content}</span>
              </div>
            </TooltipTrigger>
            <TooltipContent side="top">
              <div className="text-xs text-muted-foreground">{fullTimestamp}</div>
            </TooltipContent>
          </Tooltip>
        </div>
      );
    }

    return (
      <div className="my-4 flex items-center justify-center gap-2 animate-in fade-in duration-300">
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
              <div className="text-muted-foreground">{fullTimestamp}</div>
            </div>
          </TooltipContent>
        </Tooltip>
      </div>
    );
  }

  // ── Internal note: right-aligned card with amber accent ──
  if (isInternal) {
    return (
      <div className={`flex justify-end ${isConsecutive ? 'mt-1' : 'mt-5'}`}>
        <div className="max-w-[75%]">
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
                <div className="prose-chat text-sm leading-relaxed text-amber-900 dark:text-amber-200">
                  {mentionParts ? (
                    <p className="whitespace-pre-wrap">{mentionParts}</p>
                  ) : (
                    <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>{displayContent}</Markdown>
                  )}
                </div>
              </div>
            </TooltipTrigger>
            <TooltipContent side="top" align="start">{tooltipContent}</TooltipContent>
          </Tooltip>
        </div>
      </div>
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

        <div
          data-slot="support-message-bubble"
          className={hasTableContent ? 'min-w-0 max-w-[min(78vw,46rem)] lg:max-w-[min(72vw,48rem)]' : 'min-w-0 max-w-[70%]'}
        >
          {showBubble && (
            <Tooltip>
              <TooltipTrigger asChild>
                <div
                  className={`rounded-2xl px-3.5 py-2 text-sm leading-relaxed [overflow-wrap:anywhere] ${
                    isCustomer
                      ? `bg-muted text-foreground ${isLastInGroup ? 'rounded-bl-sm' : ''}`
                      : `bg-blue-600 text-white dark:bg-blue-500 ${isLastInGroup ? 'rounded-br-sm' : ''}`
                  } ${hasTableContent ? 'overflow-hidden' : ''}`}
                >
                  {displayContent && (
                    <div
                      className="prose-chat"
                      data-chat-tone={isCustomer ? 'customer' : 'agent'}
                      data-has-table={hasTableContent ? 'true' : 'false'}
                    >
                      <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>{displayContent}</Markdown>
                    </div>
                  )}
                  {fileAttachments.length > 0 && (
                    <div className={`${displayContent ? 'mt-2' : ''} space-y-1.5`}>
                      {fileAttachments.map((att) => (
                        <a
                          key={att.id}
                          href={att.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-xs transition-colors hover:bg-muted/50 ${
                            isCustomer ? 'border-border' : 'border-white/20 text-white hover:bg-white/10'
                          }`}
                        >
                          <AttachmentIcon className="h-3.5 w-3.5 shrink-0 opacity-60" />
                          <span className="truncate font-medium">{att.file_name}</span>
                          <span className="shrink-0 opacity-60">{formatFileSize(att.file_size)}</span>
                          <Download04Icon className="ml-auto h-3.5 w-3.5 shrink-0 opacity-60" />
                        </a>
                      ))}
                    </div>
                  )}
                  {linkPreviews.length > 0 && (
                    <div className={`${displayContent || fileAttachments.length > 0 ? 'mt-2' : ''} space-y-2`}>
                      {linkPreviews.map((preview) => (
                        <LinkPreviewCard
                          key={`${message.id}:${preview.url}`}
                          preview={preview}
                          isOutgoing={!isCustomer}
                        />
                      ))}
                    </div>
                  )}
                </div>
              </TooltipTrigger>
              <TooltipContent side="top" align={isCustomer ? 'start' : 'end'}>
                {tooltipContent}
              </TooltipContent>
            </Tooltip>
          )}

          {/* Image attachments: outside the bubble, clickable for preview */}
          {imageAttachments.length > 0 && (
            <div className={`${showBubble ? 'mt-1.5' : ''} space-y-1.5`}>
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
          )}
        </div>

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

      {/* Lightbox modal — rendered in portal for full-screen overlay */}
      {lightboxSrc && createPortal(
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
      )}

      {/* Status below the bubble row — outside the avatar alignment */}
      {hasStatusBelow && (
        <div className={`mt-0.5 ${isCustomer ? 'pl-9' : 'pr-9'}`}>
          {hasEmailBadge && (
            <div className={`mb-0.5 flex ${isCustomer ? '' : 'justify-end'}`}>
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
          )}

          {/* Read receipt indicator */}
          {receiptStatus && (
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
              ) : (
                <>
                  <TickDouble01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="text-[11px] text-muted-foreground">Delivered</span>
                </>
              )}
            </div>
          )}

          {/* AI metadata: confidence badge + collapsible sources */}
          {aiMeta && (
            <div className={`mt-0.5 ${isCustomer ? '' : 'text-right'}`}>
              <div className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground">
                <span className="rounded-full bg-primary/10 px-1.5 py-0.5 font-medium text-primary">
                  {(aiMeta.ai_confidence * 100).toFixed(0)}% confident
                </span>
                {aiMeta.ai_sources?.length > 0 && (
                  <button
                    onClick={() => setSourcesOpen(!sourcesOpen)}
                    className="inline-flex items-center gap-0.5 rounded px-1 py-0.5 hover:bg-muted"
                  >
                    <File01Icon className="h-3 w-3" />
                    {aiMeta.ai_sources.length} source{aiMeta.ai_sources.length > 1 ? 's' : ''}
                    {sourcesOpen ? <ArrowUp01Icon className="h-3 w-3" /> : <ArrowDown01Icon className="h-3 w-3" />}
                  </button>
                )}
              </div>
              {sourcesOpen && aiMeta.ai_sources?.length > 0 && (
                <div className="mt-1.5 space-y-1 rounded-lg border bg-muted/50 p-2 text-left text-xs">
                  {aiMeta.ai_sources.map((src) => (
                    <div key={src.docId} className="flex items-start gap-1.5">
                      <File01Icon className="mt-0.5 h-3 w-3 shrink-0 text-muted-foreground" />
                      <span className="font-medium">{src.title}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
});
