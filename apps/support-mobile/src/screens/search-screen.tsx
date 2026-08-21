import { useDeferredValue, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, useRouter } from '@tanstack/react-router'
import { Search, SlidersHorizontal, X } from 'lucide-react'
import {
  flattenConversationSearchPages,
  hasSupportConversationSearchInput,
  useInfiniteConversationSearch,
  type SupportConversationSearchResult,
  type SupportSearchHighlight,
} from '@helpin-ai/support-core'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { useWorkspacePermissions } from '@mobile/lib/use-workspace-permissions'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { displayNameFor } from '@mobile/inbox/conversation-cell'
import { formatRelativeTime } from '@mobile/inbox/inbox-helpers'
import { getAvatarColor } from '@/components/support/helpers'
import { PrimaryNavigation } from '@mobile/navigation/primary-navigation'
import { SearchFilterSheet } from '@mobile/search/search-filter-sheet'
import {
  EMPTY_CONVERSATION_SEARCH_FILTERS,
  activeConversationSearchFilterCount,
  buildConversationSearchParams,
} from '@mobile/search/search-filters'
import { Avatar } from '@mobile/ui/avatar'
import { EmptyState } from '@mobile/ui/empty-state'
import { OfflineBanner } from '@mobile/ui/offline-banner'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { TopBar } from '@mobile/ui/top-bar'

const FIELD_LABELS: Record<string, string> = {
  title: 'Title',
  customer_email: 'Email',
  customer_name: 'Customer',
  display_id: 'Number',
  message: 'Message',
}

const STATUS_LABELS: Record<string, string> = {
  open: 'Open',
  waiting_on_customer: 'Waiting',
  resolved: 'Resolved',
  spam: 'Spam',
}

const PRIORITY_LABELS: Record<string, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  urgent: 'Urgent',
}

export function highlightSearchText(text: string, ranges: SupportSearchHighlight['ranges']): ReactNode {
  if (ranges.length === 0) return text
  const parts: ReactNode[] = []
  let cursor = 0

  ranges
    .slice()
    .sort((a, b) => a.start - b.start)
    .forEach((range, index) => {
      const start = Math.max(cursor, Math.min(text.length, range.start))
      const end = Math.max(start, Math.min(text.length, range.end))
      if (start > cursor) parts.push(text.slice(cursor, start))
      if (end > start) {
        parts.push(
          <mark
            key={`${start}-${end}-${index}`}
            className="rounded-sm bg-amber-200/80 px-0.5 text-foreground dark:bg-amber-500/30"
          >
            {text.slice(start, end)}
          </mark>,
        )
      }
      cursor = end
    })
  if (cursor < text.length) parts.push(text.slice(cursor))
  return parts.length > 0 ? parts : text
}

function resultHighlight(result: SupportConversationSearchResult) {
  const primary = result.highlights[0]
  if (primary) return highlightSearchText(primary.text, primary.ranges)
  return result.snippet || result.conversation.last_message || result.conversation.subject
}

function fieldSummary(fields: string[]) {
  if (fields.length === 0) return 'Filter match'
  return fields.map((field) => FIELD_LABELS[field] ?? field).join(', ')
}

