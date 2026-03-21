import { memo, useMemo, useState, type ReactNode } from 'react';
import Markdown from 'react-markdown';
import { Bot, CheckCheck, CheckCircle2, ChevronDown, ChevronUp, FileText, RotateCcw, StickyNote, XCircle } from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import type { AIMessageMetadata, SupportMessage, TicketSource } from '@/lib/pmTypes';
import { formatTimestamp, getInitial, getAvatarColor } from './helpers';

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

const SOURCE_LABELS: Record<string, string> = {
  widget: 'Chat Widget',
  email: 'Email',
  internal: 'Internal',
  api: 'API',
};

const SENDER_TYPE_LABELS: Record<string, string> = {
  customer: 'Customer',
  user: 'Agent',
  agent: 'AI Agent',
};

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
  receiptStatus?: 'delivered' | 'read' | null;
}

export const MessageBubble = memo(function MessageBubble({ message, isConsecutive, isLastInGroup = true, source, receiptStatus }: MessageBubbleProps) {
  const currentUser = useAuthStore((s) => s.user);
  const isCustomer = message.sender_type === 'customer';
  const isAgent = message.sender_type === 'agent';
  const isInternal = message.is_internal;
  const senderName = message.sender_display_name
    ?? (isCustomer ? 'Customer' : isAgent ? 'Agent' : currentUser?.full_name ?? 'You');
  const showAvatar = isLastInGroup;
  const fullTimestamp = formatTimestamp(message.created_at);
  const senderLabel = SENDER_TYPE_LABELS[message.sender_type] ?? message.sender_type;
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

  // Parse AI metadata if present
  const aiMeta = useMemo<AIMessageMetadata | null>(() => {
    if (!message.metadata) return null;
    try {
      const meta = JSON.parse(message.metadata);
      return meta.ai_auto_reply ? meta : null;
    } catch { return null; }
  }, [message.metadata]);

  // Highlight @mentions in internal notes
  const mentionParts = useMemo(() => {
    if (!isInternal) return null;
    return renderMentionHighlights(displayContent);
  }, [displayContent, isInternal]);

  const [sourcesOpen, setSourcesOpen] = useState(false);

  const tooltipContent = (
    <div className="space-y-0.5 text-xs">
      <div className="font-medium">{senderName}</div>
      <div className="text-muted-foreground">{fullTimestamp}</div>
      <div className="text-muted-foreground">
        {senderLabel}
        {sourceLabel && ` · via ${sourceLabel}`}
      </div>
    </div>
  );

  // ── System message: right-aligned pill with avatar (Crisp-style) ──
  if (message.message_type === 'system') {
    const isResolved = message.content.toLowerCase().includes('resolved');
    const isReopened = message.content.toLowerCase().includes('reopened');
    const isClosed = message.content.toLowerCase().includes('closed');

    const icon = isResolved ? <CheckCircle2 className="h-4 w-4 shrink-0" />
      : isReopened ? <RotateCcw className="h-3.5 w-3.5 shrink-0" />
      : isClosed ? <XCircle className="h-4 w-4 shrink-0" />
      : <CheckCircle2 className="h-4 w-4 shrink-0" />;

    const avatarUrl = message.sender_avatar_url;

    return (
      <div className="my-4 flex items-center justify-end gap-2 animate-in fade-in slide-in-from-right-2 duration-300">
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="flex items-center gap-2.5 rounded-full bg-slate-700 px-4 py-2 text-white shadow-sm" style={{ border: 'none' }}>
              {icon}
              <span className="text-sm font-medium">{message.content}</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="left">
            <div className="space-y-0.5 text-xs">
              <div className="font-medium">{senderName}</div>
              <div className="text-muted-foreground">{fullTimestamp}</div>
            </div>
          </TooltipContent>
        </Tooltip>
        {avatarUrl ? (
          <img src={avatarUrl} alt={senderName} className="h-7 w-7 rounded-full object-cover shadow-sm" />
        ) : (
          <div className={`flex h-7 w-7 items-center justify-center rounded-full text-[11px] font-semibold shadow-sm ${getAvatarColor(message.sender_user_id || senderName)}`}>
            {getInitial(senderName)}
          </div>
        )}
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
              <div className="rounded-lg border-r-[3px] border-r-amber-400 bg-amber-50 px-4 py-2.5 dark:bg-amber-950/20">
                <div className="mb-1.5 flex items-center gap-1.5">
                  <StickyNote className="h-3 w-3 text-amber-500 dark:text-amber-400" />
                  <span className="text-[11px] text-amber-600 dark:text-amber-400">
                    <span className="font-semibold">{senderName}</span>
                    <span className="font-normal"> left a private note</span>
                  </span>
                </div>
                <div className="prose-chat text-sm leading-relaxed text-amber-900 dark:text-amber-200">
                  {mentionParts ? (
                    <p className="whitespace-pre-wrap">{mentionParts}</p>
                  ) : (
                    <Markdown components={{ a: ({ href, children }) => <a href={href} target="_blank" rel="noopener noreferrer">{children}</a> }}>{displayContent}</Markdown>
                  )}
                </div>
              </div>
            </TooltipTrigger>
            <TooltipContent side="left">{tooltipContent}</TooltipContent>
          </Tooltip>
        </div>
      </div>
    );
  }

  // ── Chat bubble ──
  const avatarUrl = message.sender_avatar_url;

  const avatarEl = isCustomer ? (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[11px] font-semibold shadow-sm ${getAvatarColor(message.sender_user_id || senderName)}`}>
          {getInitial(senderName)}
        </div>
      </TooltipTrigger>
      <TooltipContent side="left"><span className="text-xs font-medium">{senderName}</span></TooltipContent>
    </Tooltip>
  ) : avatarUrl ? (
    <Tooltip>
      <TooltipTrigger asChild>
        <img src={avatarUrl} alt={senderName} className="h-7 w-7 shrink-0 rounded-full object-cover shadow-sm" />
      </TooltipTrigger>
      <TooltipContent side="right"><span className="text-xs font-medium">{senderName}</span></TooltipContent>
    </Tooltip>
  ) : isAgent ? (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary shadow-sm">
          <Bot className="h-3.5 w-3.5" />
        </div>
      </TooltipTrigger>
      <TooltipContent side="right"><span className="text-xs font-medium">{senderName}</span></TooltipContent>
    </Tooltip>
  ) : (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[11px] font-semibold shadow-sm ${getAvatarColor(message.sender_user_id || senderName)}`}>
          {getInitial(senderName)}
        </div>
      </TooltipTrigger>
      <TooltipContent side="right"><span className="text-xs font-medium">{senderName}</span></TooltipContent>
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

        <div className="max-w-[70%]">
          <Tooltip>
            <TooltipTrigger asChild>
              <div
                className={`rounded-2xl px-3.5 py-2 text-sm leading-relaxed ${
                  isCustomer
                    ? `bg-muted text-foreground ${isLastInGroup ? 'rounded-bl-sm' : ''}`
                    : `bg-blue-600 text-white dark:bg-blue-500 ${isLastInGroup ? 'rounded-br-sm' : ''}`
                }`}
              >
                <div className="prose-chat">
                  <Markdown components={{ a: ({ href, children }) => <a href={href} target="_blank" rel="noopener noreferrer">{children}</a> }}>{displayContent}</Markdown>
                </div>
              </div>
            </TooltipTrigger>
            <TooltipContent side={isCustomer ? 'right' : 'left'}>
              {tooltipContent}
            </TooltipContent>
          </Tooltip>
        </div>

        {/* Right side: avatar or spacer (agent/user messages) */}
        {!isCustomer && (
          <div className="ml-2 flex w-7 shrink-0 flex-col justify-end">
            {showAvatar && avatarEl}
          </div>
        )}
      </div>

      {/* Status below the bubble row — outside the avatar alignment */}
      {hasStatusBelow && (
        <div className={`mt-0.5 ${isCustomer ? 'pl-9' : 'pr-9'}`}>
          {hasEmailBadge && (
            <div className={`mb-0.5 flex ${isCustomer ? '' : 'justify-end'}`}>
              <span className="inline-flex items-center rounded-full bg-blue-500/10 px-2 py-0.5 text-[11px] font-medium text-blue-600 dark:text-blue-300">
                Via email
              </span>
            </div>
          )}

          {/* Read receipt indicator */}
          {receiptStatus && (
            <div className={`flex items-center gap-1 ${isCustomer ? '' : 'justify-end'}`}>
              {receiptStatus === 'read' ? (
                <>
                  <CheckCheck className="h-3.5 w-3.5 text-blue-500" />
                  <span className="text-[11px] text-muted-foreground">Read in chat</span>
                </>
              ) : (
                <>
                  <CheckCheck className="h-3.5 w-3.5 text-muted-foreground" />
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
                    <FileText className="h-3 w-3" />
                    {aiMeta.ai_sources.length} source{aiMeta.ai_sources.length > 1 ? 's' : ''}
                    {sourcesOpen ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
                  </button>
                )}
              </div>
              {sourcesOpen && aiMeta.ai_sources?.length > 0 && (
                <div className="mt-1.5 space-y-1 rounded-lg border bg-muted/50 p-2 text-left text-xs">
                  {aiMeta.ai_sources.map((src) => (
                    <div key={src.docId} className="flex items-start gap-1.5">
                      <FileText className="mt-0.5 h-3 w-3 shrink-0 text-muted-foreground" />
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
