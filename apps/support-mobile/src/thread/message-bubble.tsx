import { useState, type ReactNode } from 'react'
import { motion } from 'motion/react'
import { Bot, CheckCheck, CheckCircle2, Mail, MoreHorizontal, Paperclip, StickyNote, User, XCircle, Zap } from 'lucide-react'
import type { SupportMessage } from '@helpin-ai/support-core'
import {
  getSupportSystemEventBadge,
  getSupportSystemEventText,
  toSupportSystemEventSegments,
  type SystemEventBadge,
} from '@/components/support/supportSystemEvent'
import { cn } from '@mobile/lib/cn'
import { riseIn } from '@mobile/lib/motion'
import { Avatar } from '@mobile/ui/avatar'
import { getAvatarColor } from '@/components/support/helpers'
import { EmailBody } from './email-body'
import { Markdown } from './markdown'
import { ImageViewer } from './image-viewer'
import { splitMentionSegments, type SupportReceiptStatus } from './thread-helpers'

export interface MessageBubbleProps {
  message: SupportMessage
  /** 'left' for the customer's side, 'right' for our side (user/agent/ai). Ignored for internal notes and system events, which always render full-width. */
  align: 'left' | 'right'
  /** Read-receipt state for the last outbound reply; only passed to that one message. */
  receiptStatus?: SupportReceiptStatus | null
  /** Opens the action sheet from the compact control beside the message time. */
  onMessageActions?: () => void
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function openAttachment(url: string): void {
  window.open(url, '_blank', 'noopener,noreferrer')
}

/** Display name for a message's sender, for note/system narration. */
function messageSenderName(message: SupportMessage): string {
  if (message.sender_display_name) return message.sender_display_name
  switch (message.sender_type) {
    case 'customer':
      return 'Customer'
    case 'ai':
      return 'AI Assistant'
    default:
      return 'Agent'
  }
}

function SystemEventBadgeChip({ badge }: { badge: SystemEventBadge }) {
  const icon =
    badge.kind === 'routing_rule' ? (
      <Zap className="h-3 w-3" />
    ) : badge.kind === 'customer_requested_human' ? (
      <User className="h-3 w-3" />
    ) : (
      <Bot className="h-3 w-3" />
    )
  return (
    <span
      aria-label={badge.label}
      title={badge.label}
      className={cn(
        'inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full',
        badge.kind === 'routing_rule'
          ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300'
          : badge.kind === 'ai_routing'
            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
            : 'bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300',
      )}
    >
      {icon}
    </span>
  )
}

function SystemEventBubble({ message, senderName }: { message: SupportMessage; senderName: string }) {
  const eventType = message.system_event_type
  const text = getSupportSystemEventText(eventType, message.content, senderName)
  const segments = toSupportSystemEventSegments(eventType, text, { extended: true })
  const badge = getSupportSystemEventBadge(eventType, message.content)
  const isTeammateJoined = eventType === 'teammate_joined'
  const isEscalation = eventType === 'ai_escalated' || eventType === 'customer_requested_human'
  const isResolved = eventType === 'resolved'
  const isClosed = eventType === 'closed'
  const time = formatMessageTime(message.created_at)
  const avatarSeed = message.sender_user_id || message.sender_agent_id || senderName

  const leading = isResolved ? (
    <>
      <CheckCircle2 aria-label="Resolved" className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-300" />
      <Avatar
        name={senderName}
        src={message.sender_avatar_url}
        size={20}
        className={getAvatarColor(avatarSeed)}
      />
    </>
  ) : isClosed ? (
    <XCircle aria-label="Closed" className="h-4 w-4 shrink-0" />
  ) : badge ? (
    <SystemEventBadgeChip badge={badge} />
  ) : (
    <Avatar
      name={senderName}
      src={message.sender_avatar_url}
      size={20}
      className={getAvatarColor(avatarSeed)}
    />
  )

  return (
    <div
      data-testid="system-event"
      data-system-event-type={eventType}
      className="flex flex-col items-center py-1"
    >
      <div
        data-testid="system-event-surface"
        aria-label={[text, time].filter(Boolean).join(', ')}
        className={cn(
          'inline-flex max-w-[92%] items-center gap-2 text-[12px] leading-snug [overflow-wrap:anywhere]',
          isTeammateJoined
            ? 'px-1 py-1 text-muted-foreground'
            : cn(
                'rounded-2xl border px-3 py-2',
                isResolved
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200'
                  : isClosed
                    ? 'border-slate-700 bg-slate-700 text-white dark:border-slate-600 dark:bg-slate-800'
                    : isEscalation
                      ? 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200'
                      : 'border-border/60 bg-muted/60 text-muted-foreground',
              ),
        )}
      >
        {leading}
        <span className="min-w-0 max-w-full">
          {segments.map((segment, index) =>
            segment.bold ? (
              <strong key={index} className="font-semibold text-current">{segment.text}</strong>
            ) : (
              <span key={index}>{segment.text}</span>
            ),
          )}
        </span>
      </div>
      {time && (
        <span data-testid="system-event-time" className="mt-1 text-[10px] text-muted-foreground/65">
          {time}
        </span>
      )}
    </div>
  )
}

function receiptMeta(status: SupportReceiptStatus): { label: string; read: boolean } {
  switch (status) {
    case 'read':
      return { label: 'Read in chat', read: true }
    case 'read_email':
      return { label: 'Read via email', read: true }
    case 'delivered_email':
      return { label: 'Delivered via email', read: false }
    case 'sent_email':
      return { label: 'Sent via email', read: false }
    case 'delivered':
      return { label: 'Delivered', read: false }
  }
}

export function formatMessageTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' }).format(date)
}