export function SearchScreen() {
  const { slug } = useParams({ strict: false })
  const router = useRouter()
  const setCurrentWorkspace = useWorkspaceStore((state) => state.setCurrentWorkspace)
  const [query, setQuery] = useState('')
  const [filters, setFilters] = useState(EMPTY_CONVERSATION_SEARCH_FILTERS)
  const [filtersOpen, setFiltersOpen] = useState(false)
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
  const { accessQuery, canReadSupport } = useWorkspacePermissions(workspaceId)
  const supportWorkspaceId = canReadSupport ? workspaceId : ''
  const accessDenied = accessQuery.isSuccess && !canReadSupport
  const searchParams = useMemo(
    () => buildConversationSearchParams(deferredQuery, filters),
    [deferredQuery, filters],
  )
  const hasSearchInput = hasSupportConversationSearchInput(searchParams)
  const filterCount = activeConversationSearchFilterCount(filters)

  useEffect(() => {
    if (workspace) {
      setCurrentWorkspace({ id: workspace.id, slug: workspace.slug, name: workspace.name })
    }
  }, [workspace, setCurrentWorkspace])

  const resultsQuery = useInfiniteConversationSearch(
    supportWorkspaceId,
    searchParams,
    hasSearchInput,
  )
  const results = useMemo(
    () => flattenConversationSearchPages(resultsQuery.data),
    [resultsQuery.data],
  )
  const firstPage = resultsQuery.data?.pages[0]
  const totalResults = firstPage?.total ?? results.length
  const totalCapped = firstPage?.meta.total_capped ?? false
  const hasInitialError =
    workspaceQuery.isError ||
    accessQuery.isError ||
    (resultsQuery.isError && resultsQuery.data === undefined)

  return (
    <div className="flex h-dvh flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto">
        <TopBar title="Search" subtitle={workspace?.name} />
        <OfflineBanner />

        <div className="sticky top-[calc(52px+var(--safe-top))] z-20 border-b border-border/70 bg-background/95 px-4 pb-3 pt-3 backdrop-blur-sm">
          <div className="flex gap-2">
            <label className="flex h-12 min-w-0 flex-1 items-center gap-2 rounded-2xl border border-input bg-muted/70 px-3.5 transition-colors focus-within:border-primary/50 focus-within:bg-background focus-within:ring-2 focus-within:ring-primary/20">
              <Search className="h-5 w-5 shrink-0 text-muted-foreground" />
              <input
                autoFocus
                disabled={accessDenied}
                type="search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                aria-label="Search conversations"
                placeholder="Email, #number, title, customer, or message"
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
            <Pressable
              aria-label={filterCount > 0 ? `Search filters, ${filterCount} active` : 'Search filters'}
              disabled={accessDenied || !workspaceId}
              onPress={() => setFiltersOpen(true)}
              className="relative flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-input bg-background text-foreground active:bg-muted"
            >
              <SlidersHorizontal className="h-5 w-5" />
              {filterCount > 0 && (
                <span className="absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold text-primary-foreground">
                  {filterCount > 9 ? '9+' : filterCount}
                </span>
              )}
            </Pressable>
          </div>
          {hasSearchInput && !hasInitialError && (
            <div className="mt-2 flex items-center justify-between px-1 text-footnote text-muted-foreground">
              <span>{totalResults}{totalCapped ? '+' : ''} result{totalResults === 1 ? '' : 's'}</span>
              {resultsQuery.isFetching && !resultsQuery.isFetchingNextPage && <span>Searching…</span>}
            </div>
          )}
        </div>

        {accessDenied && (
          <EmptyState
            icon={<Search className="h-6 w-6" />}
            title="Support access unavailable"
            body="Ask a workspace admin to grant you access to the Support module."
          />
        )}

        {!accessDenied && !hasInitialError && !hasSearchInput && (
          <EmptyState
            icon={<Search className="h-6 w-6" />}
            title="Search conversations"
            body="Find customers, subjects, conversation numbers, and message content—or search with filters."
          />
        )}

        {!accessDenied && hasSearchInput && (workspaceQuery.isPending || accessQuery.isPending || resultsQuery.isPending) && (
          <div className="flex min-h-48 items-center justify-center"><Spinner /></div>
        )}

        {!accessDenied && hasInitialError && (
          <EmptyState
            icon={<Search className="h-6 w-6" />}
            title="Couldn't search conversations"
            body="Check your connection and try again."
          />
        )}

        {!accessDenied && hasSearchInput && !resultsQuery.isPending && !hasInitialError && results.length === 0 && (
          <EmptyState
            icon={<Search className="h-6 w-6" />}
            title="No results"
            body="Try a broader query or remove a filter."
          />
        )}

        {!accessDenied && hasSearchInput && results.length > 0 && (
          <div aria-live="polite" aria-label={`${results.length} of ${totalResults} search results`}>
            {results.map((result) => {
              const conversation = result.conversation
              const displayName = displayNameFor(conversation)
              return (
                <Pressable
                  key={conversation.id}
                  onPress={() => router.navigate({
                    to: '/w/$slug/support/$conversationId',
                    params: { slug: slug ?? '', conversationId: conversation.id },
                  })}
                  className="flex h-auto min-h-[116px] w-full items-start gap-3 rounded-none border-b border-border/60 bg-background px-4 py-3 text-left active:bg-muted/50"
                >
                  <Avatar
                    name={displayName}
                    size={42}
                    className={getAvatarColor(conversation.customer_email || conversation.customer_name || conversation.id)}
                  />
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="shrink-0 font-mono text-[11px] text-muted-foreground">#{result.display_id}</span>
                      <span className="truncate text-body font-semibold text-foreground">{conversation.subject}</span>
                      <span className="ml-auto shrink-0 text-footnote text-muted-foreground">{formatRelativeTime(conversation.updated_at)}</span>
                    </div>
                    <div className="truncate text-footnote text-muted-foreground">
                      {displayName}{conversation.customer_name && conversation.customer_email ? ` · ${conversation.customer_email}` : ''}
                    </div>
                    <p className="line-clamp-2 text-footnote text-foreground/85">{resultHighlight(result)}</p>
                    <div className="flex flex-wrap items-center gap-1.5 pt-0.5 text-[11px] text-muted-foreground">
                      <span>{fieldSummary(result.matched_fields)}</span>
                      <span aria-hidden="true">·</span>
                      <span className="rounded-full bg-muted px-1.5 py-0.5 font-medium text-foreground/80">
                        {STATUS_LABELS[conversation.status] ?? conversation.status}
                      </span>
                      <span className="rounded-full border border-border px-1.5 py-0.5">
                        {PRIORITY_LABELS[conversation.priority] ?? conversation.priority}
                      </span>
                    </div>
                  </div>
                </Pressable>
              )
            })}
            {resultsQuery.hasNextPage && (
              <div className="flex justify-center px-4 py-4">
                <Pressable
                  haptic="impactLight"
                  disabled={resultsQuery.isFetchingNextPage}
                  onPress={() => void resultsQuery.fetchNextPage()}
                  className="flex min-h-10 items-center justify-center rounded-full bg-muted px-5 text-footnote font-medium text-foreground active:bg-muted/70 disabled:opacity-60"
                >
                  {resultsQuery.isFetchingNextPage ? <Spinner size={16} /> : resultsQuery.isFetchNextPageError ? 'Retry loading more' : `Load more (${Math.max(0, totalResults - results.length)} remaining)`}
                </Pressable>
              </div>
            )}
          </div>
        )}
      </div>

      <SearchFilterSheet
        open={filtersOpen}
        onOpenChange={setFiltersOpen}
        workspaceId={workspaceId}
        filters={filters}
        onApply={setFilters}
      />
      <PrimaryNavigation activeTab="search" workspaceId={workspaceId} workspaceSlug={slug ?? ''} />
    </div>
  )
}
