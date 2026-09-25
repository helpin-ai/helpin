import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { sampleDataService } from '@/lib/services/sampleDataService';
import type { SampleDataStatus } from '@/lib/sampleDataTypes';

/**
 * Query key for the sample data status. It sits under the workspace key so
 * workspace-wide invalidation also refreshes it.
 */
export const sampleDataQueryKey = (workspaceId: string) => ['workspaces', workspaceId, 'sample-data'] as const;

/** Whether the workspace holds sample data, and how many records of each kind. */
export function useSampleDataStatus(workspaceId?: string) {
  return useQuery({
    queryKey: sampleDataQueryKey(workspaceId ?? ''),
    queryFn: async () => unwrap(await sampleDataService.status(workspaceId!)),
    enabled: Boolean(workspaceId),
    staleTime: 30_000,
  });
}

/** Refreshes every view sample data can appear in after it is loaded or removed. */
export function invalidateSampleDataViews(queryClient: QueryClient, workspaceId: string) {
  for (const queryKey of [
    sampleDataQueryKey(workspaceId),
    queryKeys.workspaces.setup(workspaceId),
    queryKeys.workspaces.teams(workspaceId),
    ['pm', workspaceId],
    ['docs', workspaceId],
    ['crm', workspaceId],
    ['support', workspaceId],
  ]) {
    void queryClient.invalidateQueries({ queryKey });
  }
}

function useSampleDataMutation(workspaceId: string, run: (id: string) => Promise<{ data: SampleDataStatus | null; error: string | null }>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => unwrap(await run(workspaceId)),
    onSuccess: (status) => {
      queryClient.setQueryData(sampleDataQueryKey(workspaceId), status);
    },
    onSettled: () => invalidateSampleDataViews(queryClient, workspaceId),
  });
}

/** Loads sample data. Fails with the server message when it is already loaded. */
export function useLoadSampleData(workspaceId: string) {
  return useSampleDataMutation(workspaceId, sampleDataService.load);
}

/** Removes every sample record in one step. */
export function useRemoveSampleData(workspaceId: string) {
  return useSampleDataMutation(workspaceId, sampleDataService.remove);
}
