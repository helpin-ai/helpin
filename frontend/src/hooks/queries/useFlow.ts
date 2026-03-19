import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { flowService } from '@/lib/services/flowService';
import type {
  FlowRunView,
  CreateFlowTemplateRequest,
  UpdateFlowTemplateRequest,
  UpdateFlowTemplateNodeRequest,
  CreateFlowTemplateNodeRequest,
} from '@/lib/pmTypes';

function invalidateFlowTarget(qc: ReturnType<typeof useQueryClient>, wsId: string, run: FlowRunView['run']) {
  switch (run.target_type) {
    case 'epic':
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, run.target_id) });
      return;
    case 'story':
      qc.invalidateQueries({ queryKey: queryKeys.pm.stories(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, run.target_id) });
      return;
    case 'crm_deal':
      qc.invalidateQueries({ queryKey: queryKeys.crm.deals(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.crm.deal(wsId, run.target_id) });
      return;
    default:
      return;
  }
}

export function useFlowRun(wsId: string, flowRunId?: string) {
  return useQuery({
    queryKey: queryKeys.pm.flowRun(wsId, flowRunId ?? ''),
    queryFn: async () => unwrap(await flowService.getRun(wsId, flowRunId!)),
    enabled: !!wsId && !!flowRunId,
    refetchInterval: 5_000,
  });
}

export function useFlowNodeMessages(wsId: string, flowRunId?: string, nodeRunId?: string) {
  return useQuery({
    queryKey: queryKeys.pm.flowNodeMessages(wsId, flowRunId ?? '', nodeRunId ?? ''),
    queryFn: async () => unwrap(await flowService.getNodeMessages(wsId, flowRunId!, nodeRunId!)),
    enabled: !!wsId && !!flowRunId && !!nodeRunId,
    refetchInterval: 5_000,
  });
}

export function useStartFlowRun(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: Parameters<typeof flowService.startRun>[1]) =>
      unwrap(await flowService.startRun(wsId, payload)),
    onSuccess: (data) => {
      invalidateFlowTarget(qc, wsId, data.run);
      qc.setQueryData(queryKeys.pm.flowRun(wsId, data.run.id), data);
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowRuns(wsId) });
    },
    onError: (err: Error) => {
      toast.error(err.message || 'Failed to start flow');
    },
  });
}

export function useSendFlowNodeMessage(wsId: string, flowRunId: string, nodeRunId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (content: string) =>
      unwrap(await flowService.sendNodeMessage(wsId, flowRunId, nodeRunId, content)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowNodeMessages(wsId, flowRunId, nodeRunId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowRun(wsId, flowRunId) });
    },
  });
}

export function useSendFlowNodeAction(wsId: string, flowRunId: string, nodeRunId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { actionType: string; payload?: unknown }) =>
      unwrap(await flowService.sendNodeAction(wsId, flowRunId, nodeRunId, vars.actionType, vars.payload)),
    onSuccess: (data) => {
      invalidateFlowTarget(qc, wsId, data.run);
      qc.setQueryData(queryKeys.pm.flowRun(wsId, flowRunId), data);
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowRuns(wsId) });
    },
    onError: (err: Error) => {
      toast.error(err.message || 'Failed to perform action');
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowRun(wsId, flowRunId) });
    },
  });
}

export function useCancelFlowRun(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (flowRunId: string) => unwrap(await flowService.cancelRun(wsId, flowRunId)),
    onSuccess: (data) => {
      invalidateFlowTarget(qc, wsId, data.run);
      // The cancel is async (Temporal signal), so the response may still show the old status.
      // Remove stale cache so the UI does not render stale flow state.
      qc.removeQueries({ queryKey: queryKeys.pm.flowRun(wsId, data.run.id) });
      // Re-fetch after a short delay to pick up the cancelled state from the workflow.
      setTimeout(() => {
        qc.invalidateQueries({ queryKey: queryKeys.pm.flowRun(wsId, data.run.id) });
        invalidateFlowTarget(qc, wsId, data.run);
        qc.invalidateQueries({ queryKey: queryKeys.pm.flowRuns(wsId) });
      }, 1500);
    },
  });
}

