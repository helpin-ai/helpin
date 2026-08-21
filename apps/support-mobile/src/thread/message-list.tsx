import { forwardRef, useCallback, useEffect, useImperativeHandle, useLayoutEffect, useRef, type ReactNode } from 'react'
import { Sparkles } from 'lucide-react'
import type { AssignableMember, SupportMessage } from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { TeamMemberAvatar } from '@mobile/ui/team-member-avatar'
import { Skeleton } from '@mobile/ui/skeleton'
import { Spinner } from '@mobile/ui/spinner'
import { MessageBubble } from './message-bubble'
import type { SupportReceiptStatus, ThreadItem } from './thread-helpers'

/**
 * Distance-from-bottom (px) threshold from the design spec: "New message
 * pill... when scrolled up >300px and new messages append." Reused as the
 * single threshold for both directions — at or under it, new messages
 * auto-scroll (chat-like "stuck to bottom" feel); beyond it, the pill
 * surfaces instead of yanking the reader down mid-scrollback.
 */
const NEAR_BOTTOM_PX = 300

/**
 * Tighter "pinned to bottom" threshold for the ResizeObserver that
 * compensates content growth after the initial scroll — late-loading
 * `<img>` attachments grow scrollHeight AFTER the mount-time auto-scroll
 * fired, leaving it undershot (and Task 14's optimistic sends will grow
 * content the same way). While the reader is within this distance of the
 * bottom, any content-size growth instantly re-snaps to the bottom; once
 * they scroll up past it, the observer stops forcing and the "New message"
 * pill flow takes over.
 */
const PINNED_TO_BOTTOM_PX = 40
const LOAD_EARLIER_THRESHOLD_PX = 120

export interface TypingIndicatorState {
  align: 'left' | 'right'
  label: string
}

export interface MessageListHandle {
  /** Imperative scroll-to-bottom, exposed for the "New message" pill and for Task 14's composer to call after an own-send. */
  scrollToBottom: (behavior?: ScrollBehavior) => void
}

export interface MessageListProps {
  items: ThreadItem[]
  /** Workspace identities used to resolve generated teammate avatars. */
  members?: AssignableMember[]
  /** Conversation context rendered at the top of the thread so it scrolls away with message history. */
  header?: ReactNode
  loading?: boolean
  typingIndicator?: TypingIndicatorState | null
  /** The id of the last outbound reply that carries a read receipt, and its status. */
  receiptMessageId?: string | null
  receiptStatus?: SupportReceiptStatus | null
  hasEarlier?: boolean
  loadingEarlier?: boolean
  loadEarlierError?: boolean
  onLoadEarlier?: () => void
  /** Stable edge ids and the raw count distinguish prepended history from newly appended chat. */
  messageCount?: number
  oldestMessageId?: string
  newestMessageId?: string
  /** Fires when the list decides the floating "New message" pill should show/hide — the pill itself is rendered by the screen (it floats above the composer, which is screen-level layout). */
  onShowNewMessagePillChange?: (show: boolean) => void
  onMessageActions?: (message: SupportMessage) => void
}

function ActionableMessage({
  message,
  align,
  receiptStatus,
  onMessageActions,
}: {
  message: SupportMessage
  align: 'left' | 'right'
  receiptStatus?: SupportReceiptStatus | null
  onMessageActions?: (message: SupportMessage) => void
}) {
  const showActions = !!onMessageActions && !message.pending && message.message_type !== 'system'
  return (
    <div className="min-w-0">
      <MessageBubble
        message={message}
        align={align}
        receiptStatus={receiptStatus}
        onMessageActions={showActions ? () => onMessageActions!(message) : undefined}
      />
    </div>
  )
}

function ThreadSkeleton() {
  const rows: Array<{ width: string; align: 'justify-start' | 'justify-end'; outgoing?: boolean }> = [
    { width: 'w-40', align: 'justify-start' },
    { width: 'w-56', align: 'justify-end', outgoing: true },
    { width: 'w-32', align: 'justify-start' },
  ]
  return (
    <div className="flex h-full flex-col justify-end gap-3 px-3 pb-4">
      {rows.map((row, index) => (
        <div key={index} className={cn('flex', row.align)}>
          <Skeleton className={cn('h-9 rounded-2xl', row.width, row.outgoing && 'bg-blue-50 dark:bg-blue-950/40')} />
        </div>
      ))}
    </div>
  )
}

