import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { flowService } from '@/lib/services/flowService';
import type { FlowRunView } from '@/lib/pmTypes';

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
