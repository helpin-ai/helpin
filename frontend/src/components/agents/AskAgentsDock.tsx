import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import { AiMagicIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDockStore } from '@/stores/dockStore';
import { dockChatService } from '@/lib/services/dockChatService';
import { Button } from '@/components/ui/button';
import { ChatListView } from './dock/ChatListView';
import { ChatView } from './dock/ChatView';

type AskAgentsEventDetail = { query?: string; mode?: 'compose' | 'runs'; runId?: string };

/**
 * The dock: the workspace's primary chat surface. Each chat is backed by an
 * agent-runtime chat-mode run of the Ask Agent, which answers questions with
 * read-only tools and launches other agents (behind explicit approval) for
 * durable work — child results are delivered back into the chat.
 */
export function AskAgentsDock() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const {
    collapsed,
    setCollapsed,
    view,
    setView,
    activeChatId,
    setActiveChatId,
    chats,
    setChats,
  } = useDockStore();

  const [hiddenByModal, setHiddenByModal] = useState(false);
  const [pendingDraft, setPendingDraft] = useState<string | undefined>(undefined);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  const workspaceId = workspace?.id;

  const refreshChats = useCallback(async () => {
    if (!workspaceId) return;
    const res = await dockChatService.listChats(workspaceId);
    if (res.data) setChats(res.data.chats);
  }, [setChats, workspaceId]);

  // Load chats on workspace change; validate the persisted active chat.
  useEffect(() => {
    if (!workspaceId) return;
    void (async () => {
      const res = await dockChatService.listChats(workspaceId);
      if (!res.data) return;
      setChats(res.data.chats);
      const ids = new Set(res.data.chats.map((c) => c.id));
      const current = useDockStore.getState().activeChatId;
      if (current && !ids.has(current)) setActiveChatId(res.data.chats[0]?.id ?? null);
      if (!current && res.data.chats.length > 0) setActiveChatId(res.data.chats[0].id);
    })();
  }, [setActiveChatId, setChats, workspaceId]);

  const openDock = useCallback(
    (targetView?: 'chat' | 'chats') => {
      setCollapsed(false);
      if (targetView) setView(targetView);
      requestAnimationFrame(() => textareaRef.current?.focus());
    },
    [setCollapsed, setView],
  );

  const newChat = useCallback(async () => {
    if (!workspaceId) return;
    const res = await dockChatService.createChat(workspaceId);
    if (res.error || !res.data) {
      toast.error(res.error ?? 'Failed to create chat');
      return;
    }
    setActiveChatId(res.data.id);
    setView('chat');
    void refreshChats();
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, [refreshChats, setActiveChatId, setView, workspaceId]);

  // `/` focuses the dock when no editable element has focus.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== '/') return;
      const target = e.target as HTMLElement | null;
      if (!target) return;
      const tag = target.tagName?.toLowerCase();
      const isEditable = tag === 'input' || tag === 'textarea' || target.isContentEditable;
      if (isEditable) return;
      e.preventDefault();
      openDock('chat');
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [openDock]);

  // Programmatic open: window.dispatchEvent(new CustomEvent('helpin:ask-agents', { detail })).
  useEffect(() => {
    const onAsk = (event: Event) => {
      const detail = (event as CustomEvent<AskAgentsEventDetail>).detail;
      const query = detail?.query?.trim() ?? '';
      if (!query && !detail?.mode && !detail?.runId) return;
      openDock(detail?.mode === 'runs' ? 'chats' : 'chat');
      if (query) setPendingDraft(query);
    };
    window.addEventListener('helpin:ask-agents', onAsk);
    return () => window.removeEventListener('helpin:ask-agents', onAsk);
  }, [openDock]);

  // Hide (do not unmount) while a Radix dialog/alertdialog outside the dock is
  // open, so the floating surface never fights a modal for pointer events.
  useEffect(() => {
    const compute = () => {
      const dialogs = document.querySelectorAll(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      );
      let hidden = false;
      dialogs.forEach((dialog) => {
        if (!dialog.closest('[data-helpin-dock]')) hidden = true;
      });
      setHiddenByModal(hidden);
    };
    compute();
    const observer = new MutationObserver(compute);
    observer.observe(document.body, { childList: true, subtree: true, attributeFilter: ['data-state'] });
    return () => observer.disconnect();
  }, []);

  if (!workspaceId) return null;

  const dockVisibilityClass = hiddenByModal ? 'translate-y-4 opacity-0' : 'translate-y-0 opacity-100';
  const dockInteractionClass = hiddenByModal ? 'pointer-events-none' : 'pointer-events-auto';

  if (collapsed) {
    return createPortal(
      <div
        data-helpin-dock="true"
        aria-hidden={hiddenByModal}
        className={cn(
          'pointer-events-none fixed inset-x-0 bottom-8 z-[60] flex justify-center transition-[opacity,transform] duration-200 ease-out',
          dockVisibilityClass,
        )}
      >
        <button
          type="button"
          onClick={() => openDock()}
          className={cn(
            'group inline-flex items-center gap-2.5 rounded-full border border-border/70 bg-background/95 px-4 py-2.5 text-sm font-medium text-muted-foreground shadow-[0_1px_2px_rgba(15,23,42,0.05),0_8px_24px_-8px_rgba(15,23,42,0.22)] backdrop-blur transition hover:border-foreground/30 hover:bg-background hover:text-foreground',
            dockInteractionClass,
          )}
        >
          <AiMagicIcon className="h-4 w-4" />
          Ask agents
          <kbd className="ml-1 rounded border bg-muted px-1.5 py-0.5 text-[11px] font-mono text-muted-foreground">/</kbd>
        </button>
      </div>,
      document.body,
    );
  }

  const activeChat = chats.find((c) => c.id === activeChatId) ?? null;

  return createPortal(
    <div
      data-helpin-dock="true"
      aria-hidden={hiddenByModal}
      className={cn(
        'pointer-events-none fixed inset-x-0 bottom-6 z-[60] flex justify-center px-4 transition-[opacity,transform] duration-200 ease-out',
        dockVisibilityClass,
      )}
    >
      <div
        className={cn(
          'flex w-full max-w-2xl flex-col rounded-2xl border border-border/70 bg-background/95 shadow-[0_1px_2px_rgba(15,23,42,0.05),0_16px_48px_-12px_rgba(15,23,42,0.28)] ring-1 ring-black/[0.02] backdrop-blur animate-in fade-in zoom-in-95 slide-in-from-bottom-2 duration-200 ease-out dark:ring-white/[0.04]',
          dockInteractionClass,
        )}
      >
        <div className="flex items-center gap-2 border-b border-border/60 px-3 py-2">
          <AiMagicIcon className="h-4 w-4 text-muted-foreground" />
          <span className="min-w-0 flex-1 truncate text-sm font-medium">
            {view === 'chats' ? 'Chats' : activeChat?.title.trim() || 'New chat'}
          </span>
          <Button
            size="sm"
            variant="ghost"
            className="h-7 px-2 text-xs"
            onClick={() => setView(view === 'chat' ? 'chats' : 'chat')}
          >
            {view === 'chat' ? 'Chats' : 'Back'}
          </Button>
          <Button size="sm" variant="ghost" className="h-7 px-2 text-xs" onClick={() => void newChat()}>
            New chat
          </Button>
          <Button size="sm" variant="ghost" className="h-7 px-2 text-xs" onClick={() => setCollapsed(true)}>
            Close
          </Button>
        </div>
        {view === 'chats' ? (
          <ChatListView
            workspaceId={workspaceId}
            chats={chats}
            activeChatId={activeChatId}
            onSelect={(chatId) => {
              setActiveChatId(chatId);
              setView('chat');
            }}
            onChanged={() => void refreshChats()}
          />
        ) : activeChatId ? (
          <ChatView
            workspaceId={workspaceId}
            chatId={activeChatId}
            textareaRef={textareaRef}
            initialDraft={pendingDraft}
            onDraftConsumed={() => setPendingDraft(undefined)}
          />
        ) : (
          <NewChatPrompt onCreate={() => void newChat()} />
        )}
      </div>
    </div>,
    document.body,
  );
}

function NewChatPrompt({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex flex-col items-center gap-3 px-4 py-8">
      <p className="text-sm text-muted-foreground">Start a conversation with your workspace agents.</p>
      <Button size="sm" onClick={onCreate}>
        New chat
      </Button>
    </div>
  );
}
