import type { ReactNode } from 'react'
import { motion } from 'motion/react'
import { Paperclip } from 'lucide-react'
import type { SupportMessage } from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { riseIn } from '@mobile/lib/motion'
import { EmailBody } from './email-body'

export interface MessageBubbleProps {
  message: SupportMessage
  /** 'left' for the customer's side, 'right' for our side (user/agent/ai). Ignored for internal notes and system events, which always render full-width. */
  align: 'left' | 'right'
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function openAttachment(url: string): void {
  window.open(url, '_blank', 'noopener,noreferrer')
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
 * A single chat bubble: customer messages align left on a muted surface,
 * our-side messages (user/agent/ai) align right on a primary-tinted surface.
 * Internal notes and system events ignore `align` and render their own
 * full-width treatments.
 */
export function MessageBubble({ message, align }: MessageBubbleProps) {
  const imageAttachments = message.attachments?.filter((a) => a.file_type.startsWith('image/')) ?? []
  const fileAttachments = message.attachments?.filter((a) => !a.file_type.startsWith('image/')) ?? []
  const isEmail = message.via_channel === 'email' && !!message.html_body

  let content: ReactNode

  // System events (assigned, resolved, mailbox moved, ...) are narration, not
  // a chat turn — render as a centered pill instead of a directional bubble.
  if (message.message_type === 'system') {
    content = (
      <div className="flex justify-center py-1">
        <span className="selectable max-w-[85%] rounded-full bg-muted px-3 py-1 text-center text-footnote text-muted-foreground">
          {message.content}
        </span>
      </div>
    )
  } else if (message.is_internal) {
    content = (
      <div className="w-full rounded-2xl border border-amber-300/60 bg-amber-100/50 px-3 py-2 dark:border-amber-800/60 dark:bg-amber-950/30">
        <div className="mb-1 text-caption font-semibold text-amber-700 dark:text-amber-300">Internal note</div>
        {isEmail ? (
          <EmailBody html={message.html_body!} className="text-amber-900 dark:text-amber-100" />
        ) : (
          <p className="selectable whitespace-pre-wrap text-body text-amber-900 dark:text-amber-100">{message.content}</p>
        )}
        <ImageThumbnails attachments={imageAttachments} />
        <AttachmentRows attachments={fileAttachments} tone="note" />
      </div>
    )
  } else {
    content = (
      <div className={cn('flex', align === 'right' ? 'justify-end' : 'justify-start')}>
        <div
          className={cn(
            'max-w-[78%] min-w-0 rounded-[16px] px-3 py-2',
            align === 'right' ? 'bg-primary/10' : 'bg-muted',
          )}
        >
          {isEmail ? (
            <EmailBody html={message.html_body!} />
          ) : (
            <p className="selectable whitespace-pre-wrap text-body">{message.content}</p>
          )}
          <ImageThumbnails attachments={imageAttachments} />
          <AttachmentRows attachments={fileAttachments} tone="default" />
        </div>
      </div>
    )
  }

  // Task 14: a message optimistically appended by `useSendMessage` (Task 10)
  // carries `pending: true` while the request is in flight. Render it at 70%
  // opacity with a gentle rise-in so an own-send visibly registers before the
  // network confirms — settled messages (the vast majority of renders) skip
  // this wrapper entirely so historical thread loads don't replay an
  // entrance animation for every bubble.
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
