import { memo } from 'react';
import { Bot, StickyNote, User } from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import type { SupportMessage, TicketSource } from '@/lib/pmTypes';
import { formatTimestamp, getInitial } from './helpers';

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

interface MessageBubbleProps {
  message: SupportMessage;
  isConsecutive?: boolean;
  isLastInGroup?: boolean;
  source?: TicketSource;
}

export const MessageBubble = memo(function MessageBubble({ message, isConsecutive, isLastInGroup = true, source }: MessageBubbleProps) {
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

  // ── Internal note: right-aligned card with amber accent ──
  if (isInternal) {
    return (
      <div className={`flex justify-end ${isConsecutive ? 'mt-1' : 'mt-5'}`}>
        <div className="max-w-[75%]">
          {!isConsecutive && (
            <div className="mb-1 pr-1 text-right">
              <span className="text-[11px] font-medium text-muted-foreground">{senderName}</span>
            </div>
          )}
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
                <p className="whitespace-pre-wrap text-sm leading-relaxed text-amber-900 dark:text-amber-200">{message.content}</p>
              </div>
            </TooltipTrigger>
            <TooltipContent side="left">{tooltipContent}</TooltipContent>
          </Tooltip>
        </div>
      </div>
    );
  }

  // ── Chat bubble ──
  const avatarEl = isCustomer ? (
    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-semibold text-muted-foreground">
      {getInitial(senderName)}
    </div>
  ) : (
    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
      {isAgent ? <Bot className="h-3.5 w-3.5" /> : <User className="h-3.5 w-3.5" />}
    </div>
  );

  return (
    <div className={`flex ${isCustomer ? 'justify-start' : 'justify-end'} ${isConsecutive ? 'mt-0.5' : 'mt-5'}`}>
      {/* Left side: avatar or spacer (customer messages) */}
      {isCustomer && (
        <div className="mr-2 flex w-7 shrink-0 flex-col justify-end">
          {showAvatar && avatarEl}
        </div>
      )}

      <div className="max-w-[70%]">
        {/* Sender name — only on first message in a group */}
        {!isConsecutive && (
          <div className={`mb-1 ${isCustomer ? 'pl-1' : 'pr-1 text-right'}`}>
            <span className="text-[11px] font-medium text-muted-foreground">{senderName}</span>
          </div>
        )}

        {/* Bubble with hover tooltip */}
        <Tooltip>
          <TooltipTrigger asChild>
            <div
              className={`rounded-2xl px-3.5 py-2 text-sm leading-relaxed ${
                isCustomer
                  ? `bg-muted text-foreground ${isLastInGroup ? 'rounded-bl-sm' : ''}`
                  : `bg-primary text-primary-foreground ${isLastInGroup ? 'rounded-br-sm' : ''}`
              }`}
            >
              <p className="whitespace-pre-wrap">{message.content}</p>
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
  );
});
