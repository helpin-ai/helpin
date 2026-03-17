import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { planningSessionService } from '@/lib/services/planningSessionService';
import { unwrap } from '@/lib/queryUtils';

export function usePlanningSession(wsId: string, sessionId?: string) {
  return useQuery({
    queryKey: queryKeys.pm.planningSession(wsId, sessionId!),
    queryFn: async () => unwrap(await planningSessionService.get(wsId, sessionId!)),
    enabled: !!wsId && !!sessionId,
    refetchInterval: 10_000,
  });
}

export function usePlanningMessages(wsId: string, sessionId?: string) {
  return useQuery({
    queryKey: queryKeys.pm.planningMessages(wsId, sessionId!),
    queryFn: async () => unwrap(await planningSessionService.getMessages(wsId, sessionId!)),
    enabled: !!wsId && !!sessionId,
    refetchInterval: 5_000,
  });
}

export function useStartPlanningSession(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { epicId: string; agentId: string; additionalContext?: string }) =>
      unwrap(
        await planningSessionService.start(wsId, vars.epicId, {
          agent_id: vars.agentId,
          additional_context: vars.additionalContext,
        }),
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
    },
  });
}

export function useSendPlanningMessage(wsId: string, sessionId: string) {
  return useMutation({
    mutationFn: async (content: string) =>
      unwrap(await planningSessionService.sendMessage(wsId, sessionId, content)),
  });
}

export function useFinalizePlanningSession(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (sessionId: string) =>
      unwrap(await planningSessionService.finalize(wsId, sessionId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
    },
  });
}

export function useAbandonPlanningSession(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (sessionId: string) =>
      unwrap(await planningSessionService.abandon(wsId, sessionId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.epics(wsId) });
    },
  });
}
