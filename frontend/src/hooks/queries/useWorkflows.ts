import { useQuery } from '@tanstack/react-query'
import { pmWorkflowService } from '@/lib/services/pmWorkflowService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useWorkflows(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.workflows(wsId),
    queryFn: async () => unwrap(await pmWorkflowService.list(wsId)),
    enabled: !!wsId,
  })
}

export function useEpicStates(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.epicStates(wsId),
    queryFn: async () => unwrap(await pmWorkflowService.listEpicStates(wsId)),
    enabled: !!wsId,
  })
}
