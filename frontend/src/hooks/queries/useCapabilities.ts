import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { capabilityService } from '@/lib/services/capabilityService';
import type { TestEmailResult } from '@/lib/capabilityTypes';

/** What this workspace can do on this server: AI, email, support, GitHub, storage and workers. */
export function useWorkspaceCapabilities(workspaceId?: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.capabilities(workspaceId ?? ''),
    queryFn: async () => unwrap(await capabilityService.workspace(workspaceId!)),
    enabled: Boolean(workspaceId),
    staleTime: 15_000,
    // Settings are often finished in another tab (GitHub, provider consoles).
    refetchOnWindowFocus: 'always',
  });
}

/** Invalidates the capability and Setup guide views after a check changes evidence. */
export function invalidateCapabilities(queryClient: ReturnType<typeof useQueryClient>, workspaceId: string) {
  void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.capabilities(workspaceId) });
  void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.setup(workspaceId) });
}

/**
 * Sends a test email to the signed-in user. Rate limiting (429) and a missing
 * recipient (422) resolve with `ok: false` so the caller can show the reason;
 * other failures reject.
 */
export function useSendTestEmail(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (): Promise<TestEmailResult> => {
      const response = await capabilityService.sendTestEmail(workspaceId);
      if (response.status === 429) {
        return { ok: false, rate_limited: true, error: response.error ?? 'Too many test emails. Wait a minute and try again.' };
      }
      if (response.status === 422) {
        return { ok: false, error: response.error ?? 'Your account has no email address.' };
      }
      return unwrap(response);
    },
    onSettled: () => invalidateCapabilities(queryClient, workspaceId),
  });
}
