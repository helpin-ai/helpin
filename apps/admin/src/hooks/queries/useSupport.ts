import { useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { agentService } from '@/lib/services/agentService'
import { supportService } from '@/lib/services/supportService'

export function useChatSettings(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.installation(workspaceId),
    queryFn: async () => unwrap(await supportService.getInstallation(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  })
}

export function useSupportAgents(workspaceId: string) {
  return useQuery({
    queryKey: [...queryKeys.agents.all(workspaceId), 'support'] as const,
    queryFn: async () => {
      const response = await agentService.list(workspaceId)
      if (response.error) {
        throw new Error(response.error)
      }

      return (response.data ?? []).filter(
        (agent) =>
          agent.allowed_targets?.includes('support_conversation') ||
          agent.preset_key === 'support_agent',
      )
    },
    enabled: !!workspaceId,
    staleTime: 60_000,
  })
}