function TypingBubble({ align, label }: TypingIndicatorState) {
  return (
    <div className={cn('mb-3 flex flex-col gap-1', align === 'right' ? 'items-end' : 'items-start')}>
      <span className="px-1 text-footnote text-muted-foreground">{label}</span>
      <div className={cn(
        'flex items-center gap-1 rounded-2xl border border-border/40 px-3 py-2.5',
        align === 'right' ? 'rounded-br-sm bg-blue-50 dark:bg-blue-950/40' : 'rounded-bl-sm bg-muted',
      )}>
        {[0, 1, 2].map((dot) => (
          <span
            key={dot}
            className="h-1.5 w-1.5 rounded-full bg-muted-foreground/60 animate-[typing-dot-bounce_1.4s_ease-in-out_infinite]"
            style={{ animationDelay: `${dot * 0.2}s` }}
          />
        ))}
      </div>
    </div>
  )
}

function ClusterView({
  cluster,
  receiptMessageId,
  receiptStatus,
  onMessageActions,
  member,
}: {
  cluster: Extract<ThreadItem, { kind: 'cluster' }>
  receiptMessageId?: string | null
  receiptStatus?: SupportReceiptStatus | null
  onMessageActions?: (message: SupportMessage) => void
  member?: AssignableMember
}) {
  const receiptFor = (id: string) => (id === receiptMessageId ? receiptStatus : undefined)

  // Internal notes render as a right-aligned amber card with no avatar (mirrors
  // web, where the note branch has no avatar column). A cluster is homogeneous
  // in `is_internal`, so the first message decides.
  if (cluster.messages[0]?.is_internal) {
    return (
      <div className="mb-3 flex flex-col gap-1">
        {cluster.messages.map((message) => (
          <ActionableMessage
            key={message.id}
            message={message}
            align="left"
            onMessageActions={onMessageActions}
          />
        ))}
      </div>
    )
  }

  const align = cluster.senderType === 'customer' ? 'left' : 'right'
  const isAI = cluster.senderType === 'ai'
  const firstMessage = cluster.messages[0]
  const avatarSeed = firstMessage?.sender_user_id
    ?? firstMessage?.sender_agent_id
    ?? firstMessage?.sender_display_name
    ?? cluster.senderName
  // Crisp-style: one avatar per group, sitting inline at the bottom of the
  // bubble column (customer on the left, our side on the right). No visible
  // sender-name header — the name is the avatar's native tooltip, like web.
  return (
    <div className={cn('mb-4 flex items-end gap-2.5', align === 'right' && 'flex-row-reverse')}>
      <div title={cluster.senderName} className="relative shrink-0">
        <TeamMemberAvatar
          name={cluster.senderName}
          src={cluster.senderAvatarUrl}
          member={member}
          size={28}
          initialCount={1}
          fallbackSeed={avatarSeed}
        />
        {isAI && (
          <span className="absolute -bottom-0.5 -right-0.5 flex h-3.5 w-3.5 items-center justify-center rounded-full bg-background ring-2 ring-background">
            <Sparkles className="h-2.5 w-2.5 text-primary" aria-hidden />
          </span>
        )}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        {cluster.messages.map((message) => (
          <ActionableMessage
            key={message.id}
            message={message}
            align={align}
            receiptStatus={receiptFor(message.id)}
            onMessageActions={onMessageActions}
          />
        ))}
      </div>
    </div>
  )
}

/**
 * Plain (non-virtualized) chronological scroll container — the plan decision
 * is to skip virtualization at typical support-thread sizes and revisit in
 * Task 22's profiling pass if needed. Owns auto-scroll-to-bottom (on mount
 * and when already near the bottom) and reports when new messages arrive
 * while the reader has scrolled up, so the screen can surface a "New
 * message" pill.
 */
