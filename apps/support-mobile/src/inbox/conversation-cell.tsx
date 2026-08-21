import { Bot, CheckCircle2, CornerUpLeft, Mail, MessageCircle } from 'lucide-react'
import { motion } from 'motion/react'
import type { ComponentType } from 'react'
import type { AssignableMember, SupportConversation } from '@helpin-ai/support-core'
import { useSupportPresenceStore } from '@helpin-ai/support-core'
import {
  getConversationRowVisualState,
  getSupportTagPillStyle,
  isNotePreview,
  stripNotePrefix,
} from '@/components/support/conversationRowVisual'
import { getAvatarColor } from '@/components/support/helpers'
import { cn } from '@mobile/lib/cn'
import { Avatar } from '@mobile/ui/avatar'
import { CONVERSATION_CELL_HEIGHT, formatRelativeTime, isUnread, previewText } from './inbox-helpers'

const CHANNEL_ICONS: Partial<
  Record<SupportConversation['source'], ComponentType<{ className?: string; 'aria-label'?: string }>>
> = {
  email: Mail,
  widget: MessageCircle,
}

const CHANNEL_LABELS: Partial<Record<SupportConversation['source'], string>> = {
  email: 'Email',
  widget: 'Live chat',
}

/** Flow states that mean the conversation is queued and waiting on a human. Mirrors web's HUMAN_QUEUE_FLOW_STATES. */
const HUMAN_QUEUE_FLOW_STATES = new Set(['queued_for_human', 'after_hours_queue'])

const MAX_VISIBLE_TAGS = 2
const EMPTY_VIEWING_AGENT_IDS: string[] = []

/** A conversation's display name — customer name, else email, else a short visitor id, else "Anonymous". */
export function displayNameFor(conversation: SupportConversation): string {
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

/** Three animated dots — customer/agent is typing. */
function TypingDots() {
  return (
    <span aria-label="typing" className="inline-flex items-center gap-0.5 rounded-full bg-foreground/10 px-1.5 py-1">
      <span className="h-1 w-1 animate-bounce rounded-full bg-foreground/50 [animation-delay:0ms]" />
      <span className="h-1 w-1 animate-bounce rounded-full bg-foreground/50 [animation-delay:150ms]" />
      <span className="h-1 w-1 animate-bounce rounded-full bg-foreground/50 [animation-delay:300ms]" />
    </span>
  )
}

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
  reviewerMembers?: AssignableMember[]
  currentUserId?: string
}

/**
 * Fixed 80px row — the virtualizer's `estimateSize` (inbox-screen.tsx) depends
 * on this exact height, so the outer element must never grow or shrink based
 * on content (hence `overflow-hidden` + line clamps).
 *
 * Signals mirror the web conversation row (see ConversationRow.tsx): unread
 * count badge, "needs team action" row tint, internal-note label, agent-reply
 * indicator, AI handoff / resolved badges, waiting-for-human pill, coloured
 * tags, plus live presence (customer/agent typing, viewing agents, visitor
 * online) read from the shared support presence store.
 */
