import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmAutomationService } from '@/lib/services/pmAutomationService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { UpsertAutomationRequest } from '@/lib/pmTypes'

export function useAutomations(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.automations(wsId),
    queryFn: async () => unwrap(await pmAutomationService.list(wsId)),
    enabled: !!wsId,
  })
}

export function useUpsertAutomation(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: UpsertAutomationRequest) =>
      unwrap(await pmAutomationService.upsert(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.automations(wsId) })
    },
  })
}

export function useDeleteAutomation(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ automationType, teamId }: { automationType: string; teamId?: string }) =>
      unwrap(await pmAutomationService.remove(wsId, automationType, teamId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.automations(wsId) })
    },
  })
}
