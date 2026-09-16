import { useEffect, useRef, useState } from 'react';
import type { DockChat } from '@/lib/dockTypes';
import { dockChatService } from '@/lib/services/dockChatService';
import { useDockStore } from '@/stores/dockStore';
import { withDockReadDeadline } from './dockReadDeadline';

/** Explicit links take priority over restored selection and paginated history. */
export function useSharedChatLink(workspaceId: string | undefined, enabled: boolean, ready: boolean) {
  const [target, setTarget] = useState(() => enabled && typeof window !== 'undefined' ? new URL(window.location.href).searchParams.get('ask_chat')?.trim() || null : null);
  const [result, setResult] = useState<{ workspaceId?: string; chat: DockChat | null; error: string | null; pending: boolean }>({ workspaceId, chat: null, error: null, pending: !!target });
  const [attempt, setAttempt] = useState(0);
  const boundWorkspace = useRef<string | null>(null);
  const selectedId = useDockStore(state => state.activeChatId);

  useEffect(() => {
    if (!enabled || !workspaceId || !target || !ready) return;
    if (boundWorkspace.current && boundWorkspace.current !== workspaceId) {
      setTarget(null);
      setResult({ workspaceId, chat: null, error: null, pending: false });
      return;
    }
    boundWorkspace.current = workspaceId;
    const controller = new AbortController();
    let cancelled = false;
    const store = useDockStore.getState();
    store.setTab('chats');
    store.setActiveChatId(target);
    store.setCollapsed(false);
    setResult({ workspaceId, chat: null, error: null, pending: true });
    const url = new URL(window.location.href);
    url.searchParams.delete('ask_chat');
    window.history.replaceState(window.history.state, '', url);
    void withDockReadDeadline(signal => dockChatService.getChat(workspaceId, target, signal), controller).then(response => {
      if (cancelled) return;
      if (response.error || !response.data || response.data.chat.id !== target) {
        setResult({ workspaceId, chat: null, error: response.status === 403 || response.status === 404 ? 'This conversation is unavailable or is not shared with you in Helpin. Ask its owner to share it with your workspace.' : response.error || 'This shared conversation is unavailable or you do not have access.', pending: false });
        return;
      }
      store.upsertChat(response.data.chat);
      setResult({ workspaceId, chat: response.data.chat, error: null, pending: false });
    }).catch(() => {
      if (!cancelled) setResult({ workspaceId, chat: null, error: 'Could not load this shared conversation. Try again.', pending: false });
    });
    return () => { cancelled = true; controller.abort(); };
  }, [workspaceId, enabled, ready, target, attempt]);

  const inWorkspace = !result.workspaceId || result.workspaceId === workspaceId;
  return {
    chat: inWorkspace ? result.chat : null,
    pending: enabled && inWorkspace && !!target && result.pending,
    error: enabled && inWorkspace && selectedId === target ? result.error : null,
    retry: () => setAttempt(value => value + 1),
  };
}
