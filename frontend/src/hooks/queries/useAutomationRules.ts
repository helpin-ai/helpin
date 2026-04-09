import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { automationService } from '@/lib/services/automationService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateAutomationRuleRequest, UpdateAutomationRuleRequest } from '@/lib/pmTypes'

export function useAutomationRules(wsId?: string) {
  return useQuery({
    queryKey: queryKeys.automation.flows(wsId!),
    queryFn: async () => unwrap(await automationService.listFlows(wsId!)),
    enabled: !!wsId,
  })
}

export function useAutomationRulesByWorkflow(wsId?: string, workflowId?: string) {
  return useQuery({
    queryKey: queryKeys.automation.flowsByWorkflow(wsId!, workflowId!),
    queryFn: async () => unwrap(await automationService.listFlowsByWorkflow(wsId!, workflowId!)),
    enabled: !!wsId && !!workflowId,
  })
}

export function useCreateAutomationRule(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateAutomationRuleRequest) =>
      unwrap(await automationService.createFlow(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) })
    },
  })
}

export function useUpdateAutomationRule(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ ruleId, data }: { ruleId: string; data: UpdateAutomationRuleRequest }) =>
      unwrap(await automationService.updateFlow(wsId, ruleId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) })
    },
  })
}

export function useDeleteAutomationRule(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (ruleId: string) =>
      unwrap(await automationService.deleteFlow(wsId, ruleId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) })
    },
  })
}