function AttachmentRows({ attachments, tone }: { attachments: NonNullable<SupportMessage['attachments']>; tone: 'default' | 'note' }) {
  if (attachments.length === 0) return null
  return (
    <div className="mt-2 space-y-1.5">
      {attachments.map((attachment) => (
        <button
          key={attachment.id}
          type="button"
          onClick={() => openAttachment(attachment.url)}
          className={cn(
            'flex w-full min-w-0 items-center gap-2 rounded-lg border px-2.5 py-1.5 text-left text-footnote',
            tone === 'note'
              ? 'border-amber-200/80 bg-background/70 text-amber-900 dark:border-amber-900/60 dark:bg-background/70 dark:text-amber-100'
              : 'border-border/60 bg-background/80 text-foreground',
          )}
        >
          <Paperclip className="h-3 w-3 shrink-0 opacity-60" />
          <span className="min-w-0 flex-1 truncate font-medium">{attachment.file_name}</span>
          <span className="shrink-0 opacity-60">{formatFileSize(attachment.file_size)}</span>
        </button>
      ))}
    </div>
  )
}

function ImageThumbnails({ attachments }: { attachments: NonNullable<SupportMessage['attachments']> }) {
  const [viewerIndex, setViewerIndex] = useState<number | null>(null)
  if (attachments.length === 0) return null
  return (
    <>
      <div className="mt-2 flex flex-col gap-1.5">
        {attachments.map((attachment, index) => (
          <button
            key={attachment.id}
            type="button"
            onClick={() => setViewerIndex(index)}
            aria-label={`Preview ${attachment.file_name}`}
            className="block overflow-hidden rounded-xl"
          >
            <img src={attachment.url} alt={attachment.file_name} className="max-h-48 w-full rounded-xl object-cover" />
          </button>
        ))}
      </div>
      {viewerIndex !== null && (
        <ImageViewer
          images={attachments}
          initialIndex={viewerIndex}
          open
          onOpenChange={(nextOpen) => {
            if (!nextOpen) setViewerIndex(null)
          }}
        />
      )}
    </>
  )
}

/**
 * A single chat bubble. Customer messages align left on a muted surface;
 * our-side messages (user/agent/ai) align right on a primary-tinted surface,
 * with message bodies rendered as Markdown (mirrors web). Internal notes and
 * system events ignore `align` and render their own distinct treatments.
 */
