import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmLabelService } from '@/lib/services/pmLabelService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateLabelRequest, UpdateLabelRequest } from '@/lib/pmTypes'

export function useLabels(wsId: string, opts?: { teamId?: string; includeShared?: boolean }) {
  return useQuery({
    queryKey: [...queryKeys.pm.labels(wsId), opts],
    queryFn: async () => unwrap(await pmLabelService.list(wsId, opts)),
    enabled: !!wsId,
  })
}

export function useLabelsWithStats(wsId: string, opts?: { teamId?: string; includeShared?: boolean; archived?: boolean }) {
  return useQuery({
    queryKey: [...queryKeys.pm.labelsWithStats(wsId), opts],
    queryFn: async () => unwrap(await pmLabelService.listWithStats(wsId, opts)),
    enabled: !!wsId,
  })
}

export function useCreateLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateLabelRequest) => unwrap(await pmLabelService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.labels(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.labelsWithStats(wsId) })
    },
  })
}

export function useUpdateLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateLabelRequest & { id: string }) =>
      unwrap(await pmLabelService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.labels(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.labelsWithStats(wsId) })
    },
  })
}

export function useDeleteLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmLabelService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.labels(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.labelsWithStats(wsId) })
    },
  })
}
