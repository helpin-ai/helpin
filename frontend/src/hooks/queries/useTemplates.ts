import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmStoryTemplateService } from '@/lib/services/pmStoryTemplateService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateStoryTemplateRequest, UpdateStoryTemplateRequest } from '@/lib/pmTypes'

export function useTemplates(wsId: string, opts?: { teamId?: string; includeShared?: boolean; archived?: boolean }) {
  return useQuery({
    queryKey: [...queryKeys.pm.templates(wsId), opts],
    queryFn: async () => unwrap(await pmStoryTemplateService.list(wsId, opts)),
    enabled: !!wsId,
  })
}

export function useTemplate(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.pm.template(wsId, id),
    queryFn: async () => unwrap(await pmStoryTemplateService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useCreateTemplate(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateStoryTemplateRequest) =>
      unwrap(await pmStoryTemplateService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.templates(wsId) })
    },
  })
}

export function useUpdateTemplate(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateStoryTemplateRequest & { id: string }) =>
      unwrap(await pmStoryTemplateService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.templates(wsId) })
    },
  })
}

export function useDeleteTemplate(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmStoryTemplateService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.templates(wsId) })
    },
  })
}
