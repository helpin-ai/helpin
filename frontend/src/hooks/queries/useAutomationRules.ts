import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { automationRuleService } from '@/lib/services/automationRuleService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateAutomationRuleRequest, UpdateAutomationRuleRequest } from '@/lib/pmTypes'

export function useAutomationRules(wsId?: string) {
  return useQuery({
    queryKey: queryKeys.pm.automationRules(wsId!),
    queryFn: async () => unwrap(await automationRuleService.list(wsId!)),
    enabled: !!wsId,
  })
}

export function useAutomationRulesByWorkflow(wsId?: string, workflowId?: string) {
  return useQuery({
    queryKey: queryKeys.pm.automationRulesByWorkflow(wsId!, workflowId!),
    queryFn: async () => unwrap(await automationRuleService.listByWorkflow(wsId!, workflowId!)),
    enabled: !!wsId && !!workflowId,
  })
}

export function useCreateAutomationRule(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateAutomationRuleRequest) =>
      unwrap(await automationRuleService.create(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.automationRules(wsId) })
    },
  })
}

export function useUpdateAutomationRule(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ ruleId, data }: { ruleId: string; data: UpdateAutomationRuleRequest }) =>
      unwrap(await automationRuleService.update(wsId, ruleId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.automationRules(wsId) })
    },
  })
}

export function useDeleteAutomationRule(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (ruleId: string) =>
      unwrap(await automationRuleService.remove(wsId, ruleId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.automationRules(wsId) })
    },
  })
}