export const MessageList = forwardRef<MessageListHandle, MessageListProps>(function MessageList(
  {
    items,
    members = [],
    header,
    loading,
    typingIndicator,
    receiptMessageId,
    receiptStatus,
    hasEarlier,
    loadingEarlier,
    loadEarlierError,
    onLoadEarlier,
    messageCount,
    oldestMessageId,
    newestMessageId,
    onShowNewMessagePillChange,
    onMessageActions,
  },
  ref,
) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const distanceRef = useRef(0)
  const pillShownRef = useRef(false)
  const initializedRef = useRef(false)
  const prevMessageCountRef = useRef(messageCount ?? items.length)
  const prevNewestMessageIdRef = useRef(newestMessageId)
  const loadEarlierRequestedRef = useRef(false)
  const prependAnchorRef = useRef<{ scrollHeight: number; scrollTop: number; oldestMessageId?: string } | null>(null)
  const memberByUserId = new Map(
    members.filter((member) => member.user_id).map((member) => [member.user_id!, member]),
  )

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    const el = scrollRef.current
    if (!el) return
    el.scrollTo({ top: el.scrollHeight, behavior })
    if (pillShownRef.current) {
      pillShownRef.current = false
      onShowNewMessagePillChange?.(false)
    }
  }, [onShowNewMessagePillChange])

  const requestEarlier = useCallback(() => {
    const el = scrollRef.current
    if (!el || !hasEarlier || loadingEarlier || loadEarlierRequestedRef.current || !onLoadEarlier) return
    prependAnchorRef.current = {
      scrollHeight: el.scrollHeight,
      scrollTop: el.scrollTop,
      oldestMessageId,
    }
    loadEarlierRequestedRef.current = true
    onLoadEarlier()
  }, [hasEarlier, loadingEarlier, oldestMessageId, onLoadEarlier])

  useImperativeHandle(ref, () => ({ scrollToBottom }), [scrollToBottom])

  // Auto-scroll to bottom once, the first time there's something to show.
  useLayoutEffect(() => {
    if (initializedRef.current || loading || items.length === 0) return
    initializedRef.current = true
    scrollToBottom('auto')
  }, [loading, items.length, scrollToBottom])

  // Preserve the visible message when an older page is inserted above it.
  useLayoutEffect(() => {
    const anchor = prependAnchorRef.current
    const el = scrollRef.current
    if (!anchor || !el || oldestMessageId === anchor.oldestMessageId) return
    el.scrollTop = anchor.scrollTop + (el.scrollHeight - anchor.scrollHeight)
    distanceRef.current = el.scrollHeight - el.scrollTop - el.clientHeight
    prependAnchorRef.current = null
  }, [items.length, oldestMessageId])

  // While pinned to the bottom, keep it that way through content growth
  // (image loads, expanders, optimistic sends) via a ResizeObserver on the
  // inner content wrapper. distanceRef starts at 0, so the freshly-mounted
  // list counts as pinned and image loads right after the initial
  // auto-scroll get compensated. Deliberately keyed on `loading`: the
  // scroll container only mounts once loading finishes, so the observer
  // must (re)attach then. Disconnected on cleanup/unmount.
  //
  // Task 14 addition: also observe `el` (the scroll container) itself, not
  // just its content. The composer rides the keyboard via a `margin-bottom`
  // that tracks `--keyboard-inset` — when the keyboard opens, that margin
  // grows, which shrinks THIS container's height (it's a flex-1 sibling
  // above the composer), not its content height. A resize of `el` alone
  // would be invisible to a content-only observer, and a pinned-to-bottom
  // reader would see the last message slide up behind the keyboard until
  // they scrolled again. Observing both elements on the same ResizeObserver
  // instance re-snaps on either kind of size change with the one existing
  // "am I pinned" check.
  useEffect(() => {
    const el = scrollRef.current
    const content = contentRef.current
    if (loading || !el || !content) return
    const observer = new ResizeObserver(() => {
      if (distanceRef.current > PINNED_TO_BOTTOM_PX) return
      el.scrollTo({ top: el.scrollHeight })
      // scrollTo doesn't always fire a scroll event synchronously — keep the
      // pinned-distance bookkeeping accurate ourselves.
      distanceRef.current = el.scrollHeight - el.scrollTop - el.clientHeight
    })
    observer.observe(content)
    observer.observe(el)
    return () => observer.disconnect()
  }, [loading])

  // New items appended after the initial paint: stick to bottom if the
  // reader was already there, otherwise surface the "New message" pill.
  useEffect(() => {
    const currentMessageCount = messageCount ?? items.length
    if (!initializedRef.current) {
      prevMessageCountRef.current = currentMessageCount
      prevNewestMessageIdRef.current = newestMessageId
      return
    }
    const appended =
      currentMessageCount > prevMessageCountRef.current &&
      newestMessageId !== prevNewestMessageIdRef.current
    if (appended) {
      if (distanceRef.current <= NEAR_BOTTOM_PX) {
        requestAnimationFrame(() => scrollToBottom('smooth'))
      } else {
        pillShownRef.current = true
        onShowNewMessagePillChange?.(true)
      }
    }
    prevMessageCountRef.current = currentMessageCount
    prevNewestMessageIdRef.current = newestMessageId
  }, [items.length, messageCount, newestMessageId, scrollToBottom, onShowNewMessagePillChange])

  const handleScroll = () => {
    const el = scrollRef.current
    if (!el) return
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight
    distanceRef.current = distance
    if (distance <= NEAR_BOTTOM_PX && pillShownRef.current) {
      pillShownRef.current = false
      onShowNewMessagePillChange?.(false)
    }
    if (el.scrollTop <= LOAD_EARLIER_THRESHOLD_PX) requestEarlier()
  }

  useEffect(() => {
    if (!loadingEarlier) loadEarlierRequestedRef.current = false
  }, [loadingEarlier, items.length, loadEarlierError])

  // A short first page may not create a scrollbar, so continue until the
  // viewport is filled or the server reports that history is exhausted.
  useEffect(() => {
    const el = scrollRef.current
    if (!loading && el && el.scrollHeight <= el.clientHeight + LOAD_EARLIER_THRESHOLD_PX) {
      requestEarlier()
    }
  }, [loading, items.length, requestEarlier])

  if (loading) return <ThreadSkeleton />

  return (
    <div
      ref={scrollRef}
      onScroll={handleScroll}
      className="relative h-full overflow-y-auto px-4 pb-[max(var(--safe-bottom),20px)] pt-3"
    >
      {(loadingEarlier || (loadEarlierError && hasEarlier)) && (
        <div className="sticky top-2 z-10 -mb-8 flex h-8 items-center justify-center">
          {loadingEarlier ? (
            <span className="flex items-center gap-2 rounded-full border border-border/70 bg-background/95 px-3 py-1.5 text-caption text-muted-foreground backdrop-blur">
              <Spinner size={14} />
              Loading earlier messages
            </span>
          ) : (
            <button
              type="button"
              onClick={requestEarlier}
              className="rounded-full border border-border/70 bg-background/95 px-3 py-1.5 text-caption font-medium text-foreground backdrop-blur"
            >
              Retry earlier messages
            </button>
          )}
        </div>
      )}
      {/* Inner wrapper exists solely as the ResizeObserver target: the scroll
          container itself has a fixed height, so content growth is only
          observable on the child. */}
      <div ref={contentRef}>
        {header}
        {items.map((item, index) =>
          item.kind === 'day' ? (
            <div key={`day-${index}`} className="my-3 flex justify-center">
              <span className="rounded-full bg-muted px-2.5 py-1 text-caption text-muted-foreground">{item.label}</span>
            </div>
          ) : item.kind === 'system' ? (
            <div key={`system-${index}`} className={items[index + 1]?.kind === 'cluster' ? 'mb-3' : undefined}>
              <MessageBubble
                message={item.message}
                align="left"
                senderMember={item.message.sender_user_id ? memberByUserId.get(item.message.sender_user_id) : undefined}
              />
            </div>
          ) : (
            <ClusterView
              key={`cluster-${index}`}
              cluster={item}
              receiptMessageId={receiptMessageId}
              receiptStatus={receiptStatus}
              onMessageActions={onMessageActions}
              member={item.messages[0]?.sender_user_id ? memberByUserId.get(item.messages[0].sender_user_id) : undefined}
            />
          ),
        )}
        {typingIndicator && <TypingBubble align={typingIndicator.align} label={typingIndicator.label} />}
      </div>
    </div>
  )
})