export function useCancelActiveFlowRunByTarget(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { targetType: string; targetId: string }) =>
      unwrap(await flowService.cancelActiveByTarget(wsId, vars.targetType, vars.targetId)),
    onSuccess: (data) => {
      // Response is either a FlowRunView (cancelled flow run) or { status: "cancelled" } (orphaned session).
      if ('run' in data) {
        invalidateFlowTarget(qc, wsId, data.run);
        qc.removeQueries({ queryKey: queryKeys.pm.flowRun(wsId, data.run.id) });
        setTimeout(() => {
          qc.invalidateQueries({ queryKey: queryKeys.pm.flowRun(wsId, data.run.id) });
          invalidateFlowTarget(qc, wsId, data.run);
          qc.invalidateQueries({ queryKey: queryKeys.pm.flowRuns(wsId) });
        }, 1500);
      } else {
        // Orphaned session was cleaned up — invalidate flow runs to refresh state.
        qc.invalidateQueries({ queryKey: queryKeys.pm.flowRuns(wsId) });
      }
    },
  });
}

export function useFlowRuns(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.flowRuns(wsId),
    queryFn: async () => unwrap(await flowService.listRuns(wsId)),
    enabled: !!wsId,
  });
}

export function useFlowTemplates(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.flowTemplates(wsId),
    queryFn: async () => unwrap(await flowService.listTemplates(wsId)),
    enabled: !!wsId,
    staleTime: 60_000,
  });
}

// --- Flow Template CRUD hooks ---

export function useFlowDBTemplates(wsId: string) {
  return useQuery({
    queryKey: queryKeys.pm.flowDBTemplates(wsId),
    queryFn: async () => unwrap(await flowService.listDBTemplates(wsId)),
    enabled: !!wsId,
    staleTime: 60_000,
  });
}

export function useFlowDBTemplate(wsId: string, templateId?: string) {
  return useQuery({
    queryKey: queryKeys.pm.flowDBTemplate(wsId, templateId ?? ''),
    queryFn: async () => unwrap(await flowService.getTemplate(wsId, templateId!)),
    enabled: !!wsId && !!templateId,
  });
}

export function useCreateFlowTemplate(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: CreateFlowTemplateRequest) =>
      unwrap(await flowService.createTemplate(wsId, body)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}

export function useUpdateFlowTemplate(wsId: string, templateId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: UpdateFlowTemplateRequest) =>
      unwrap(await flowService.updateTemplate(wsId, templateId, body)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplate(wsId, templateId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}

export function useDeleteFlowTemplate(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (templateId: string) =>
      unwrap(await flowService.deleteTemplate(wsId, templateId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}

export function useDuplicateFlowTemplate(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (templateId: string) =>
      unwrap(await flowService.duplicateTemplate(wsId, templateId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}

export function useDuplicateFlowTemplateFromSlug(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (templateSlug: string) =>
      unwrap(await flowService.duplicateFromSlug(wsId, templateSlug)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}

// --- Flow Template Node CRUD hooks ---

export function useCreateFlowTemplateNode(wsId: string, templateId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: CreateFlowTemplateNodeRequest) =>
      unwrap(await flowService.createTemplateNode(wsId, templateId, body)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplate(wsId, templateId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}

export function useUpdateFlowTemplateNode(wsId: string, templateId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ nodeId, body }: { nodeId: string; body: UpdateFlowTemplateNodeRequest }) =>
      unwrap(await flowService.updateTemplateNode(wsId, templateId, nodeId, body)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplate(wsId, templateId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}

export function useDeleteFlowTemplateNode(wsId: string, templateId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (nodeId: string) =>
      unwrap(await flowService.deleteTemplateNode(wsId, templateId, nodeId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplate(wsId, templateId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowDBTemplates(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.flowTemplates(wsId) });
    },
  });
}