export function MessageBubble({ message, align, receiptStatus, onMessageActions }: MessageBubbleProps) {
  const imageAttachments = message.attachments?.filter((a) => a.file_type.startsWith('image/')) ?? []
  const fileAttachments = message.attachments?.filter((a) => !a.file_type.startsWith('image/')) ?? []
  const isEmail = message.via_channel === 'email' && !!message.html_body
  const isCustomer = message.sender_type === 'customer'
  const senderName = messageSenderName(message)

  let content: ReactNode

  // System events (assigned, resolved, escalated, routed, ...) are narration,
  // not a chat turn — a centered pill with humanized text + an optional badge.
  if (message.message_type === 'system') {
    content = <SystemEventBubble message={message} senderName={senderName} />
  } else if (message.is_internal) {
    // Internal note: right-aligned amber card with web's accent edge and highlighted @mentions.
    const mentionSegments = splitMentionSegments(message.content)
    content = (
      <div className="flex w-full flex-col items-end">
        <div
          data-testid="message-bubble"
          className="w-full max-w-[92%] rounded-xl border-r-[3px] border-r-amber-400 bg-amber-50 px-3.5 py-2.5 dark:border-r-amber-500 dark:bg-amber-950/20"
        >
          <div className="mb-1.5 flex items-center gap-1.5 text-caption uppercase text-amber-700 dark:text-amber-300">
            <StickyNote className="h-3 w-3 shrink-0" />
            <span className="font-semibold">Note · {senderName}</span>
          </div>
          {isEmail ? (
            <EmailBody html={message.html_body!} className="text-amber-900 dark:text-amber-100" />
          ) : (
            <p className="selectable whitespace-pre-wrap text-body text-amber-900 dark:text-amber-100">
              {mentionSegments.map((segment, index) =>
                segment.mention ? (
                  <span key={index} className="mention-highlight">{segment.text}</span>
                ) : (
                  <span key={index}>{segment.text}</span>
                ),
              )}
            </p>
          )}
          <ImageThumbnails attachments={imageAttachments} />
          <AttachmentRows attachments={fileAttachments} tone="note" />
        </div>
        <div data-testid="message-meta" className="mt-0.5 flex items-center justify-end gap-0.5 px-1 text-[11px] text-amber-700/70 dark:text-amber-300/70">
          <span>{formatMessageTime(message.created_at)}</span>
          {onMessageActions && (
            <button
              type="button"
              aria-label="Message actions"
              onClick={onMessageActions}
              className="-my-1.5 flex h-8 w-8 items-center justify-center rounded-full text-current active:bg-amber-100 dark:active:bg-amber-900/40"
            >
              <MoreHorizontal className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>
    )
  } else {
    const receipt = receiptStatus ? receiptMeta(receiptStatus) : null
    const showEmailReceived = message.via_channel === 'email' && isCustomer
    content = (
      <div className={cn('flex flex-col', align === 'right' ? 'items-end' : 'items-start')}>
        <div
          data-testid="message-bubble"
          className={cn(
            'max-w-[92%] min-w-0 rounded-2xl border border-border/40 px-3.5 py-2 text-foreground/85 dark:text-foreground',
            align === 'right'
              ? 'rounded-br-sm bg-blue-50 dark:bg-blue-950/40'
              : 'rounded-bl-sm bg-muted',
          )}
        >
          {isEmail ? (
            <EmailBody html={message.html_body!} />
          ) : (
            <Markdown className="text-body">{message.content}</Markdown>
          )}
          <ImageThumbnails attachments={imageAttachments} />
          <AttachmentRows attachments={fileAttachments} tone="default" />
        </div>
        <div
          data-testid="message-meta"
          className={cn(
            'mt-0.5 flex items-center gap-1 px-1 text-[11px] text-muted-foreground/80',
            align === 'right' ? 'justify-end' : 'justify-start',
          )}
        >
          <span>{formatMessageTime(message.created_at)}</span>
          {onMessageActions && (
            <button
              type="button"
              aria-label="Message actions"
              onClick={onMessageActions}
              className="-my-1.5 flex h-8 w-8 items-center justify-center rounded-full text-current active:bg-muted"
            >
              <MoreHorizontal className="h-4 w-4" />
            </button>
          )}
          {showEmailReceived && (
            <>
              <span>·</span>
              <Mail className="h-3 w-3 shrink-0" />
              <span>Received via email</span>
            </>
          )}
          {receipt && (
            <>
              <span>·</span>
              <CheckCheck className={cn('h-3.5 w-3.5 shrink-0', receipt.read && 'text-blue-500')} />
              <span>{receipt.label}</span>
            </>
          )}
        </div>
      </div>
    )
  }

  // A message optimistically appended by `useSendMessage` carries `pending`
  // while in flight — render at 70% opacity with a gentle rise-in so an
  // own-send visibly registers before the network confirms.
  if (!message.pending) return content

  return (
    <motion.div
      initial={{ opacity: 0, y: riseIn.initial.y }}
      animate={{ opacity: 0.7, y: 0 }}
      transition={riseIn.transition}
    >
      {content}
    </motion.div>
  )
}
