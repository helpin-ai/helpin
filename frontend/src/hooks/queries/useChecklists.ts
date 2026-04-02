import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmChecklistService } from '@/lib/services/pmChecklistService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateChecklistItemRequest, UpdateChecklistItemRequest } from '@/lib/pmTypes'

export function useChecklists(wsId: string, taskId: string) {
  return useQuery({
    queryKey: queryKeys.pm.checklists(wsId, taskId),
    queryFn: async () => unwrap(await pmChecklistService.list(wsId, taskId)),
    enabled: !!wsId && !!taskId,
  })
}

export function useCreateChecklistItem(wsId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateChecklistItemRequest) =>
      unwrap(await pmChecklistService.create(wsId, taskId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.checklists(wsId, taskId) })
    },
  })
}

export function useUpdateChecklistItem(wsId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateChecklistItemRequest & { id: string }) =>
      unwrap(await pmChecklistService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.checklists(wsId, taskId) })
    },
  })
}

export function useDeleteChecklistItem(wsId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmChecklistService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.checklists(wsId, taskId) })
    },
  })
}
