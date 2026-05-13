import { useQuery } from '@tanstack/react-query'
import { agentService } from '@/lib/services/agentService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { Agent } from '@/lib/pmTypes'

export function useAgents(workspaceId: string) {
  return useQuery<Agent[]>({
    queryKey: queryKeys.automation.agents(workspaceId),
    queryFn: async () => {
      const payload = unwrap(await agentService.list(workspaceId))
      // Backend returns [] for empty but guard against a `null` JSON body too
      // so consumers can always call `.filter`/`.map` without checking.
      return Array.isArray(payload) ? payload : []
    },
    enabled: !!workspaceId,
    staleTime: 60_000,
  })
}
