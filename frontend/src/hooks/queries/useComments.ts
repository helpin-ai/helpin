import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmCommentService } from '@/lib/services/pmCommentService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateCommentRequest, UpdateCommentRequest } from '@/lib/pmTypes'

export function useComments(wsId: string, entityType: 'task' | 'epic' | 'doc', entityId: string) {
  return useQuery({
    queryKey: queryKeys.pm.comments(wsId, entityId),
    queryFn: async () => unwrap(await pmCommentService.list(wsId, entityType, entityId)),
    enabled: !!wsId && !!entityId,
  })
}

export function useCreateComment(wsId: string, entityId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCommentRequest) => unwrap(await pmCommentService.create(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.comments(wsId, entityId) })
    },
  })
}

export function useUpdateComment(wsId: string, entityId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCommentRequest & { id: string }) =>
      unwrap(await pmCommentService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.comments(wsId, entityId) })
    },
  })
}

export function useDeleteComment(wsId: string, entityId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmCommentService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.comments(wsId, entityId) })
    },
  })
}
