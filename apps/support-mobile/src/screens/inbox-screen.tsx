import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useRouter } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useVirtualizer } from '@tanstack/react-virtual'
import { Check, ChevronDown, Inbox as InboxIcon, Mail, MailOpen, Menu } from 'lucide-react'
import {
  useConversations,
  useMarkConversationRead,
  useMarkConversationUnread,
  useUnreadStats,
  useUpdateConversationStatus,
} from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import { isTauri } from '@mobile/lib/host'
import { getPushPrimingPref, setLastWorkspaceSlug, shouldShowPriming } from '@mobile/lib/prefs'
import { PermissionPrimingSheet } from '@mobile/push/permission-priming-sheet'
import { useAuthStore } from '@mobile/stores/auth-store'
import { TopBar } from '@mobile/ui/top-bar'
import { OfflineBanner } from '@mobile/ui/offline-banner'
import { Skeleton } from '@mobile/ui/skeleton'
import { EmptyState } from '@mobile/ui/empty-state'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import type { Workspace } from '@mobile/lib/types'
import { toast } from 'sonner'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { useSupportViewStore } from '@mobile/stores/support-view-store'
import { useResolvedTransitionStore } from '@mobile/stores/resolved-transition-store'
import { CELL_EXIT_DURATION_MS, ConversationCell } from '@mobile/inbox/conversation-cell'
import { ViewsDrawer } from '@mobile/inbox/views-drawer'
import { selectionTitle, selectionToConversationFilters } from '@mobile/inbox/use-inbox-filters'
import { SwipeableRow, type SwipeAction } from '@mobile/inbox/swipeable-row'
import { PULL_ARM_THRESHOLD, usePullToRefresh } from '@mobile/inbox/use-pull-to-refresh'
import { CONVERSATION_CELL_HEIGHT, isUnread } from '@mobile/inbox/inbox-helpers'

