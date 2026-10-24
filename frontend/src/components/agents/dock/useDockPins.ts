import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { DockChat, DockRunSummary } from '@/lib/dockTypes';
import { dockChatService } from '@/lib/services/dockChatService';
import { unwrap } from '@/lib/queryUtils';
import { queryKeys } from '@/lib/queryKeys';

export interface DockPins { chats: DockChat[]; runs: DockRunSummary[] }
const emptyPins: DockPins = { chats: [], runs: [] };

export function useDockPins(workspaceId: string | undefined, userId: string | undefined, enabled: boolean) {
  const client = useQueryClient();
  const key = queryKeys.dock.pins(workspaceId, userId);
  const query = useQuery({
    queryKey: key,
    queryFn: async () => unwrap(await dockChatService.listPins(workspaceId!)),
    enabled: enabled && !!workspaceId && !!userId,
    staleTime: 15_000,
    refetchInterval: enabled ? 30_000 : false,
  });
  const mutation = useMutation({
    mutationFn: async ({ kind, id, pinned }: { kind: 'chat' | 'run'; id: string; pinned: boolean }) => {
      unwrap(await dockChatService.setPin(workspaceId!, kind, id, pinned));
    },
    onSuccess: async () => { await client.invalidateQueries({ queryKey: key }); },
    onError: () => { toast.error('Unable to update pin. Please try again.'); },
  });
  return {
    ...(query.data ?? emptyPins),
    loading: query.isPending,
    pending: mutation.isPending,
    unavailable: query.isPending || query.isError,
    togglePin: (kind: 'chat' | 'run', id: string, pinned: boolean) => mutation.mutate({ kind, id, pinned }),
    removeArchivedChat: (id: string) => {
      client.setQueryData<DockPins>(key, (current) => current ? { ...current, chats: current.chats.filter(chat => chat.id !== id) } : current);
      void client.invalidateQueries({ queryKey: key });
    },
  };
}
