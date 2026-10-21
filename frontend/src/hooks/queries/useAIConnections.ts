import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef } from 'react'
import { aiConnectionService } from '@/lib/services/aiConnectionService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap, unwrapRequired } from '@/lib/queryUtils'
import { invalidateCapabilities } from './useCapabilities'

function invalidateAI(queryClient: ReturnType<typeof useQueryClient>, workspaceId: string) {
  queryClient.invalidateQueries({ queryKey: queryKeys.ai.root(workspaceId) })
}

export function useAIConnections(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.ai.connections(workspaceId),
    queryFn: async () => unwrap(await aiConnectionService.list(workspaceId)),
    enabled: Boolean(workspaceId),
    retry: false,
  })
}

export function useAIModelEndpoints(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.ai.endpoints(workspaceId),
    queryFn: async () => unwrap(await aiConnectionService.endpoints(workspaceId)) ?? [],
    enabled: Boolean(workspaceId) && enabled,
    staleTime: 60_000,
    retry: false,
  })
}

export function useCreateAIConnection(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (request: {
      name: string
      provider: string
      api_key?: string
      scope?: 'personal' | 'workspace'
      endpoint_id?: string
    }) => unwrap(await aiConnectionService.create(workspaceId, request)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

export function useReconnectAIConnection(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, apiKey }: { id: string; apiKey?: string }) =>
      unwrap(await aiConnectionService.reconnect(workspaceId, id, apiKey)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

export function usePollAIConnection(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await aiConnectionService.poll(workspaceId, id)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

/**
 * Runs a live completion through a connection. Settles by refreshing AI
 * connections and workspace capabilities, which record the test result.
 */
export function useTestAIConnection(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ connectionId, model }: { connectionId: string; model?: string }) =>
      unwrapRequired(await aiConnectionService.test(workspaceId, connectionId, model), 'AI connection test'),
    onSettled: () => {
      invalidateAI(queryClient, workspaceId)
      invalidateCapabilities(queryClient, workspaceId)
    },
  })
}

export function useDisconnectAIConnection(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await aiConnectionService.disconnect(workspaceId, id)),
    onSuccess: () => invalidateAI(queryClient, workspaceId),
  })
}

const connectionModelsKey = (workspaceId: string, id: string) => [...queryKeys.ai.root(workspaceId), 'models', id] as const
export function useConnectionModels(workspaceId: string, id: string, enabled = true, { refreshOnOpen = false }: { refreshOnOpen?: boolean } = {}) {
  const refreshedConnection = useRef<string | null>(null)
  return useQuery({
    queryKey: connectionModelsKey(workspaceId, id),
    queryFn: async () => {
      const connectionKey = `${workspaceId}/${id}`
      const refresh = refreshOnOpen && refreshedConnection.current !== connectionKey
      // Refresh once per opening; saving visibility or model settings should
      // invalidate the local list without repeatedly contacting the provider.
      if (refresh) refreshedConnection.current = connectionKey
      return unwrapRequired(await (refresh
        ? aiConnectionService.refreshModels(workspaceId, id)
        : aiConnectionService.models(workspaceId, id)), 'Connection models')
    },
    enabled: Boolean(workspaceId && id) && enabled,
    refetchOnMount: refreshOnOpen ? 'always' : true,
    staleTime: 5 * 60_000,
    refetchInterval: 6 * 60 * 60_000,
    retry: false,
  })
}
export function useRefreshConnectionModels(workspaceId: string, id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => unwrapRequired(await aiConnectionService.refreshModels(workspaceId, id), 'Connection models'),
    onSuccess: data => queryClient.setQueryData(connectionModelsKey(workspaceId, id), data),
  })
}
