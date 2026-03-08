import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmViewService } from '@/lib/services/pmViewService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useViews(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.views(wsId),
    queryFn: async () => unwrap(await pmViewService.list(wsId)),
    enabled: !!wsId,
  })
}

export function useCreateView(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: { name: string; filters: Record<string, string>; is_shared: boolean; is_pinned: boolean }) =>
      unwrap(await pmViewService.create(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.views(wsId) })
    },
  })
}

export function useUpdateView(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: { id: string; name?: string; filters?: Record<string, string>; is_shared?: boolean; is_pinned?: boolean }) =>
      unwrap(await pmViewService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.views(wsId) })
    },
  })
}

export function useDeleteView(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmViewService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.views(wsId) })
    },
  })
}
