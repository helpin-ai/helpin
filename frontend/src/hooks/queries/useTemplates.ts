import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmTaskTemplateService } from '@/lib/services/pmTaskTemplateService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateTaskTemplateRequest, UpdateTaskTemplateRequest } from '@/lib/pmTypes'

export function useTemplates(wsId: string, opts?: { teamId?: string; includeShared?: boolean; archived?: boolean }) {
  return useQuery({
    queryKey: [...queryKeys.pm.templates(wsId), opts],
    queryFn: async () => unwrap(await pmTaskTemplateService.list(wsId, opts)),
    enabled: !!wsId,
  })
}

export function useTemplate(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.pm.template(wsId, id),
    queryFn: async () => unwrap(await pmTaskTemplateService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useCreateTemplate(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateTaskTemplateRequest) =>
      unwrap(await pmTaskTemplateService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.templates(wsId) })
    },
  })
}

export function useUpdateTemplate(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateTaskTemplateRequest & { id: string }) =>
      unwrap(await pmTaskTemplateService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.templates(wsId) })
    },
  })
}

export function useDeleteTemplate(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmTaskTemplateService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.templates(wsId) })
    },
  })
}
