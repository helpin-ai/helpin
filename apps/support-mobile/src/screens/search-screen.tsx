import { useDeferredValue, useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, useRouter } from '@tanstack/react-router'
import { Search, X } from 'lucide-react'
import { useConversations } from '@helpin-ai/support-core'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { ConversationCell } from '@mobile/inbox/conversation-cell'
import { PrimaryNavigation } from '@mobile/navigation/primary-navigation'
import { EmptyState } from '@mobile/ui/empty-state'
import { OfflineBanner } from '@mobile/ui/offline-banner'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { TopBar } from '@mobile/ui/top-bar'

export function SearchScreen() {
  const { slug } = useParams({ strict: false })
  const router = useRouter()
  const setCurrentWorkspace = useWorkspaceStore((state) => state.setCurrentWorkspace)
  const [query, setQuery] = useState('')
  const deferredQuery = useDeferredValue(query.trim())

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
      setCurrentWorkspace({
        id: workspace.id,
        slug: workspace.slug,
        name: workspace.name,
      })
    }
  }, [workspace, setCurrentWorkspace])

  // Passing an empty workspace id keeps the query disabled until the user has
  // entered something, avoiding an unnecessary full-inbox fetch on this tab.
  const resultsQuery = useConversations(
    deferredQuery ? workspaceId : '',
    deferredQuery ? { search: deferredQuery } : undefined,
    true,
  )
  const conversations = resultsQuery.data?.data ?? []

  return (
    <div className="flex h-dvh flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto">
        <TopBar title="Search" subtitle={workspace?.name} />
        <OfflineBanner />

        <div className="sticky top-[calc(52px+var(--safe-top))] z-20 border-b border-border bg-background px-4 pb-3">
          <label className="flex h-11 items-center gap-2 rounded-xl bg-muted px-3 focus-within:ring-2 focus-within:ring-primary/40">
            <Search className="h-5 w-5 shrink-0 text-muted-foreground" />
            <input
              autoFocus
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              aria-label="Search conversations"
              placeholder="Search conversations"
              className="min-w-0 flex-1 bg-transparent text-body text-foreground outline-none placeholder:text-muted-foreground"
            />
            {query && (
              <Pressable
                aria-label="Clear search"
                onPress={() => setQuery('')}
                className="flex items-center justify-center rounded-full text-muted-foreground"
              >
                <X className="h-4 w-4" />
              </Pressable>
            )}
          </label>
        </div>

        {!deferredQuery && (
          <EmptyState
            icon={<Search className="h-6 w-6" />}
            title="Search conversations"
            body="Find customers, subjects, and message content."
          />
        )}

        {deferredQuery && (workspaceQuery.isPending || resultsQuery.isPending) && (
          <div className="flex min-h-48 items-center justify-center">
            <Spinner />
          </div>
        )}

        {deferredQuery && (workspaceQuery.isError || resultsQuery.isError) && (
          <EmptyState
            icon={<Search className="h-6 w-6" />}
            title="Couldn't search conversations"
            body="Check your connection and try again."
          />
        )}

        {deferredQuery && !resultsQuery.isPending && !resultsQuery.isError && conversations.length === 0 && (
          <EmptyState
            icon={<Search className="h-6 w-6" />}
            title="No results"
            body={`No conversations match “${deferredQuery}”.`}
          />
        )}

        {deferredQuery && conversations.length > 0 && (
          <div aria-live="polite" aria-label={`${conversations.length} search results`}>
            {conversations.map((conversation) => (
              <ConversationCell
                key={conversation.id}
                conversation={conversation}
                onPress={() =>
                  router.navigate({
                    to: '/w/$slug/support/$conversationId',
                    params: {
                      slug: slug ?? '',
                      conversationId: conversation.id,
                    },
                  })
                }
              />
            ))}
          </div>
        )}
      </div>

      <PrimaryNavigation activeTab="search" workspaceId={workspaceId} workspaceSlug={slug ?? ''} />
    </div>
  )
}
