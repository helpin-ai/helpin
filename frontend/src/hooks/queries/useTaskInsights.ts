import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { TaskUpdateFilter } from '@/lib/pmTypes';

export function useTaskUpdates(workspaceId: string, taskId: string, filter: TaskUpdateFilter) {
  return useQuery({
    queryKey: queryKeys.pm.taskUpdates(workspaceId, taskId, filter),
    queryFn: async () => unwrap(await pmTaskService.listUpdates(workspaceId, taskId, filter)),
    enabled: !!workspaceId && !!taskId,
  });
}
export function useInitializeTaskUpdatesRead(workspaceId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (seenThrough: string) => pmTaskService.updateReadState(workspaceId, taskId, seenThrough, true),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks', taskId, 'updates'] }),
  });
}

export function useMarkTaskUpdatesRead(workspaceId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (seenThrough: string) => pmTaskService.updateReadState(workspaceId, taskId, seenThrough, false),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks', taskId, 'updates'] }),
  });
}

export function useTaskStandingBrief(workspaceId: string, taskId: string) {
  return useQuery({
    queryKey: queryKeys.pm.taskStandingBrief(workspaceId, taskId),
    queryFn: async () => unwrap(await pmTaskService.getStandingBrief(workspaceId, taskId)),
    enabled: !!workspaceId && !!taskId,
    refetchInterval: (query) => query.state.data?.status === 'pending_refresh' ? 10_000 : false,
  });
}

export function useRefreshTaskStandingBrief(workspaceId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => unwrap(await pmTaskService.refreshStandingBrief(workspaceId, taskId)),
    onSuccess: (brief) => queryClient.setQueryData(queryKeys.pm.taskStandingBrief(workspaceId, taskId), brief),
  });
}

export function useDismissTaskStandingBriefSuggestion(workspaceId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (key: string) => pmTaskService.dismissStandingBriefSuggestion(workspaceId, taskId, key),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.pm.taskStandingBrief(workspaceId, taskId) }),
  });
}
