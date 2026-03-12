import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { supportService } from '@/lib/services/supportService';
import { unwrap } from '@/lib/queryUtils';
import type { SupportInboxSettings } from '@/lib/pmTypes';

export function useChatSettings(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.installation(workspaceId),
    queryFn: async () => unwrap(await supportService.getInstallation(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
}

export function useUpdateChatSettings(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (settings: Partial<SupportInboxSettings>) =>
      supportService.updateInstallationSettings(workspaceId, settings).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.installation(workspaceId) });
    },
  });
}

export function useRegenerateWidgetKey(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => supportService.regenerateWidgetKey(workspaceId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.installation(workspaceId) });
    },
  });
}
