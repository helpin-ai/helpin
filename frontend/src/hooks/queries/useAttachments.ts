import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmAttachmentService } from '@/lib/services/pmAttachmentService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateAttachmentRequest } from '@/lib/pmTypes'

export function useAttachments(wsId: string, entityType: string, entityId: string) {
  return useQuery({
    queryKey: queryKeys.pm.attachments(wsId, entityId),
    queryFn: async () => unwrap(await pmAttachmentService.list(wsId, entityType, entityId)),
    enabled: !!wsId && !!entityId,
  })
}

export function useInitiateUpload(wsId: string, entityId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateAttachmentRequest) =>
      unwrap(await pmAttachmentService.initiateUpload(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.attachments(wsId, entityId) })
    },
  })
}

export function useConfirmUpload(wsId: string, entityId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmAttachmentService.confirmUpload(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.attachments(wsId, entityId) })
    },
  })
}

export function useDeleteAttachment(wsId: string, entityId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmAttachmentService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.attachments(wsId, entityId) })
    },
  })
}
