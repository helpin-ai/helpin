import { useQuery } from '@tanstack/react-query';
import { dockChatService } from '@/lib/services/dockChatService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';

export function useAskAgentDefaults(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.dock.aiDefaults(workspaceId),
    queryFn: async () => {
      const defaults = unwrap(await dockChatService.getAIDefaults(workspaceId));
      if (!defaults || !Object.hasOwn(defaults, 'ai_profile_id')) throw new Error('Missing Ask Agent defaults');
      return defaults;
    },
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
}
