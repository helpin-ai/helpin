import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmEpicService } from '@/lib/services/pmEpicService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateEpicRequest, EpicWithStats, UpdateEpicRequest, UpdateEpicHealthRequest } from '@/lib/pmTypes'

interface EpicFilters {
  team_id?: string
  state_id?: string
  label_id?: string
  archived?: boolean
}

export function useEpics(wsId: string, filters?: EpicFilters) {
  return useQuery({
    queryKey: [...queryKeys.pm.epics(wsId), filters],
    queryFn: async () => unwrap(await pmEpicService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useEpic(wsId: string, id: string) {
  const queryClient = useQueryClient()
  return useQuery({
    queryKey: queryKeys.pm.epic(wsId, id),
    queryFn: async () => unwrap(await pmEpicService.get(wsId, id)),
    enabled: !!wsId && !!id,
    placeholderData: () => {
      const cachedLists = queryClient.getQueriesData<unknown>({ queryKey: queryKeys.pm.epics(wsId) })
      for (const [queryKey, value] of cachedLists) {
        const filters = queryKey[3]
        const isEpicList = queryKey.length === 4
          && (filters === undefined || (typeof filters === 'object' && filters !== null && !Array.isArray(filters)))
        if (!isEpicList) continue
        if (!Array.isArray(value)) continue
        const match = value.find((candidate: unknown) => {
          if (!candidate || typeof candidate !== 'object' || !('epic' in candidate)) return false
          const epic = (candidate as { epic?: { id?: string } }).epic
          return epic?.id === id
        })
        if (match) return match as EpicWithStats
      }
      return undefined
    },
  })
}

export function useEpicTasks(wsId: string, epicId: string) {
  return useQuery({
    queryKey: queryKeys.pm.epicTasks(wsId, epicId),
    queryFn: async () => unwrap(await pmEpicService.listTasks(wsId, epicId)),
    enabled: !!wsId && !!epicId,
  })
}

export function useCreateEpic(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateEpicRequest) => {
      const response = unwrap(await pmEpicService.create(data))
      return response.epic
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) })
    },
  })
}

export function useUpdateEpic(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateEpicRequest & { id: string }) =>
      unwrap(await pmEpicService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, id) })
    },
  })
}

export function useDeleteEpic(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmEpicService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) })
    },
  })
}

export function useUpdateEpicHealth(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateEpicHealthRequest & { id: string }) =>
      unwrap(await pmEpicService.updateHealth(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, id) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) })
    },
  })
}
