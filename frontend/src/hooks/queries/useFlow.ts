import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { flowService } from '@/lib/services/flowService';

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
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, data.run.target_id) });
      qc.setQueryData(queryKeys.pm.flowRun(wsId, data.run.id), data);
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
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, data.run.target_id) });
      qc.setQueryData(queryKeys.pm.flowRun(wsId, flowRunId), data);
    },
  });
}

export function useCancelFlowRun(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (flowRunId: string) => unwrap(await flowService.cancelRun(wsId, flowRunId)),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
      qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, data.run.target_id) });
      // The cancel is async (Temporal signal), so the response may still show the old status.
      // Remove stale cache so the UI does not render stale flow state.
      qc.removeQueries({ queryKey: queryKeys.pm.flowRun(wsId, data.run.id) });
      // Re-fetch after a short delay to pick up the cancelled state from the workflow.
      setTimeout(() => {
        qc.invalidateQueries({ queryKey: queryKeys.pm.flowRun(wsId, data.run.id) });
        qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
        qc.invalidateQueries({ queryKey: queryKeys.pm.epic(wsId, data.run.target_id) });
      }, 1500);
    },
  });
}
