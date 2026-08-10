import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import { AiMagicIcon, Cancel01Icon, Maximize01Icon } from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDockStore } from '@/stores/dockStore';
import { dockChatService } from '@/lib/services/dockChatService';
import type { DockChat, DockRunSummary } from '@/lib/dockTypes';
import { DockRoster } from './dock/DockRoster';
import { ChatView } from './dock/ChatView';
import { DockRunView } from './dock/DockRunView';
import { dockRunContext, dockRunTitle, presentDockRun } from './dock/dockPresentation';
import { buildCodingSessionPath } from '@/lib/codingSessionSurface';

type AskAgentsEventDetail = { query?: string; mode?: 'compose' | 'runs'; runId?: string; chatId?: string };

/** Persistent workspace presence layer for chats and user-owned agent runs. */
export function AskAgentsDock() {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const {
    collapsed,
    setCollapsed,
    tab,
    setTab,
    activeChatId,
    setActiveChatId,
    activeRunId,
    setActiveRunId,
    chats,
    setChats,
    drafts,
    setDraft,
    clearDraft,
    lastAttentionIds,
    setLastAttentionIds,
    activateWorkspace,
  } = useDockStore();
  const workspaceId = workspace?.id;
  const [runs, setRuns] = useState<DockRunSummary[]>([]);
  const [runsLoading, setRunsLoading] = useState(true);
  const [chatsLoading, setChatsLoading] = useState(true);
  const [runsError, setRunsError] = useState<string | null>(null);
  const [chatsError, setChatsError] = useState<string | null>(null);
  const [hiddenByModal, setHiddenByModal] = useState(false);
  const [pendingDraft, setPendingDraft] = useState<string | undefined>();
  const [attentionNudge, setAttentionNudge] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  const orderedRuns = useMemo(() => [...runs].sort((left, right) => {
    const leftPresentation = presentDockRun(left.run.status, left.run.pause_reason, left.attention_kind);
    const rightPresentation = presentDockRun(right.run.status, right.run.pause_reason, right.attention_kind);
    const groupOrder = { needs_you: 0, running: 1, recent: 2 } as const;
    const groupDelta = groupOrder[leftPresentation.group] - groupOrder[rightPresentation.group];
    return groupDelta || Date.parse(right.last_activity_at) - Date.parse(left.last_activity_at);
  }), [runs]);
  const attentionRuns = useMemo(() => orderedRuns.filter((run) => !!run.attention_kind), [orderedRuns]);
  const activeRun = orderedRuns.find((summary) => summary.run.id === activeRunId) ?? null;
  const activeChat = chats.find((chat) => chat.id === activeChatId) ?? null;

  const refreshRuns = useCallback(async () => {
    if (!workspaceId) return [];
    setRunsError(null);
    const result = await dockChatService.listRuns(workspaceId);
    if (useWorkspaceStore.getState().currentWorkspace?.id !== workspaceId) return [];
    if (result.error || !result.data) {
      setRunsError(result.error ?? 'Unable to load agent runs');
      setRunsLoading(false);
      return [];
    }
    setRuns(result.data.runs ?? []);
    setRunsLoading(false);
    return result.data.runs ?? [];
  }, [workspaceId]);

  const refreshChats = useCallback(async () => {
    if (!workspaceId) return;
    setChatsError(null);
    const result = await dockChatService.listChats(workspaceId);
    if (useWorkspaceStore.getState().currentWorkspace?.id !== workspaceId) return;
    if (result.error || !result.data) {
      setChatsError(result.error ?? 'Unable to load conversations');
      setChatsLoading(false);
      return;
    }
    setChats(result.data.chats ?? []);
    setChatsLoading(false);
  }, [setChats, workspaceId]);

  useEffect(() => {
    if (!workspaceId) return;
    activateWorkspace(workspaceId);
    let cancelled = false;
    queueMicrotask(() => {
      if (cancelled) return;
      setRuns([]);
      setRunsLoading(true);
      setChatsLoading(true);
      void Promise.all([refreshRuns(), refreshChats()]);
    });
    return () => { cancelled = true; };
  }, [activateWorkspace, refreshChats, refreshRuns, workspaceId]);

  useEffect(() => {
    if (runsLoading) return;
    const current = useDockStore.getState().activeRunId;
    if (current && orderedRuns.some((summary) => summary.run.id === current)) return;
    setActiveRunId(orderedRuns[0]?.run.id ?? null);
  }, [orderedRuns, runsLoading, setActiveRunId]);

  useEffect(() => {
    if (chatsLoading) return;
    const current = useDockStore.getState().activeChatId;
    if (current && chats.some((chat) => chat.id === current)) return;
    setActiveChatId(chats[0]?.id ?? null);
  }, [chats, chatsLoading, setActiveChatId]);

  useEffect(() => {
    if (runsLoading) return;
    const nextIds = attentionRuns.map((summary) => summary.run.id);
    const hasNew = nextIds.some((id) => !lastAttentionIds.includes(id));
    if (hasNew && lastAttentionIds.length > 0) {
      const startTimer = window.setTimeout(() => setAttentionNudge(true), 0);
      const endTimer = window.setTimeout(() => setAttentionNudge(false), 350);
      setLastAttentionIds(nextIds);
      return () => {
        window.clearTimeout(startTimer);
        window.clearTimeout(endTimer);
      };
    }
    setLastAttentionIds(nextIds);
  }, [attentionRuns, lastAttentionIds, runsLoading, setLastAttentionIds]);

  useEffect(() => {
    if (!workspaceId) return;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const refresh = () => {
      if (timer) return;
      timer = setTimeout(() => {
        timer = null;
        void refreshRuns();
      }, 180);
    };
    window.addEventListener('agent_run-updated', refresh);
    window.addEventListener('coding_session-updated', refresh);
    return () => {
      if (timer) clearTimeout(timer);
      window.removeEventListener('agent_run-updated', refresh);
      window.removeEventListener('coding_session-updated', refresh);
    };
  }, [refreshRuns, workspaceId]);

  const openDock = useCallback((targetTab?: 'agents' | 'chats') => {
    if (targetTab) setTab(targetTab);
    setCollapsed(false);
    if (targetTab === 'chats') requestAnimationFrame(() => textareaRef.current?.focus());
  }, [setCollapsed, setTab]);

  const closeDock = useCallback(() => {
    setCollapsed(true);
    requestAnimationFrame(() => triggerRef.current?.focus());
  }, [setCollapsed]);

  const newChat = useCallback(async () => {
    if (!workspaceId) return;
    const result = await dockChatService.createChat(workspaceId);
    if (result.error || !result.data) {
      toast.error(result.error ?? 'Failed to create chat');
      return;
    }
    setChats([result.data, ...chats.filter((chat) => chat.id !== result.data?.id)]);
    setActiveChatId(result.data.id);
    setTab('chats');
    setCollapsed(false);
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, [chats, setActiveChatId, setChats, setCollapsed, setTab, workspaceId]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target;
      const editable = target instanceof Element
        && target.matches('input, textarea, select, [contenteditable="true"]');
      if (event.key === '/' && !editable && !event.metaKey && !event.ctrlKey && !event.altKey) {
        event.preventDefault();
        openDock('chats');
        if (!useDockStore.getState().activeChatId) void newChat();
      } else if (event.key.toLowerCase() === 'n' && !collapsed && !editable && !event.metaKey && !event.ctrlKey && !event.altKey) {
        event.preventDefault();
        void newChat();
      } else if (event.key === 'Escape' && !collapsed) {
        event.preventDefault();
        closeDock();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [closeDock, collapsed, newChat, openDock]);

  useEffect(() => {
    if (collapsed) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Tab' || !panelRef.current) return;
      const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      )).filter((element) => element.offsetParent !== null);
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [collapsed]);

  useEffect(() => {
    const onAsk = (event: Event) => {
      const detail = (event as CustomEvent<AskAgentsEventDetail>).detail ?? {};
      const query = detail.query?.trim();
      if (detail.runId) {
        setActiveRunId(detail.runId);
        openDock('agents');
        return;
      }
      if (detail.chatId) {
        setActiveChatId(detail.chatId);
        openDock('chats');
      } else {
        openDock(detail.mode === 'runs' ? 'agents' : 'chats');
      }
      if (query) setPendingDraft(query);
    };
    window.addEventListener('helpin:ask-agents', onAsk);
    return () => window.removeEventListener('helpin:ask-agents', onAsk);
  }, [openDock, setActiveChatId, setActiveRunId]);

  useEffect(() => {
    const compute = () => {
      const dialogs = document.querySelectorAll('[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]');
      let hidden = false;
      dialogs.forEach((dialog) => { if (!dialog.closest('[data-helpin-dock]')) hidden = true; });
      setHiddenByModal(hidden);
    };
    compute();
    const observer = new MutationObserver(compute);
    observer.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-state'] });
    return () => observer.disconnect();
  }, []);

  if (!workspaceId || typeof document === 'undefined') return null;

  return createPortal(
    <div
      data-helpin-dock="true"
      aria-hidden={hiddenByModal || undefined}
      className={cn('agent-dock-root fixed inset-0 z-[60]', hiddenByModal && 'pointer-events-none opacity-0')}
    >
      {!collapsed ? (
        <button type="button" aria-label="Close agent dock" tabIndex={-1} onClick={closeDock} className="agent-dock-scrim absolute inset-0 bg-[rgba(28,27,25,.10)] backdrop-blur-[1.5px] dark:bg-black/35" />
      ) : null}

      <div className="agent-dock-anchor pointer-events-none absolute inset-x-0 bottom-[calc(22px+env(safe-area-inset-bottom))] flex flex-col items-center gap-2.5 px-3">
        {!collapsed ? (
          <div
            ref={panelRef}
            id="agent-dock-panel"
            role="dialog"
            aria-modal="true"
            aria-label="Agents and chats"
            className="agent-dock-panel pointer-events-auto flex h-[min(600px,calc(100dvh-104px))] w-[min(900px,92vw)] min-h-[360px] overflow-hidden rounded-[18px] border border-[#e6e3dd] bg-[#fffefa] shadow-[0_30px_70px_-26px_rgba(28,27,25,.5)] dark:border-[#37352f] dark:bg-[#242320]"
          >
            <DockRoster
              workspaceId={workspaceId}
              tab={tab}
              runs={orderedRuns}
              chats={chats}
              selectedRunId={activeRunId}
              selectedChatId={activeChatId}
              loadingRuns={runsLoading}
              loadingChats={chatsLoading}
              runsError={runsError}
              chatsError={chatsError}
              onTabChange={(next) => {
                setTab(next);
                if (next === 'agents' && !activeRunId) setActiveRunId(orderedRuns[0]?.run.id ?? null);
                if (next === 'chats' && !activeChatId) setActiveChatId(chats[0]?.id ?? null);
              }}
              onSelectRun={(runId) => { setActiveRunId(runId); setTab('agents'); }}
              onSelectChat={(chatId) => { setActiveChatId(chatId); setTab('chats'); }}
              onNewChat={() => void newChat()}
              onChatsChanged={() => void refreshChats()}
              onRetryRuns={() => void refreshRuns()}
              onRetryChats={() => void refreshChats()}
            />
            <section className="flex min-w-0 flex-1 flex-col bg-[#fffefa] dark:bg-[#242320]">
              <DockPaneHeader
                tab={tab}
                run={activeRun}
                chat={activeChat}
                workspaceSlug={workspace.slug}
                onClose={closeDock}
              />
              {tab === 'agents' ? (
                activeRun ? (
                  <DockRunView
                    key={activeRun.run.id}
                    workspaceId={workspaceId}
                    summary={activeRun}
                    draft={drafts[`run:${activeRun.run.id}`] ?? ''}
                    onDraftChange={(value) => setDraft(`run:${activeRun.run.id}`, value)}
                    onRunChanged={() => void refreshRuns()}
                  onRunContinued={(runId) => {
                      clearDraft(`run:${activeRun.run.id}`);
                      void refreshRuns().then((nextRuns) => {
                        if (nextRuns.some((summary) => summary.run.id === runId)) setActiveRunId(runId);
                      });
                    }}
                  />
                ) : (
                  <EmptyRunPane onNewChat={() => void newChat()} />
                )
              ) : activeChat ? (
                <ChatView
                  key={activeChat.id}
                  workspaceId={workspaceId}
                  chatId={activeChat.id}
                  textareaRef={textareaRef}
                  initialDraft={pendingDraft}
                  onDraftConsumed={() => setPendingDraft(undefined)}
                  draftValue={drafts[`chat:${activeChat.id}`] ?? ''}
                  onDraftChange={(value) => setDraft(`chat:${activeChat.id}`, value)}
                  onChatChanged={() => void refreshChats()}
                />
              ) : (
                <EmptyChatPane onNewChat={() => void newChat()} />
              )}
            </section>
          </div>
        ) : null}

        <DockTrigger
          triggerRef={triggerRef}
          open={!collapsed}
          runs={orderedRuns}
          attentionCount={attentionRuns.length}
          nudge={attentionNudge}
          onClick={() => collapsed ? openDock() : closeDock()}
        />
      </div>
      <div className="sr-only" aria-live="polite">{attentionRuns.length > 0 ? `${attentionRuns.length} agent${attentionRuns.length === 1 ? '' : 's'} need your attention` : ''}</div>
    </div>,
    document.body,
  );
}

function DockPaneHeader({
  tab,
  run,
  chat,
  workspaceSlug,
  onClose,
}: {
  tab: 'agents' | 'chats';
  run: DockRunSummary | null;
  chat: DockChat | null;
  workspaceSlug?: string;
  onClose: () => void;
}) {
  const presentation = run ? presentDockRun(run.run.status, run.run.pause_reason, run.attention_kind) : null;
  const fullPath = run ? buildCodingSessionPath(workspaceSlug, run.run.id) : null;
  return (
    <header className="flex min-h-[51px] items-center gap-2.5 border-b border-[#f1efea] px-3.5 py-2.5 dark:border-[#302f2b]">
      {run && tab === 'agents' ? (
        <AgentAvatar name={run.agent.name} presetKey={run.agent.preset_key} iconKey={run.agent.icon_key} className="h-[26px] w-[26px] rounded-[8px] border-0 shadow-none" />
      ) : (
        <span className="agent-dock-sparkle grid h-[26px] w-[26px] shrink-0 place-items-center rounded-[8px]"><AiMagicIcon className="h-3.5 w-3.5 text-white" /></span>
      )}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[13.5px] font-semibold text-[#1c1b19] dark:text-[#eeeae1]">
          {tab === 'agents' ? (run ? dockRunTitle(run) : 'Agents') : chat?.title.trim() || 'New chat'}
        </span>
        <span className="block truncate font-mono text-[10.5px] text-[#a5a29b]">
          {tab === 'agents' && run ? dockRunContext(run) : 'Workspace conversation'}
        </span>
      </span>
      {presentation ? (
        <span className="shrink-0 rounded-full px-2 py-[3px] text-[11px] font-semibold" style={{ backgroundColor: presentation.chipBackground, color: presentation.chipForeground }}>
          {presentation.label}
        </span>
      ) : null}
      {fullPath ? (
        <a href={fullPath} aria-label="Open full agent session" title="Open full session" className="grid h-8 w-8 place-items-center rounded-md text-[#a5a29b] transition hover:bg-[#f4f2ee] hover:text-[#4b4945] dark:hover:bg-[#302f2b]">
          <Maximize01Icon className="h-3.5 w-3.5" />
        </a>
      ) : null}
      <button type="button" onClick={onClose} aria-label="Close agent dock" className="grid h-8 w-8 place-items-center rounded-md text-[#a5a29b] transition hover:bg-[#f4f2ee] hover:text-[#4b4945] dark:hover:bg-[#302f2b]">
        <Cancel01Icon className="h-3.5 w-3.5" />
      </button>
    </header>
  );
}

function DockTrigger({
  triggerRef,
  open,
  runs,
  attentionCount,
  nudge,
  onClick,
}: {
  triggerRef: React.RefObject<HTMLButtonElement | null>;
  open: boolean;
  runs: DockRunSummary[];
  attentionCount: number;
  nudge: boolean;
  onClick: () => void;
}) {
  const visible = runs.slice(0, 4);
  return (
    <button
      ref={triggerRef}
      type="button"
      aria-expanded={open}
      aria-controls="agent-dock-panel"
      onClick={onClick}
      className={cn(
        'agent-dock-trigger pointer-events-auto flex min-h-[42px] max-w-[calc(100vw-24px)] items-center gap-3 rounded-[26px] border border-[#e6e3dd] bg-[#fffefa] py-[7px] pe-2 ps-4 text-[#1c1b19] shadow-[0_12px_30px_-14px_rgba(28,27,25,.45)] transition-[transform,border-color] duration-200 hover:border-[#d2cec5] dark:border-[#37352f] dark:bg-[#242320] dark:text-[#eeeae1]',
        nudge && 'agent-dock-attention-nudge',
      )}
    >
      <span className="flex min-w-0 items-center gap-2">
        <span className="agent-dock-sparkle grid h-[13px] w-[13px] shrink-0 place-items-center rounded-[4px]"><AiMagicIcon className="h-2.5 w-2.5 text-white" /></span>
        <span className="truncate text-[13.5px] font-medium">Ask agents</span>
        <kbd className="rounded-[5px] border border-[#eae7e0] px-[5px] py-px font-mono text-[11px] text-[#a5a29b] dark:border-[#3a3832]">/</kbd>
      </span>
      {runs.length > 0 ? (
        <>
          <span className="h-[22px] w-px shrink-0 bg-[#eeece7] dark:bg-[#3a3832]" />
          <span className="flex shrink-0 items-center ps-1">
            {visible.map((summary) => {
              const presentation = presentDockRun(summary.run.status, summary.run.pause_reason, summary.attention_kind);
              return (
                <span key={summary.run.id} className="agent-dock-stack-item relative -ms-1.5 first:ms-0">
                  <AgentAvatar name={summary.agent.name} presetKey={summary.agent.preset_key} iconKey={summary.agent.icon_key} className="h-[26px] w-[26px] rounded-[9px] border-0 shadow-[0_0_0_2px_#fffefa] dark:shadow-[0_0_0_2px_#242320]" />
                  <span className="absolute -bottom-px -end-px h-2 w-2 rounded-full border-2 border-[#fffefa] dark:border-[#242320]" style={{ backgroundColor: presentation.dot }} />
                </span>
              );
            })}
            {runs.length > 4 ? <span className="-ms-1.5 grid h-[26px] min-w-[26px] place-items-center rounded-[9px] bg-[#f0eee9] px-1 text-[10px] font-semibold text-[#6b6862] shadow-[0_0_0_2px_#fffefa] dark:bg-[#37352f] dark:text-[#c4c0b7] dark:shadow-[0_0_0_2px_#242320]">+{runs.length - 4}</span> : null}
          </span>
          {attentionCount > 0 ? (
            <span className="agent-dock-attention-badge flex shrink-0 items-center gap-1.5 rounded-full border border-[#f5dcb3] bg-[#fff7ea] py-1 pe-[11px] ps-[9px] text-[12px] font-semibold text-[#b45309] dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
              <span className="agent-dock-attention-dot h-1.5 w-1.5 rounded-full bg-[#d97706]" />
              {attentionCount} need you
            </span>
          ) : null}
          <span aria-hidden className="pe-2 text-[12px] text-[#a5a29b]">{open ? '⌄' : '⌃'}</span>
        </>
      ) : null}
    </button>
  );
}

function EmptyRunPane({ onNewChat }: { onNewChat: () => void }) {
  return (
    <div className="grid min-h-0 flex-1 place-items-center px-6 text-center">
      <div>
        <p className="text-[13px] text-[#8a8781]">No agents running. Describe a task in a new chat.</p>
        <button type="button" onClick={onNewChat} className="mt-3 rounded-[9px] bg-[#1c1b19] px-3 py-2 text-[12.5px] font-semibold text-white hover:bg-[#34322e] dark:bg-[#eeeae1] dark:text-[#1c1b19]">New chat or task</button>
      </div>
    </div>
  );
}

function EmptyChatPane({ onNewChat }: { onNewChat: () => void }) {
  return (
    <div className="grid min-h-0 flex-1 place-items-center px-6 text-center">
      <div>
        <p className="text-[13px] text-[#8a8781]">No conversations yet.</p>
        <button type="button" onClick={onNewChat} className="mt-3 rounded-[9px] bg-[#1c1b19] px-3 py-2 text-[12.5px] font-semibold text-white hover:bg-[#34322e] dark:bg-[#eeeae1] dark:text-[#1c1b19]">Start a conversation</button>
      </div>
    </div>
  );
}
