import { useQuery } from '@tanstack/react-query'
import { pmChecklistService } from '@/lib/services/pmChecklistService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useChecklists(wsId: string, taskId: string) {
  return useQuery({
    queryKey: queryKeys.pm.checklists(wsId, taskId),
    queryFn: async () => unwrap(await pmChecklistService.list(wsId, taskId)),
    enabled: !!wsId && !!taskId,
  })
}
