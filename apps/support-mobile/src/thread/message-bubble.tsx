import type { ReactNode } from 'react'
import { motion } from 'motion/react'
import { Bot, CheckCheck, Mail, Paperclip, StickyNote, User, Zap } from 'lucide-react'
import type { SupportMessage } from '@helpin-ai/support-core'
import {
  getSupportSystemEventBadge,
  getSupportSystemEventText,
  toSupportSystemEventSegments,
  type SystemEventBadge,
} from '@/components/support/supportSystemEvent'
import { cn } from '@mobile/lib/cn'
import { riseIn } from '@mobile/lib/motion'
import { EmailBody } from './email-body'
import { Markdown } from './markdown'
import { splitMentionSegments, type SupportReceiptStatus } from './thread-helpers'

export interface MessageBubbleProps {
  message: SupportMessage
  /** 'left' for the customer's side, 'right' for our side (user/agent/ai). Ignored for internal notes and system events, which always render full-width. */
  align: 'left' | 'right'
  /** Read-receipt state for the last outbound reply; only passed to that one message. */
  receiptStatus?: SupportReceiptStatus | null
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
  // Routing events render a labelled pill; escalation events show just the icon
  // (the narration text already spells out the escalation).
  if (badge.kind === 'routing_rule' || badge.kind === 'ai_routing') {
    return (
      <span
        className={cn(
          'inline-flex shrink-0 items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] font-medium',
          badge.kind === 'routing_rule'
            ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
            : 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
        )}
      >
        {icon}
        {badge.label}
      </span>
    )
  }
  return <span className="shrink-0 text-muted-foreground">{icon}</span>
}

function receiptMeta(status: SupportReceiptStatus): { label: string; read: boolean } {
  switch (status) {
    case 'read':
      return { label: 'Read', read: true }
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
              ? 'border-amber-300/70 bg-amber-100/40 text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-100'
              : 'border-border/60 text-foreground',
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
  if (attachments.length === 0) return null
  return (
    <div className="mt-2 flex flex-col gap-1.5">
      {attachments.map((attachment) => (
        <button
          key={attachment.id}
          type="button"
          onClick={() => openAttachment(attachment.url)}
          aria-label={`Open ${attachment.file_name}`}
          className="block overflow-hidden rounded-xl"
        >
          <img src={attachment.url} alt={attachment.file_name} className="max-h-48 w-full rounded-xl object-cover" />
        </button>
      ))}
    </div>
  )
}

/**
 * A single chat bubble. Customer messages align left on a muted surface;
 * our-side messages (user/agent/ai) align right on a primary-tinted surface,
 * with message bodies rendered as Markdown (mirrors web). Internal notes and
 * system events ignore `align` and render their own full-width treatments.
 */
export function MessageBubble({ message, align, receiptStatus }: MessageBubbleProps) {
  const imageAttachments = message.attachments?.filter((a) => a.file_type.startsWith('image/')) ?? []
  const fileAttachments = message.attachments?.filter((a) => !a.file_type.startsWith('image/')) ?? []
  const isEmail = message.via_channel === 'email' && !!message.html_body
  const isCustomer = message.sender_type === 'customer'
  const senderName = messageSenderName(message)

  let content: ReactNode

  // System events (assigned, resolved, escalated, routed, ...) are narration,
  // not a chat turn — a centered pill with humanized text + an optional badge.
  if (message.message_type === 'system') {
    const text = getSupportSystemEventText(message.system_event_type, message.content, senderName)
    const segments = toSupportSystemEventSegments(message.system_event_type, text)
    const badge = getSupportSystemEventBadge(message.system_event_type, message.content)
    content = (
      <div className="flex justify-center py-1">
        <span className="selectable inline-flex max-w-[90%] items-center gap-1.5 rounded-full bg-muted px-3 py-1 text-footnote text-muted-foreground">
          {badge && <SystemEventBadgeChip badge={badge} />}
          <span className="min-w-0">
            {segments.map((segment, index) =>
              segment.bold ? (
                <strong key={index} className="font-medium text-foreground">{segment.text}</strong>
              ) : (
                <span key={index}>{segment.text}</span>
              ),
            )}
          </span>
        </span>
      </div>
    )
  } else if (message.is_internal) {
    // Internal note: amber card, "{name} left a private note", @mentions highlighted.
    const mentionSegments = splitMentionSegments(message.content)
    content = (
      <div className="w-full rounded-2xl border border-amber-300/60 bg-amber-100/50 px-3 py-2 dark:border-amber-800/60 dark:bg-amber-950/30">
        <div className="mb-1 flex items-center gap-1.5 text-caption text-amber-700 dark:text-amber-300">
          <StickyNote className="h-3 w-3 shrink-0" />
          <span>
            <span className="font-semibold">{senderName}</span> left a private note
          </span>
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
    )
  } else {
    const receipt = receiptStatus ? receiptMeta(receiptStatus) : null
    const showEmailReceived = message.via_channel === 'email' && isCustomer
    content = (
      <div className={cn('flex flex-col', align === 'right' ? 'items-end' : 'items-start')}>
        <div
          className={cn(
            'max-w-[85%] min-w-0 rounded-[16px] px-3 py-2',
            align === 'right' ? 'bg-primary/10' : 'bg-muted',
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
        {(showEmailReceived || receipt) && (
          <div
            className={cn(
              'mt-0.5 flex items-center gap-1 px-1 text-[11px] text-muted-foreground',
              align === 'right' ? 'justify-end' : 'justify-start',
            )}
          >
            {showEmailReceived && (
              <>
                <Mail className="h-3 w-3 shrink-0" />
                <span>Received by email</span>
              </>
            )}
            {receipt && (
              <>
                <CheckCheck className={cn('h-3.5 w-3.5 shrink-0', receipt.read && 'text-blue-500')} />
                <span>{receipt.label}</span>
              </>
            )}
          </div>
        )}
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
