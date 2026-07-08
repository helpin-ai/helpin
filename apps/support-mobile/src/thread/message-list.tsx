import { forwardRef, useCallback, useEffect, useImperativeHandle, useLayoutEffect, useRef } from 'react'
import { Sparkles } from 'lucide-react'
import { cn } from '@mobile/lib/cn'
import { Avatar } from '@mobile/ui/avatar'
import { Skeleton } from '@mobile/ui/skeleton'
import { MessageBubble } from './message-bubble'
import type { ThreadItem } from './thread-helpers'

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
  loading?: boolean
  typingIndicator?: TypingIndicatorState | null
  /** Fires when the list decides the floating "New message" pill should show/hide — the pill itself is rendered by the screen (it floats above the composer, which is screen-level layout). */
  onShowNewMessagePillChange?: (show: boolean) => void
}

function ThreadSkeleton() {
  const rows: Array<{ width: string; align: 'justify-start' | 'justify-end' }> = [
    { width: 'w-40', align: 'justify-start' },
    { width: 'w-56', align: 'justify-end' },
    { width: 'w-32', align: 'justify-start' },
  ]
  return (
    <div className="flex h-full flex-col justify-end gap-3 px-3 pb-4">
      {rows.map((row, index) => (
        <div key={index} className={cn('flex', row.align)}>
          <Skeleton className={cn('h-9 rounded-[16px]', row.width)} />
        </div>
      ))}
    </div>
  )
}

function TypingBubble({ align, label }: TypingIndicatorState) {
  return (
    <div className={cn('mb-3 flex flex-col gap-1', align === 'right' ? 'items-end' : 'items-start')}>
      <span className="px-1 text-footnote text-muted-foreground">{label}</span>
      <div className={cn('flex items-center gap-1 rounded-[16px] px-3 py-2.5', align === 'right' ? 'bg-primary/10' : 'bg-muted')}>
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

function ClusterView({ cluster }: { cluster: Extract<ThreadItem, { kind: 'cluster' }> }) {
  const align = cluster.senderType === 'customer' ? 'left' : 'right'
  const isAI = cluster.senderType === 'ai'
  return (
    <div className="mb-3">
      <div className={cn('mb-1 flex items-center gap-1.5 px-1', align === 'right' && 'flex-row-reverse')}>
        <Avatar name={cluster.senderName} size={18} />
        <span className="text-footnote font-medium text-muted-foreground">{cluster.senderName}</span>
        {isAI && <Sparkles className="h-3 w-3 text-primary" aria-hidden />}
      </div>
      <div className="flex flex-col gap-1">
        {cluster.messages.map((message) => (
          <MessageBubble key={message.id} message={message} align={align} />
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
  { items, loading, typingIndicator, onShowNewMessagePillChange },
  ref,
) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const distanceRef = useRef(0)
  const pillShownRef = useRef(false)
  const initializedRef = useRef(false)
  const prevCountRef = useRef(items.length)

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    const el = scrollRef.current
    if (!el) return
    el.scrollTo({ top: el.scrollHeight, behavior })
    if (pillShownRef.current) {
      pillShownRef.current = false
      onShowNewMessagePillChange?.(false)
    }
  }, [onShowNewMessagePillChange])

  useImperativeHandle(ref, () => ({ scrollToBottom }), [scrollToBottom])

  // Auto-scroll to bottom once, the first time there's something to show.
  useLayoutEffect(() => {
    if (initializedRef.current || loading || items.length === 0) return
    initializedRef.current = true
    scrollToBottom('auto')
  }, [loading, items.length, scrollToBottom])

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
    if (!initializedRef.current) {
      prevCountRef.current = items.length
      return
    }
    if (items.length > prevCountRef.current) {
      if (distanceRef.current <= NEAR_BOTTOM_PX) {
        requestAnimationFrame(() => scrollToBottom('smooth'))
      } else {
        pillShownRef.current = true
        onShowNewMessagePillChange?.(true)
      }
    }
    prevCountRef.current = items.length
  }, [items.length, scrollToBottom, onShowNewMessagePillChange])

  const handleScroll = () => {
    const el = scrollRef.current
    if (!el) return
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight
    distanceRef.current = distance
    if (distance <= NEAR_BOTTOM_PX && pillShownRef.current) {
      pillShownRef.current = false
      onShowNewMessagePillChange?.(false)
    }
  }

  if (loading) return <ThreadSkeleton />

  return (
    <div
      ref={scrollRef}
      onScroll={handleScroll}
      className="h-full overflow-y-auto px-3 pb-[max(var(--safe-bottom),16px)] pt-2"
    >
      {/* Inner wrapper exists solely as the ResizeObserver target: the scroll
          container itself has a fixed height, so content growth is only
          observable on the child. */}
      <div ref={contentRef}>
        {items.map((item, index) =>
          item.kind === 'day' ? (
            <div key={`day-${index}`} className="my-3 flex justify-center">
              <span className="rounded-full bg-muted px-2.5 py-1 text-caption text-muted-foreground">{item.label}</span>
            </div>
          ) : (
            <ClusterView key={`cluster-${index}`} cluster={item} />
          ),
        )}
        {typingIndicator && <TypingBubble align={typingIndicator.align} label={typingIndicator.label} />}
      </div>
    </div>
  )
})
