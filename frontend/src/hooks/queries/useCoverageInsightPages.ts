import { useInfiniteQuery } from '@tanstack/react-query'
import type { CoverageInsightPage } from '@/lib/supportCoverageTypes'

type PageLoader<T> = (workspaceId: string, limit: number, offset: number) => Promise<{
  data: CoverageInsightPage<T> | null
  error: string | null
}>

const PAGE_SIZE = 25

export function useCoverageInsightPages<T extends { id: string }>(
  workspaceId: string,
  surface: 'topics' | 'signals',
  enabled: boolean,
  loadPage: PageLoader<T>,
) {
  const query = useInfiniteQuery({
    queryKey: ['supportCoverage', workspaceId, surface],
    enabled: Boolean(workspaceId) && enabled,
    initialPageParam: 0,
    queryFn: async ({ pageParam }) => {
      const response = await loadPage(workspaceId, PAGE_SIZE, pageParam)
      if (response.error || !response.data) {
        throw new Error(response.error || 'Could not load coverage results')
      }
      return response.data
    },
    getNextPageParam: (lastPage, _pages, offset) => {
      const count = lastPage.items?.length ?? 0
      const nextOffset = offset + count
      if (!count) return undefined
      return typeof lastPage.total === 'number'
        ? nextOffset < lastPage.total ? nextOffset : undefined
        : count === PAGE_SIZE ? nextOffset : undefined
    },
    staleTime: 30_000,
  })
  const items = enabled
    ? [...new Map(query.data?.pages.flatMap(page => page.items ?? []).map(item => [item.id, item])).values()]
    : []
  return {
    items,
    total: enabled ? query.data?.pages.at(-1)?.total : undefined,
    loading: enabled && query.isPending,
    error: query.isError && !query.isFetchNextPageError ? query.error.message : null,
    loadMoreError: query.isFetchNextPageError ? query.error?.message : null,
    hasMore: enabled && Boolean(query.hasNextPage),
    loadingMore: query.isFetchingNextPage,
    loadMore: query.fetchNextPage,
    refresh: query.refetch,
  }
}
