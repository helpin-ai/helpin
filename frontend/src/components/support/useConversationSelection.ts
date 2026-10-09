import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import type { SupportConversation } from '@/lib/pmTypes';
import { loadConversationSelection } from '@/lib/supportBulkActions';

const EMPTY_SELECTION = new Map<string, SupportConversation>();

export function useConversationSelection(scope: string) {
  const [state, setState] = useState({ scope, items: EMPTY_SELECTION, loading: false, loadedCount: 0, allMatches: false });
  const loader = useRef<AbortController | null>(null);
  useEffect(() => () => loader.current?.abort(), [scope]);
  const current = state.scope === scope ? state : { scope, items: EMPTY_SELECTION, loading: false, loadedCount: 0, allMatches: false };
  if (state.scope !== scope) setState(current);

  const clear = useCallback(() => {
    loader.current?.abort();
    setState({ scope, items: EMPTY_SELECTION, loading: false, loadedCount: 0, allMatches: false });
  }, [scope]);

  const toggle = useCallback((conversation: SupportConversation) => {
    setState((previous) => {
      const items = new Map(previous.scope === scope ? previous.items : EMPTY_SELECTION);
      if (items.has(conversation.id)) items.delete(conversation.id);
      else items.set(conversation.id, conversation);
      return { scope, items, loading: false, loadedCount: 0, allMatches: false };
    });
  }, [scope]);

  const selectLoaded = useCallback((conversations: SupportConversation[]) => {
    setState((previous) => {
      const items = new Map(previous.scope === scope ? previous.items : EMPTY_SELECTION);
      conversations.forEach((conversation) => items.set(conversation.id, conversation));
      return { scope, items, loading: false, loadedCount: 0, allMatches: false };
    });
  }, [scope]);

  const removeCompleted = useCallback((ids: string[]) => {
    setState((previous) => {
      if (previous.scope !== scope) return previous;
      const items = new Map(previous.items);
      ids.forEach((id) => items.delete(id));
      return { ...previous, items, allMatches: false };
    });
  }, [scope]);

  const selectAllMatches = useCallback(async (
    workspaceId: string,
    filters: Parameters<typeof loadConversationSelection>[1],
    filterConversations: (conversations: SupportConversation[]) => SupportConversation[],
  ) => {
    loader.current?.abort();
    const controller = new AbortController();
    loader.current = controller;
    setState((previous) => ({ ...previous, scope, loading: true, loadedCount: 0 }));
    try {
      const all = await loadConversationSelection(workspaceId, filters, controller.signal, (loadedCount) => {
        if (!controller.signal.aborted) setState((previous) => ({ ...previous, loadedCount }));
      });
      if (controller.signal.aborted) return;
      setState({ scope, items: new Map(filterConversations(all).map((item) => [item.id, item])), loading: false, loadedCount: 0, allMatches: true });
    } catch (error) {
      if (controller.signal.aborted) return;
      setState((previous) => ({ ...previous, loading: false }));
      toast.error('Could not select all conversations', { description: error instanceof Error ? error.message : 'Please try again.' });
    }
  }, [scope]);

  return { ...current, clear, toggle, selectLoaded, selectAllMatches, removeCompleted };
}
