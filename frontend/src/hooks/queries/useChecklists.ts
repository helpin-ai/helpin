import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmChecklistService } from '@/lib/services/pmChecklistService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateChecklistItemRequest, UpdateChecklistItemRequest } from '@/lib/pmTypes'

export function useChecklists(wsId: string, storyId: string) {
  return useQuery({
    queryKey: queryKeys.pm.checklists(wsId, storyId),
    queryFn: async () => unwrap(await pmChecklistService.list(wsId, storyId)),
    enabled: !!wsId && !!storyId,
  })
}

export function useCreateChecklistItem(wsId: string, storyId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateChecklistItemRequest) =>
      unwrap(await pmChecklistService.create(wsId, storyId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.checklists(wsId, storyId) })
    },
  })
}

export function useUpdateChecklistItem(wsId: string, storyId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateChecklistItemRequest & { id: string }) =>
      unwrap(await pmChecklistService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.checklists(wsId, storyId) })
    },
  })
}

export function useDeleteChecklistItem(wsId: string, storyId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmChecklistService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.checklists(wsId, storyId) })
    },
  })
}
