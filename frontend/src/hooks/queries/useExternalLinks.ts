import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateExternalLinkRequest, UpdateExternalLinkRequest } from '@/lib/pmTypes'

export function useExternalLinks(wsId: string, taskId: string) {
  return useQuery({
    queryKey: queryKeys.pm.externalLinks(wsId, taskId),
    queryFn: async () => unwrap(await pmExternalLinkService.list(wsId, taskId)),
    enabled: !!wsId && !!taskId,
  })
}

export function useCreateExternalLink(wsId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateExternalLinkRequest) =>
      unwrap(await pmExternalLinkService.create(wsId, taskId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(wsId, taskId) })
    },
  })
}

export function useUpdateExternalLink(wsId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateExternalLinkRequest & { id: string }) =>
      unwrap(await pmExternalLinkService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(wsId, taskId) })
    },
  })
}

export function useDeleteExternalLink(wsId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmExternalLinkService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(wsId, taskId) })
    },
  })
}
