import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateExternalLinkRequest, UpdateExternalLinkRequest } from '@/lib/pmTypes'

export function useExternalLinks(wsId: string, storyId: string) {
  return useQuery({
    queryKey: queryKeys.pm.externalLinks(wsId, storyId),
    queryFn: async () => unwrap(await pmExternalLinkService.list(wsId, storyId)),
    enabled: !!wsId && !!storyId,
  })
}

export function useCreateExternalLink(wsId: string, storyId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateExternalLinkRequest) =>
      unwrap(await pmExternalLinkService.create(wsId, storyId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(wsId, storyId) })
    },
  })
}

export function useUpdateExternalLink(wsId: string, storyId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateExternalLinkRequest & { id: string }) =>
      unwrap(await pmExternalLinkService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(wsId, storyId) })
    },
  })
}

export function useDeleteExternalLink(wsId: string, storyId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmExternalLinkService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(wsId, storyId) })
    },
  })
}
