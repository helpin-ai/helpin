import { Mail, MessageCircle } from 'lucide-react'
import { motion } from 'motion/react'
import type { ComponentType } from 'react'
import type { SupportConversation } from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { Avatar } from '@mobile/ui/avatar'
import { Badge, type BadgeTone } from '@mobile/ui/badge'
import { CONVERSATION_CELL_HEIGHT, formatRelativeTime, isUnread, previewText } from './inbox-helpers'

const CHANNEL_ICONS: Partial<Record<SupportConversation['source'], ComponentType<{ className?: string }>>> = {
  email: Mail,
  widget: MessageCircle,
}

const STATUS_BADGES: Partial<Record<SupportConversation['status'], { label: string; tone: BadgeTone }>> = {
  waiting_on_customer: { label: 'Waiting', tone: 'warning' },
  resolved: { label: 'Resolved', tone: 'success' },
  spam: { label: 'Spam', tone: 'neutral' },
}

function displayNameFor(conversation: SupportConversation): string {
  if (conversation.customer_name) return conversation.customer_name
  if (conversation.customer_email) return conversation.customer_email
  if (conversation.anonymous_id) return `Visitor #${conversation.anonymous_id.slice(0, 6)}`
  return 'Anonymous'
}

/**
 * How long the `isExiting` fade takes. The inbox screen waits exactly this
 * long after a Resolve commit before removing the row from the rendered
 * list, so keep the two in lockstep via this constant.
 */
export const CELL_EXIT_DURATION_MS = 200

export interface ConversationCellProps {
  conversation: SupportConversation
  onPress: () => void
  /**
   * True for the brief window after a "Resolve" swipe commits — fades and
   * shrinks the row in place so it doesn't just pop out from under the
   * finger. The inbox screen removes the row from the rendered list once the
   * fade completes (CELL_EXIT_DURATION_MS).
   */
  isExiting?: boolean
}

/**
 * Fixed 84px row — the virtualizer's `estimateSize` (inbox-screen.tsx) depends
 * on this exact height, so the outer element must never grow or shrink based
 * on content (hence `overflow-hidden` + 2-line preview clamp).
 *
 * Read vs. unread share the identical layout; only the leading dot's opacity
 * and the name/preview color/weight change, so nothing shifts when a
 * conversation is marked read.
 */
export function ConversationCell({ conversation, onPress, isExiting }: ConversationCellProps) {
  const unread = isUnread(conversation)
  const displayName = displayNameFor(conversation)
  const preview = previewText(conversation)
  const time = formatRelativeTime(conversation.updated_at)
  const ChannelIcon = CHANNEL_ICONS[conversation.source]
  const statusBadge = conversation.status !== 'open' ? STATUS_BADGES[conversation.status] : undefined

  return (
    <motion.div
      role="button"
      tabIndex={0}
      data-testid="conversation-cell"
      onClick={onPress}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onPress()
        }
      }}
      animate={isExiting ? { opacity: 0, scale: 0.96 } : { opacity: 1, scale: 1 }}
      transition={{ duration: CELL_EXIT_DURATION_MS / 1000 }}
      style={{ height: CONVERSATION_CELL_HEIGHT }}
      className="box-border flex w-full cursor-pointer items-center gap-3 overflow-hidden border-b border-border/60 bg-background px-4 text-left active:bg-muted/50"
    >
      {/* Fixed 16px leading gutter — width never changes; only the dot's opacity does. */}
      <div className="flex h-4 w-4 shrink-0 items-center justify-center">
        <span
          data-testid="unread-dot"
          className={cn(
            'h-2 w-2 rounded-full bg-[var(--unread-dot)] transition-opacity',
            unread ? 'opacity-100' : 'opacity-0',
          )}
        />
      </div>

      <div className="relative shrink-0">
        <Avatar name={displayName} size={44} />
        {ChannelIcon && (
          <span className="absolute -bottom-0.5 -right-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-background text-muted-foreground ring-2 ring-background">
            <ChannelIcon className="h-2.5 w-2.5" />
          </span>
        )}
      </div>

      <div className="flex min-w-0 flex-1 flex-col justify-center gap-0.5">
        <div className="flex items-center justify-between gap-2">
          <span
            data-testid="conversation-name"
            className="min-w-0 flex-1 truncate text-body font-semibold text-foreground"
          >
            {displayName}
          </span>
          <span data-testid="conversation-time" className="shrink-0 text-footnote tnum text-muted-foreground">
            {time}
          </span>
        </div>
        <div className="flex items-start justify-between gap-2">
          <p
            data-testid="conversation-preview"
            className={cn(
              'line-clamp-2 min-w-0 flex-1 text-body leading-[18px]',
              unread ? 'font-medium text-foreground' : 'text-muted-foreground',
            )}
          >
            {preview}
          </p>
          {statusBadge && (
            <Badge tone={statusBadge.tone} className="mt-0.5 shrink-0">
              {statusBadge.label}
            </Badge>
          )}
        </div>
      </div>
    </motion.div>
  )
}
