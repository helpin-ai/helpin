import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { automationService } from '@/lib/services/automationService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type {
  AutomationInventoryResponse,
  AutomationTriggerExecutionFilters,
  AutomationTriggerExecutionListResponse,
} from '@/lib/types';
import type {
  CreateAgentFromTemplateRequest,
  CreateAgentRequest,
  CreateAutomationRuleRequest,
  InstallFlowTemplateRequest,
  UninstallFlowTemplateRequest,
  UpdateAgentRequest,
  UpdateAutomationRuleRequest,
} from '@/lib/pmTypes';

// The backend sometimes emits JSON `null` for empty slices (Go `nil` slice →
// `null` when encoded without explicit `if x == nil { x = []T{} }`). Coerce
// every list field here so every consumer sees arrays, not null/undefined.
function sanitizeOverview(payload: AutomationInventoryResponse | null | undefined): AutomationInventoryResponse {
  return {
    groups: Array.isArray(payload?.groups) ? payload!.groups : [],
    items: Array.isArray(payload?.items) ? payload!.items : [],
    trigger_catalog: Array.isArray(payload?.trigger_catalog) ? payload!.trigger_catalog : [],
    generated_at: payload?.generated_at ?? '',
  };
}

function sanitizeActivity(payload: AutomationTriggerExecutionListResponse | null | undefined): AutomationTriggerExecutionListResponse {
  return {
    data: Array.isArray(payload?.data) ? payload!.data : [],
    total: payload?.total ?? 0,
    page: payload?.page ?? 1,
    per_page: payload?.per_page ?? 0,
    total_pages: payload?.total_pages ?? 0,
  };
}

export function useAutomationOverview(wsId: string, enabled = true) {
  return useQuery<AutomationInventoryResponse>({
    queryKey: queryKeys.automation.overview(wsId),
    queryFn: async () => sanitizeOverview(unwrap(await automationService.getOverview(wsId))),
    enabled: !!wsId && enabled,
    staleTime: 60_000,
  });
}

export function useAutomationActivity(wsId: string, filters: AutomationTriggerExecutionFilters, enabled = true) {
  return useQuery<AutomationTriggerExecutionListResponse>({
    queryKey: queryKeys.automation.activity(wsId, filters as Record<string, unknown>),
    queryFn: async () => sanitizeActivity(unwrap(await automationService.listActivity(wsId, filters))),
    enabled: !!wsId && enabled,
    placeholderData: (previousData) => previousData,
    staleTime: 30_000,
  });
}

export function useAutomationFlows(wsId?: string) {
  return useQuery({
    queryKey: queryKeys.automation.flows(wsId!),
    queryFn: async () => unwrap(await automationService.listFlows(wsId!)),
    enabled: !!wsId,
    staleTime: 30_000,
  });
}

export function useAutomationFlowsByWorkflow(wsId?: string, workflowId?: string) {
  return useQuery({
    queryKey: queryKeys.automation.flowsByWorkflow(wsId!, workflowId!),
    queryFn: async () => unwrap(await automationService.listFlowsByWorkflow(wsId!, workflowId!)),
    enabled: !!wsId && !!workflowId,
    staleTime: 30_000,
  });
}

export function useAutomationFlowTemplates(wsId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.automation.flowTemplates(wsId),
    queryFn: async () => unwrap(await automationService.listFlowTemplates(wsId)),
    enabled: !!wsId && enabled,
    staleTime: 60_000,
  });
}

export function useInstallAutomationFlowTemplate(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ templateKey, payload }: { templateKey: string; payload: InstallFlowTemplateRequest }) =>
      unwrap(await automationService.installFlowTemplate(wsId, templateKey, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.agents(wsId) });
    },
  });
}

export function useUninstallAutomationFlowTemplate(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ instanceId, payload }: { instanceId: string; payload: UninstallFlowTemplateRequest }) =>
      unwrap(await automationService.uninstallFlowTemplate(wsId, instanceId, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.agents(wsId) });
    },
  });
}

export function useCreateAutomationFlow(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateAutomationRuleRequest) => unwrap(await automationService.createFlow(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) });
    },
  });
}

export function useUpdateAutomationFlow(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ flowId, data }: { flowId: string; data: UpdateAutomationRuleRequest }) =>
      unwrap(await automationService.updateFlow(wsId, flowId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) });
    },
  });
}

export function useDeleteAutomationFlow(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (flowId: string) => unwrap(await automationService.deleteFlow(wsId, flowId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) });
    },
  });
}

export function useAutomationTriggerCatalog(wsId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.automation.triggerCatalog(wsId),
    queryFn: async () => unwrap(await automationService.listTriggerCatalog(wsId)),
    enabled: !!wsId && enabled,
    staleTime: 60_000,
  });
}

export function useAutomationToolCatalog(wsId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.automation.toolCatalog(wsId),
    queryFn: async () => unwrap(await automationService.listToolCatalog(wsId)),
    enabled: !!wsId && enabled,
    staleTime: 60_000,
  });
}

export function useAutomationAgents(wsId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.automation.agents(wsId),
    queryFn: async () => unwrap(await automationService.listAgents(wsId)),
    enabled: !!wsId && enabled,
    staleTime: 60_000,
  });
}

export function useAutomationAgentTemplates(wsId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.automation.agentTemplates(wsId),
    queryFn: async () => unwrap(await automationService.listAgentTemplates(wsId)),
    enabled: !!wsId && enabled,
    staleTime: 60_000,
  });
}

export function useAutomationAgentUsage(wsId: string, agentId: string | undefined | null, enabled = true) {
  return useQuery({
    queryKey: queryKeys.automation.agentUsage(wsId, agentId ?? ''),
    queryFn: async () => unwrap(await automationService.getAgentUsage(wsId, agentId!)),
    enabled: !!wsId && !!agentId && enabled,
    staleTime: 30_000,
  });
}

export function useCreateAutomationAgent(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: CreateAgentRequest) => unwrap(await automationService.createAgent(wsId, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.agents(wsId) });
    },
  });
}

export function useCreateAutomationAgentFromTemplate(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ templateId, payload }: { templateId: string; payload: CreateAgentFromTemplateRequest }) =>
      unwrap(await automationService.createAgentFromTemplate(wsId, templateId, payload)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.agents(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.flows(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.overview(wsId) });
    },
  });
}

export function useUpdateAutomationAgent(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ agentId, payload }: { agentId: string; payload: UpdateAgentRequest }) =>
      unwrap(await automationService.updateAgent(wsId, agentId, payload)),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.agents(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.agent(wsId, vars.agentId) });
      qc.invalidateQueries({ queryKey: queryKeys.automation.agentUsage(wsId, vars.agentId) });
    },
  });
}

export function useDeleteAutomationAgent(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (agentId: string) => unwrap(await automationService.deleteAgent(wsId, agentId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.agents(wsId) });
    },
  });
}