function InboxSkeletonList() {
  return (
    <div>
      {Array.from({ length: 8 }).map((_, index) => (
        <div
          key={index}
          style={{ height: CONVERSATION_CELL_HEIGHT }}
          className="box-border flex w-full items-center gap-3 border-b border-border/60 px-4"
        >
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

  const selection = useSupportViewStore((s) => s.selection)
  const setSelection = useSupportViewStore((s) => s.setSelection)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const edgeStart = useRef<{ x: number; y: number } | null>(null)
  const workspacesQuery = useQuery({
    queryKey: ['workspaces'],
    queryFn: async () => {
      const { data, error } = await workspacesService.list()
      if (error || !data) throw new Error(error ?? 'Failed to load workspaces')
      return data
    },
    enabled: drawerOpen,
  })

  // Push-notification permission priming: first time a signed-in user lands
  // on Inbox (after the workspace has resolved), show the priming sheet —
  // but only inside the native shell, and only when `shouldShowPriming`
  // says we haven't already asked recently / gotten an answer. Checked once
  // per screen mount rather than on a live subscription: the decision only
  // changes as a result of THIS sheet's own actions, which close the sheet
  // and don't need to re-open it within the same mount.
  const user = useAuthStore((s) => s.user)
  const [primingSheetOpen, setPrimingSheetOpen] = useState(false)
  useEffect(() => {
    if (!workspace || !user || !isTauri()) return
    let cancelled = false
    void getPushPrimingPref().then((pref) => {
      if (!cancelled && shouldShowPriming(pref, new Date())) setPrimingSheetOpen(true)
    })
    return () => {
      cancelled = true
    }
  }, [workspace, user])

  const unreadStats = useUnreadStats(workspaceId)
  // Reuse the web's own filter rulebook so each view returns identical
  // conversations to the web app (see use-inbox-filters).
  const filters = useMemo(() => selectionToConversationFilters(selection), [selection])
  // `keepPrevious` avoids a skeleton flash when switching views — the previous
  // view's data stays on screen (dimmed below) until the new one loads instead
  // of getting torn down first.
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

  // Seed the fade-out → remove lifecycle for a row (shared by swipe-resolve and
  // the "resolved from the thread" transition).
  const beginExit = useCallback((conversationId: string) => {
    setResolving((prev) => new Map(prev).set(conversationId, 'fading'))
    const timer = setTimeout(() => {
      resolveTimersRef.current.delete(conversationId)
      setResolving((prev) => {
        if (prev.get(conversationId) !== 'fading') return prev
        return new Map(prev).set(conversationId, 'removed')
      })
    }, CELL_EXIT_DURATION_MS)
    resolveTimersRef.current.set(conversationId, timer)
  }, [])

  // Cancel a pending exit (mutation error, or Undo) so the row reappears.
  const cancelExit = useCallback((conversationId: string) => {
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
  }, [])

  const handleResolve = (conversationId: string) => {
    beginExit(conversationId)
    updateStatus.mutate(
      { conversationId, status: 'resolved' },
      {
        onError: () => {
          cancelExit(conversationId)
          haptic('notificationError')
        },
      },
    )
  }

  const handleUndoResolve = useCallback(
    (conversationId: string) => {
      cancelExit(conversationId)
      updateStatus.mutate({ conversationId, status: 'open' })
      haptic('impactLight')
    },
    [cancelExit, updateStatus],
  )

  // A conversation resolved from the thread screen: animate it out of the list
  // and offer Undo (the status change already happened server-side).
  const pendingResolvedId = useResolvedTransitionStore((s) => s.pendingResolvedId)
  const clearResolvedTransition = useResolvedTransitionStore((s) => s.clear)
  useEffect(() => {
    if (!pendingResolvedId) return
    const id = pendingResolvedId
    clearResolvedTransition()
    beginExit(id)
    toast.success('Resolved', {
      action: { label: 'Undo', onClick: () => handleUndoResolve(id) },
    })
  }, [pendingResolvedId, clearResolvedTransition, beginExit, handleUndoResolve])

  const scrollRef = useRef<HTMLDivElement>(null)
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

  const currentTitle = selectionTitle(selection)

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

  const handleSelectWorkspace = (nextWorkspace: Workspace) => {
    void setLastWorkspaceSlug(nextWorkspace.slug)
    setCurrentWorkspace({ id: nextWorkspace.id, slug: nextWorkspace.slug, name: nextWorkspace.name })
    setSelection({ kind: 'builtin', navFilter: 'inbox', mailboxId: 'all' })
    router.navigate({ to: '/w/$slug/support', params: { slug: nextWorkspace.slug } })
  }

  return (
    <div className="relative h-dvh">
      <div
        ref={scrollRef}
        onPointerDown={pull.handlers.onPointerDown}
        onPointerMove={pull.handlers.onPointerMove}
        onPointerUp={pull.handlers.onPointerUp}
        onPointerCancel={pull.handlers.onPointerCancel}
        className="h-full overflow-y-auto"
      >
        <TopBar
          title={currentTitle}
          // Title lives inline next to the menu button (drawer nav pattern),
          // so suppress the top bar's own centered title slot.
          titleSlot={<></>}
          leading={
            <Pressable
              aria-label={`Current view: ${currentTitle}. Open views menu`}
              haptic="selection"
              onPress={() => setDrawerOpen(true)}
              className="flex min-w-0 max-w-[72vw] items-center gap-1.5 rounded-full py-1 pr-1.5"
            >
              <Menu className="h-6 w-6 shrink-0 text-foreground" />
              <span className="truncate text-headline">{currentTitle}</span>
              <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
            </Pressable>
          }
        />

        <OfflineBanner />

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

      {/* Left-edge strip: swipe in from the very edge to open the views drawer.
          A dedicated 16px zone so it never competes with the list's vertical
          pull-to-refresh or the rows' horizontal swipe actions. */}
      <button
        type="button"
        aria-hidden
        tabIndex={-1}
        className="fixed inset-y-0 left-0 z-20 w-4"
        onPointerDown={(e) => {
          edgeStart.current = { x: e.clientX, y: e.clientY }
        }}
        onPointerMove={(e) => {
          const start = edgeStart.current
          if (!start) return
          const dx = e.clientX - start.x
          const dy = e.clientY - start.y
          if (dx > 24 && dx > Math.abs(dy)) {
            edgeStart.current = null
            haptic('selection')
            setDrawerOpen(true)
          }
        }}
        onPointerUp={() => {
          edgeStart.current = null
        }}
        onPointerCancel={() => {
          edgeStart.current = null
        }}
      />

      <ViewsDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        workspaceId={workspaceId}
        workspace={workspace}
        workspaces={workspacesQuery.data}
        workspacesLoading={workspacesQuery.isPending}
        workspacesError={workspacesQuery.isError}
        onRetryWorkspaces={() => void workspacesQuery.refetch()}
        onSelectWorkspace={handleSelectWorkspace}
        onOpenSettings={() => router.navigate({ to: '/w/$slug/settings', params: { slug: slug ?? '' } })}
        activeSelection={selection}
        onSelect={setSelection}
      />

      <PermissionPrimingSheet open={primingSheetOpen} onOpenChange={setPrimingSheetOpen} />
    </div>
  )
}
