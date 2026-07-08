import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useRouter } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useMotionValue } from 'motion/react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { ChevronsUpDown, Inbox as InboxIcon } from 'lucide-react'
import { useConversations, useSupportMailboxes, useUnreadStats } from '@helpin-ai/support-core'
import { TopBar } from '@mobile/ui/top-bar'
import { SegmentedControl, type Segment } from '@mobile/ui/segmented-control'
import { Skeleton } from '@mobile/ui/skeleton'
import { EmptyState } from '@mobile/ui/empty-state'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { TabShell } from '@mobile/navigation/tab-bar'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { ConversationCell } from '@mobile/inbox/conversation-cell'
import { MailboxSheet } from '@mobile/inbox/mailbox-sheet'
import { CONVERSATION_CELL_HEIGHT, filtersForSegment, type InboxSegment } from '@mobile/inbox/inbox-helpers'

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
  const conversationsQuery = useConversations(workspaceId, filters)
  const conversations = conversationsQuery.data?.data ?? []

  const scrollRef = useRef<HTMLDivElement>(null)
  const scrollY = useMotionValue(0)
  const virtualizer = useVirtualizer({
    count: conversations.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => CONVERSATION_CELL_HEIGHT,
    overscan: 8,
  })

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
      <div ref={scrollRef} onScroll={(e) => scrollY.set(e.currentTarget.scrollTop)} className="h-full overflow-y-auto">
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
          <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const conversation = conversations[virtualRow.index]
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
                  <ConversationCell
                    conversation={conversation}
                    onPress={() => handleSelectConversation(conversation.id)}
                  />
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
