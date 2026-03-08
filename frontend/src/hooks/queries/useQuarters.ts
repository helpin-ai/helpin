import { useQuery } from '@tanstack/react-query'
import { rewardQuartersService } from '@/lib/services/quartersService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useQuarters(wsId: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.quarters(wsId),
    queryFn: async () => unwrap(await rewardQuartersService.list(wsId)),
    enabled: !!wsId,
    staleTime: 5 * 60_000,
  })
}
