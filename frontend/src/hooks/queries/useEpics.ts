import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmEpicService } from '@/lib/services/pmEpicService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateEpicRequest, UpdateEpicRequest, UpdateEpicHealthRequest } from '@/lib/pmTypes'

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
  return useQuery({
    queryKey: queryKeys.pm.epic(wsId, id),
    queryFn: async () => unwrap(await pmEpicService.get(wsId, id)),
    enabled: !!wsId && !!id,
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