export function ConversationCell({
  conversation,
  onPress,
  isExiting,
  reviewerMembers = [],
  currentUserId,
}: ConversationCellProps) {
  const displayName = displayNameFor(conversation)
  const preview = previewText(conversation)
  const time = formatRelativeTime(conversation.updated_at)
  const ChannelIcon = CHANNEL_ICONS[conversation.source]
  const unread = isUnread(conversation)
  const unreadCount = conversation.unread_count ?? 0
  const visual = getConversationRowVisualState(conversation)

  // Live presence (populated by useSupportRealtime, which the inbox runs).
  const customerTyping = useSupportPresenceStore((s) => s.typingIndicators[conversation.id])
  const isCustomerTyping = typeof customerTyping === 'string'
  const agentTypingMap = useSupportPresenceStore((s) => s.agentTyping[conversation.id])
  const firstAgentTyping = agentTypingMap ? Object.values(agentTypingMap)[0] : undefined
  const isAgentTyping = !!firstAgentTyping
  const viewingAgentIds = useSupportPresenceStore((s) => s.viewingAgents[conversation.id] ?? EMPTY_VIEWING_AGENT_IDS)
  const reviewerByUserId = new Map(
    reviewerMembers.filter((member) => member.user_id).map((member) => [member.user_id!, member]),
  )
  const reviewers = viewingAgentIds
    .filter((id) => id !== currentUserId)
    .map((id) => reviewerByUserId.get(id) ?? { id, display_name: 'Teammate', avatar_url: undefined })
  const isVisitorOnline = useSupportPresenceStore((s) =>
    conversation.anonymous_id ? !!s.onlineVisitors[conversation.anonymous_id] : false,
  )

  const systemTags = conversation.system_tags ?? []
  const hasAIHandoff = systemTags.includes('ai_handoff')
  const hasAIResolved =
    systemTags.includes('ai_resolved') ||
    conversation.flow_state === 'resolved_by_ai' ||
    conversation.ai_state === 'resolved'
  const isWaitingForHuman = !!conversation.flow_state && HUMAN_QUEUE_FLOW_STATES.has(conversation.flow_state)
  const waitSince = conversation.customer_requested_human_at || conversation.ai_escalated_at || conversation.updated_at
  const hasAgentReplyPreview =
    conversation.last_message_sender_type === 'user' || conversation.last_message_sender_type === 'agent'
  const agentFirstName = firstAgentTyping?.name?.split(' ')[0] || 'Agent'
  const isNote = isNotePreview(conversation.last_message)

  const tags = conversation.tags ?? []
  const visibleTags = tags.slice(0, MAX_VISIBLE_TAGS)
  const hiddenTagCount = Math.max(0, tags.length - visibleTags.length)
  const hasSecondaryRow = isWaitingForHuman || tags.length > 0

  // Explicit label so VoiceOver/TalkBack announce "«name», «preview», «time»,
  // unread" instead of the default subtree computation (which also picks up
  // avatar initials). Kept plain/stable regardless of live-presence overlays.
  const cellLabel = [displayName, preview, time, unread ? 'unread' : null].filter(Boolean).join(', ')

  return (
    <motion.div
      role="button"
      tabIndex={0}
      aria-label={cellLabel}
      data-testid="conversation-cell"
      onClick={onPress}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onPress()
        }
      }}
      animate={isExiting ? { opacity: 0, x: 56, scale: 0.98 } : { opacity: 1, x: 0, scale: 1 }}
      transition={{ duration: CELL_EXIT_DURATION_MS / 1000, ease: 'easeOut' }}
      style={{ height: CONVERSATION_CELL_HEIGHT }}
      className={cn(
        'box-border flex w-full cursor-pointer items-center gap-2.5 overflow-hidden border-b border-border/60 px-4 text-left active:bg-muted/50',
        // "Needs team action" (open + unread/awaiting reply) gets a subtle tint, mirroring web.
        visual.needsTeamAction ? 'bg-primary/[0.06]' : 'bg-background',
      )}
    >
      <div className="relative shrink-0">
        <Avatar
          name={displayName}
          size={40}
          className={getAvatarColor(
            conversation.customer_email || conversation.customer_name || conversation.id,
          )}
        />
        {isVisitorOnline && (
          <span
            aria-label="Visitor online"
            className="absolute -left-0.5 -top-0.5 h-2.5 w-2.5 rounded-full bg-green-400 ring-2 ring-background"
          />
        )}
      </div>

      <div className="flex min-w-0 flex-1 flex-col justify-center gap-0.5">
        {/* Channel icon + name + time */}
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            {ChannelIcon && (
              <ChannelIcon
                aria-label={CHANNEL_LABELS[conversation.source]}
                className="h-3.5 w-3.5 shrink-0 text-muted-foreground/70"
              />
            )}
            <span
              data-testid="conversation-name"
              className={cn(
                'min-w-0 truncate text-body text-foreground',
                visual.usesUnreadTypography ? 'font-semibold' : 'font-medium',
              )}
            >
              {displayName}
            </span>
          </div>
          <span data-testid="conversation-time" className="shrink-0 text-footnote tnum text-muted-foreground">
            {time}
          </span>
        </div>

        {/* Preview + trailing indicator */}
        <div className="flex items-center justify-between gap-2">
          {/* The reply icon is a flex sibling of the clamped text (not inside it):
              a leading icon inside a line-clamp box gets pushed onto its own
              line, wrapping the preview to the next line. */}
          <div className="flex min-w-0 flex-1 items-start gap-1">
            {hasAgentReplyPreview && !isCustomerTyping && !isAgentTyping && !isNote && (
              <CornerUpLeft aria-label="Team replied" className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground/70" />
            )}
            <p
              data-testid="conversation-preview"
              className={cn(
                'min-w-0 flex-1 text-footnote',
                hasSecondaryRow ? 'line-clamp-1' : 'line-clamp-2',
                visual.usesUnreadTypography ? 'font-medium text-foreground/80' : 'text-muted-foreground',
              )}
            >
              {isCustomerTyping ? (
                <span className="italic text-muted-foreground">{customerTyping || 'typing…'}</span>
              ) : isAgentTyping ? (
                <span className="italic text-primary/80">{agentFirstName} is typing…</span>
              ) : isNote ? (
                <>
                  <span className="font-medium text-amber-600 dark:text-amber-400">Note: </span>
                  {stripNotePrefix(conversation.last_message ?? '')}
                </>
              ) : (
                preview
              )}
            </p>
          </div>

          {/* Trailing status/activity — mirrors web's exclusive indicator chain. */}
          <div className="flex shrink-0 items-center gap-1">
            {hasAIHandoff && (
              <span
                aria-label="AI handed off to team"
                className="inline-flex h-4 w-4 items-center justify-center rounded-full bg-amber-100 text-amber-700 ring-1 ring-amber-200/80 dark:bg-amber-950/40 dark:text-amber-300"
              >
                <Bot className="h-2.5 w-2.5" />
              </span>
            )}
            {isCustomerTyping || isAgentTyping ? (
              <TypingDots />
            ) : unread ? (
              <span
                data-testid="unread-count"
                className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-semibold text-white"
              >
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            ) : reviewers.length > 0 ? (
              <span
                aria-label={`${reviewers.map((reviewer) => reviewer.display_name).join(', ')} viewing`}
                title={`${reviewers.map((reviewer) => reviewer.display_name).join(', ')} viewing`}
                className="flex flex-row-reverse justify-end pl-1"
              >
                {reviewers.slice(0, 3).reverse().map((reviewer, index) => (
                  <Avatar
                    key={reviewer.id}
                    name={reviewer.display_name}
                    src={reviewer.avatar_url}
                    size={20}
                    className={cn(
                      index !== 0 && '-mr-1.5',
                      'ring-2 ring-background',
                      getAvatarColor(reviewer.id || reviewer.display_name),
                    )}
                  />
                ))}
                {reviewers.length > 3 && (
                  <span className="-mr-1.5 flex h-5 min-w-5 items-center justify-center rounded-full bg-muted px-1 text-[9px] font-semibold ring-2 ring-background">
                    +{reviewers.length - 3}
                  </span>
                )}
              </span>
            ) : hasAIResolved ? (
              <span
                aria-label="Resolved by AI"
                className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 ring-1 ring-emerald-200/80 dark:bg-emerald-950/40 dark:text-emerald-300"
              >
                <Bot className="h-3 w-3" />
              </span>
            ) : conversation.status === 'resolved' ? (
              <CheckCircle2 aria-label="Resolved" className="h-5 w-5 text-green-500" />
            ) : null}
          </div>
        </div>

        {/* Secondary row: waiting-for-human pill + coloured tags (only when present). */}
        {hasSecondaryRow && (
          <div className="mt-0.5 flex min-w-0 items-center gap-1 overflow-hidden">
            {isWaitingForHuman && (
              <span className="inline-flex shrink-0 items-center rounded-full border border-amber-200/70 bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium leading-none text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
                Waiting for human · {formatRelativeTime(waitSince)}
              </span>
            )}
            {visibleTags.map((tag) => (
              <span
                key={tag.id}
                title={tag.name}
                style={getSupportTagPillStyle(tag.color)}
                className="inline-flex max-w-[7rem] shrink-0 items-center rounded-full border border-border/70 bg-background/70 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground"
              >
                <span className="min-w-0 truncate">{tag.name}</span>
              </span>
            ))}
            {hiddenTagCount > 0 && (
              <span className="inline-flex shrink-0 items-center rounded-full border border-border/70 bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">
                +{hiddenTagCount}
              </span>
            )}
          </div>
        )}
      </div>
    </motion.div>
  )
}
