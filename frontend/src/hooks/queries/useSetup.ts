import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { setupService } from '@/lib/services/setupService';
import type { SetupGoalKey } from '@/lib/setupTypes';

export function useSetup(workspaceId?: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.setup(workspaceId ?? ''),
    queryFn: async () => unwrap(await setupService.get(workspaceId!)),
    enabled: Boolean(workspaceId),
    staleTime: 15_000,
  });
}

export function useUpdateSetupGoals(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (goals: SetupGoalKey[]) => unwrap(await setupService.updateGoals(workspaceId, goals)),
    onSuccess: (view) => queryClient.setQueryData(queryKeys.workspaces.setup(workspaceId), view),
  });
}

export function useUpdateSetupPreference(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (dismissed: boolean) => unwrap(await setupService.updatePreference(workspaceId, dismissed)),
    onSuccess: (preference) => queryClient.setQueryData(queryKeys.workspaces.setup(workspaceId), (current: unknown) => {
      if (!current || typeof current !== 'object') return current;
      return { ...current, preference };
    }),
  });
}
