import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useRouter } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useMotionValue } from 'motion/react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { Check, ChevronsUpDown, Inbox as InboxIcon, Mail, MailOpen } from 'lucide-react'
import {
  useConversations,
  useMarkConversationRead,
  useMarkConversationUnread,
  useSupportMailboxes,
  useUnreadStats,
  useUpdateConversationStatus,
} from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import { TopBar } from '@mobile/ui/top-bar'
import { OfflineBanner } from '@mobile/ui/offline-banner'
import { SegmentedControl, type Segment } from '@mobile/ui/segmented-control'
import { Skeleton } from '@mobile/ui/skeleton'
import { EmptyState } from '@mobile/ui/empty-state'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { TabShell } from '@mobile/navigation/tab-bar'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { CELL_EXIT_DURATION_MS, ConversationCell } from '@mobile/inbox/conversation-cell'
import { MailboxSheet } from '@mobile/inbox/mailbox-sheet'
import { SwipeableRow, type SwipeAction } from '@mobile/inbox/swipeable-row'
import { PULL_ARM_THRESHOLD, usePullToRefresh } from '@mobile/inbox/use-pull-to-refresh'
import { CONVERSATION_CELL_HEIGHT, filtersForSegment, isUnread, type InboxSegment } from '@mobile/inbox/inbox-helpers'

const SEGMENTS: Segment<InboxSegment>[] = [
  { value: 'mine', label: 'Mine' },
  { value: 'unassigned', label: 'Unassigned' },
  { value: 'all', label: 'All' },
]

