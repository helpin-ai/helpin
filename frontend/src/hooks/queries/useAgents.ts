import { useQuery } from '@tanstack/react-query'
import { automationService } from '@/lib/services/automationService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useAgents(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.automation.agents(workspaceId),
    queryFn: async () => unwrap(await automationService.listAgents(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  })
}
