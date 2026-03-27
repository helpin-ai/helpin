import { useQuery } from '@tanstack/react-query'
import { agentService } from '@/lib/services/agentService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useAgents(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.agents.all(workspaceId),
    queryFn: async () => unwrap(await agentService.list(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  })
}