function InboxSkeletonList() {
  return (
    <div>
      {Array.from({ length: 8 }).map((_, index) => (
        <div
          key={index}
          style={{ height: CONVERSATION_CELL_HEIGHT }}
          className="box-border flex w-full items-center gap-3 border-b border-border/60 px-4"
        >
          <div className="w-4 shrink-0" />
          <Skeleton className="h-11 w-11 shrink-0 rounded-full" />
          <div className="flex min-w-0 flex-1 flex-col justify-center gap-2">
            <div className="flex items-center justify-between gap-2">
              <Skeleton className="h-3.5 w-32" />
              <Skeleton className="h-3 w-8" />
            </div>
            <Skeleton className="h-3.5 w-full" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function InboxScreen() {
  const { slug } = useParams({ strict: false })
  const router = useRouter()
  const setCurrentWorkspace = useWorkspaceStore((s) => s.setCurrentWorkspace)

  const workspaceQuery = useQuery({
    queryKey: ['workspace', slug],
    queryFn: async () => {
      const { data, error } = await workspacesService.getBySlug(slug ?? '')
      if (error || !data) throw new Error(error ?? 'Failed to load workspace')
      return data
    },
    enabled: !!slug,
  })
  const workspace = workspaceQuery.data
  const workspaceId = workspace?.id ?? ''

  useEffect(() => {
    if (workspace) {
      setCurrentWorkspace({ id: workspace.id, slug: workspace.slug, name: workspace.name })
    }
  }, [workspace, setCurrentWorkspace])

  const [segment, setSegment] = useState<InboxSegment>('mine')
  const [selectedMailboxId, setSelectedMailboxId] = useState<string | null>(null)
  const [mailboxSheetOpen, setMailboxSheetOpen] = useState(false)

  const unreadStats = useUnreadStats(workspaceId, selectedMailboxId)
  const mailboxesQuery = useSupportMailboxes(workspaceId)
  const filters = useMemo(() => filtersForSegment(segment, selectedMailboxId), [segment, selectedMailboxId])
  // `keepPrevious` avoids a skeleton flash when switching segments/mailboxes —
  // the previous page's data stays on screen (dimmed below) until the new
  // page loads instead of getting torn down first.
  const conversationsQuery = useConversations(workspaceId, filters, true)
  const rawConversations = useMemo(() => conversationsQuery.data?.data ?? [], [conversationsQuery.data])
  const markRead = useMarkConversationRead(workspaceId)
  const markUnread = useMarkConversationUnread(workspaceId)
  const updateStatus = useUpdateConversationStatus(workspaceId)

  // Resolve-swipe lifecycle per conversation id:
  //   'fading'  — still rendered, cell fading out (CELL_EXIT_DURATION_MS)
  //   'removed' — filtered out of the rendered list (no ghost full-height row
  //               while the mutation/refetch is still in flight)
  // Ids are dropped from the map when (a) the mutation errors (row reappears +
  // error haptic), (b) the refetched list no longer contains the row, or
  // (c) the refetched row is confirmed resolved server-side (e.g. the "All"
  // segment keeps resolved conversations — show it again with its badge).
  const [resolving, setResolving] = useState<Map<string, 'fading' | 'removed'>>(new Map())
  const resolveTimersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
  useEffect(() => {
    const timers = resolveTimersRef.current
    return () => timers.forEach((timer) => clearTimeout(timer))
  }, [])

  useEffect(() => {
    setResolving((prev) => {
      if (prev.size === 0) return prev
      const byId = new Map(rawConversations.map((c) => [c.id, c]))
      const next = new Map(prev)
      let changed = false
      prev.forEach((stage, id) => {
        const conversation = byId.get(id)
        if (!conversation || (stage === 'removed' && conversation.status === 'resolved')) {
          next.delete(id)
          changed = true
        }
      })
      return changed ? next : prev
    })
  }, [rawConversations])

  const conversations = useMemo(
    () => rawConversations.filter((c) => resolving.get(c.id) !== 'removed'),
    [rawConversations, resolving],
  )

  const handleResolve = (conversationId: string) => {
    setResolving((prev) => new Map(prev).set(conversationId, 'fading'))
    const timer = setTimeout(() => {
      resolveTimersRef.current.delete(conversationId)
      setResolving((prev) => {
        // Skip if the mutation already errored (id gone) mid-fade.
        if (prev.get(conversationId) !== 'fading') return prev
        return new Map(prev).set(conversationId, 'removed')
      })
    }, CELL_EXIT_DURATION_MS)
    resolveTimersRef.current.set(conversationId, timer)

    updateStatus.mutate(
      { conversationId, status: 'resolved' },
      {
        onError: () => {
          const pending = resolveTimersRef.current.get(conversationId)
          if (pending !== undefined) {
            clearTimeout(pending)
            resolveTimersRef.current.delete(conversationId)
          }
          setResolving((prev) => {
            if (!prev.has(conversationId)) return prev
            const next = new Map(prev)
            next.delete(conversationId)
            return next
          })
          haptic('notificationError')
        },
      },
    )
  }

  const scrollRef = useRef<HTMLDivElement>(null)
  const scrollY = useMotionValue(0)
  const virtualizer = useVirtualizer({
    count: conversations.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => CONVERSATION_CELL_HEIGHT,
    overscan: 8,
  })

  const pull = usePullToRefresh({
    scrollRef,
    onRefresh: async () => {
      await Promise.all([conversationsQuery.refetch(), unreadStats.refetch()])
    },
  })
  const pullSpinning = pull.refreshing || pull.pullDistance >= PULL_ARM_THRESHOLD

  // Current mailbox's name, or "Inbox" for the "All inboxes" scope / while the
  // mailbox list hasn't loaded yet.
  const mailboxName = useMemo(() => {
    if (!selectedMailboxId || selectedMailboxId === 'all') return 'Inbox'
    return mailboxesQuery.data?.find((m) => m.id === selectedMailboxId)?.name ?? 'Inbox'
  }, [selectedMailboxId, mailboxesQuery.data])

  const isWorkspaceLoading = workspaceQuery.isPending
  const isWorkspaceError = workspaceQuery.isError
  const hasConversationData = conversationsQuery.data !== undefined
  const showSkeleton = isWorkspaceLoading || (!!workspaceId && !hasConversationData && !conversationsQuery.isError)
  const showError = !isWorkspaceLoading && (isWorkspaceError || conversationsQuery.isError)
  const showEmpty = !showSkeleton && !showError && conversations.length === 0

  const handleRetry = () => {
    if (isWorkspaceError) {
      void workspaceQuery.refetch()
    } else {
      void conversationsQuery.refetch()
    }
  }

  const handleSelectConversation = (conversationId: string) => {
    router.navigate({ to: '/w/$slug/support/$conversationId', params: { slug: slug ?? '', conversationId } })
  }

  return (
    <TabShell workspaceSlug={slug ?? ''} workspaceId={workspaceId}>
      <div
        ref={scrollRef}
        onScroll={(e) => scrollY.set(e.currentTarget.scrollTop)}
        onPointerDown={pull.handlers.onPointerDown}
        onPointerMove={pull.handlers.onPointerMove}
        onPointerUp={pull.handlers.onPointerUp}
        onPointerCancel={pull.handlers.onPointerCancel}
        className="h-full overflow-y-auto"
      >
        <TopBar
          large
          title={mailboxName}
          scrollY={scrollY}
          trailing={
            <Pressable
              aria-label="Choose mailbox"
              haptic="selection"
              onPress={() => setMailboxSheetOpen(true)}
              className="flex items-center justify-center rounded-full"
            >
              <ChevronsUpDown className="h-5 w-5 text-muted-foreground" />
            </Pressable>
          }
        />

        <OfflineBanner />

        <div className="px-4 pb-2">
          <SegmentedControl
            segments={SEGMENTS.map((s) => ({
              ...s,
              count:
                s.value === 'mine'
                  ? unreadStats.data?.my_inbox
                  : s.value === 'unassigned'
                    ? unreadStats.data?.unassigned
                    : unreadStats.data?.total,
            }))}
            value={segment}
            onChange={setSegment}
          />
        </div>

        <div
          style={{ height: pull.pullDistance }}
          className="flex items-end justify-center overflow-hidden"
        >
          {(pull.pullDistance > 0 || pull.refreshing) && (
            <div
              className="pb-2"
              style={pullSpinning ? undefined : { transform: `rotate(${pull.pullDistance * 2.6}deg)` }}
            >
              <Spinner className={pullSpinning ? undefined : 'animate-none'} />
            </div>
          )}
        </div>

        {showSkeleton && <InboxSkeletonList />}

        {showError && (
          <EmptyState
            icon={<InboxIcon className="h-6 w-6" />}
            title="Couldn't load conversations"
            body="Check your connection and try again."
            action={
              <Pressable
                haptic="impactLight"
                onPress={handleRetry}
                className="rounded-full bg-primary px-4 py-2 text-body font-medium text-primary-foreground"
              >
                {conversationsQuery.isFetching || workspaceQuery.isFetching ? <Spinner /> : 'Retry'}
              </Pressable>
            }
          />
        )}

        {showEmpty && (
          <EmptyState
            icon={<InboxIcon className="h-6 w-6" />}
            title="Inbox zero"
            body="New conversations will appear here."
          />
        )}

        {!showSkeleton && !showError && !showEmpty && (
          <div
            style={{ height: virtualizer.getTotalSize(), position: 'relative' }}
            className={cn(
              'transition-opacity duration-200',
              conversationsQuery.isPlaceholderData && 'opacity-60',
            )}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const conversation = conversations[virtualRow.index]
              const unread = isUnread(conversation)
              const leadingAction: SwipeAction = unread
                ? {
                    label: 'Read',
                    icon: MailOpen,
                    tone: 'primary',
                    onCommit: () => markRead.mutate(conversation.id),
                  }
                : {
                    label: 'Unread',
                    icon: Mail,
                    tone: 'primary',
                    onCommit: () => markUnread.mutate(conversation.id),
                  }
              const trailingAction: SwipeAction = {
                label: 'Resolve',
                icon: Check,
                tone: 'success',
                onCommit: () => handleResolve(conversation.id),
              }
              return (
                <div
                  key={conversation.id}
                  data-index={virtualRow.index}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: virtualRow.size,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  <SwipeableRow leading={leadingAction} trailing={trailingAction}>
                    <ConversationCell
                      conversation={conversation}
                      onPress={() => handleSelectConversation(conversation.id)}
                      isExiting={resolving.get(conversation.id) === 'fading'}
                    />
                  </SwipeableRow>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <MailboxSheet
        workspaceId={workspaceId}
        open={mailboxSheetOpen}
        onOpenChange={setMailboxSheetOpen}
        selectedMailboxId={selectedMailboxId}
        onSelect={setSelectedMailboxId}
      />
    </TabShell>
  )
}
