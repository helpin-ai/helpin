import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { aiProfileService, type AIProfile, type SaveAIProfile } from '@/lib/services/aiProfileService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

function invalidateAI(queryClient: ReturnType<typeof useQueryClient>, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: queryKeys.ai.root(workspaceId) })
}

export function useAIProfiles(workspaceId: string, options: { enabled?: boolean; staleTime?: number } = {}) {
  const { enabled = true, staleTime } = options
  return useQuery({
    queryKey: queryKeys.ai.profiles(workspaceId),
    queryFn: async () => unwrap(await aiProfileService.list(workspaceId)) ?? [],
    enabled: Boolean(workspaceId) && enabled,
    staleTime,
    retry: false,
  })
}

export function useAISettings(workspaceId: string, options: { enabled?: boolean } = {}) {
  const { enabled = true } = options
  return useQuery({
    queryKey: queryKeys.ai.settings(workspaceId),
    queryFn: async () => unwrap(await aiProfileService.settings(workspaceId)),
    enabled: Boolean(workspaceId) && enabled,
    retry: false,
  })
}

export function useSaveAIProfile(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ value, id }: { value: SaveAIProfile; id?: string }) =>
      unwrap(await aiProfileService.save(workspaceId, value, id)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

export function useDeleteAIProfile(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (profile: AIProfile) => unwrap(await aiProfileService.remove(workspaceId, profile)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

export function useSetDefaultAIProfile(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (profileId: string | null) =>
      unwrap(await aiProfileService.setDefault(workspaceId, profileId)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

export function useSetAIModelVisibility(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ profile, hidden }: { profile: AIProfile; hidden: boolean }) =>
      unwrap(await aiProfileService.setVisibility(workspaceId, profile, hidden)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

export function useEnableAIModel(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (value: {connection_id: string; model: string; name: string}) => unwrap(await aiProfileService.enableModel(workspaceId, value)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}
