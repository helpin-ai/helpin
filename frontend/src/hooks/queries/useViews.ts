import { useQuery } from '@tanstack/react-query'
import { pmViewService } from '@/lib/services/pmViewService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useViews(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.views(wsId),
    queryFn: async () => unwrap(await pmViewService.list(wsId)),
    enabled: !!wsId,
  })
}
