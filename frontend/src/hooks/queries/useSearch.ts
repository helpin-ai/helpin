import { useQuery } from '@tanstack/react-query'
import { searchService } from '@/lib/services/searchService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useSearch(wsId: string, query: string) {
  return useQuery({
    queryKey: queryKeys.pm.search(wsId, query),
    queryFn: async () => unwrap(await searchService.search(wsId, query)),
    enabled: !!wsId && query.length >= 2,
    staleTime: 10_000,
  })
}
