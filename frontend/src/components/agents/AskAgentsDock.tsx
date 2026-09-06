import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import { ArrowDown01Icon, Cancel01Icon, CheckmarkCircle02Icon, CollapseIcon, ExpandIcon, GlobeIcon, LinkSquare01Icon, LockIcon, MoreVerticalIcon, UserGroupIcon } from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { AskAgentAvatar, type AskAgentAvatarState } from '@/components/agents/AskAgentAvatar';
import { deriveAskAgentAvatarState } from '@/components/agents/askAgentPresence';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
	DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Skeleton } from '@/components/ui/skeleton';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useDockStore } from '@/stores/dockStore';
import { dockChatService } from '@/lib/services/dockChatService';
import { dockChatModuleForContext, type DockChat, type DockChatVisibility, type DockRunSummary } from '@/lib/dockTypes';
import type { CommandBarPageContext } from '@/lib/pmTypes';
import { DockRoster } from './dock/DockRoster';
import { ChatView } from './dock/ChatView';
import { DockRunView } from './dock/DockRunView';
import { useAgentRunStream, type AgentRunStreamFetchers } from './dock/useAgentRunStream';
import { dockRunContext, dockRunTitle, presentDockRun } from './dock/dockPresentation';
import { buildCodingSessionPath } from '@/lib/codingSessionSurface';
import { AnimatedDockChatTitle } from './dock/AnimatedDockChatTitle';
import { usePageContext } from '@/components/command-bar/pageContext';
import { PublicShareMenuActions } from './PublicShareMenuActions';

type AskAgentsEventDetail = {
  query?: string;
  mode?: 'compose' | 'runs';
  intent?: 'new_chat' | 'resume';
  runId?: string;
  chatId?: string;
};
type DockFocusTarget = 'composer' | 'selection' | 'header';

/** Persistent workspace presence layer for chats and user-owned agent runs. */
interface AskAgentsDockProps {
  presentation?: 'floating' | 'embedded';
  requiredPageContext?: CommandBarPageContext | null;
  associatedSupportConversationId?: string;
  active?: boolean;
  hideCollapsedTrigger?: boolean;
  onClose?: () => void;
}

export function AskAgentsDock({
  presentation = 'floating',
  requiredPageContext,
  associatedSupportConversationId,
  active = true,
  hideCollapsedTrigger = false,
  onClose,
}: AskAgentsDockProps = {}) {
  const embedded = presentation === 'embedded';
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const currentUserId = useAuthStore((state) => state.user?.id);
  const pageContext = usePageContext();
  const creationModule = dockChatModuleForContext(requiredPageContext ?? pageContext);
  const {
    collapsed,
    setCollapsed,
    tab: globalTab,
    setTab: setGlobalTab,
    activeChatId: globalActiveChatId,
    setActiveChatId: setGlobalActiveChatId,
    activeRunId: globalActiveRunId,
    setActiveRunId: setGlobalActiveRunId,
    chats,
    setChats,
    upsertChat,
    drafts,
    setDraft,
    clearDraft,
    lastAttentionIds,
    setLastAttentionIds,
    activateWorkspace,
  } = useDockStore();
  const [embeddedActiveChatId, setEmbeddedActiveChatId] = useState<string | null>(null);
  const tab = embedded ? 'chats' : globalTab;
  const activeChatId = embedded ? embeddedActiveChatId : globalActiveChatId;
  const activeRunId = embedded ? null : globalActiveRunId;
  const setTab = useCallback((next: 'agents' | 'chats') => {
    if (!embedded) setGlobalTab(next);
  }, [embedded, setGlobalTab]);
  const setActiveChatId = useCallback((chatId: string | null) => {
    if (embedded) setEmbeddedActiveChatId(chatId);
    else setGlobalActiveChatId(chatId);
  }, [embedded, setGlobalActiveChatId]);
  const setActiveRunId = useCallback((runId: string | null) => {
    if (!embedded) setGlobalActiveRunId(runId);
  }, [embedded, setGlobalActiveRunId]);
  const workspaceId = workspace?.id;
  useEffect(() => {
    if (embedded || !workspaceId || typeof window === 'undefined') return;
    const url = new URL(window.location.href);
    const sharedChatID = url.searchParams.get('ask_chat')?.trim();
    if (!sharedChatID) return;
    setTab('chats');
    setActiveChatId(sharedChatID);
    setCollapsed(false);
    url.searchParams.delete('ask_chat');
    window.history.replaceState(window.history.state, '', url);
  }, [embedded, setActiveChatId, setCollapsed, setTab, workspaceId]);
  const [runs, setRuns] = useState<DockRunSummary[]>([]);
  const [runsLoading, setRunsLoading] = useState(true);
  const [chatsLoading, setChatsLoading] = useState(true);
  const [runsError, setRunsError] = useState<string | null>(null);
  const [chatsError, setChatsError] = useState<string | null>(null);
  const [hiddenByModal, setHiddenByModal] = useState(false);
  const [maximized, setMaximized] = useState(false);
  const [chatScrollRequest, setChatScrollRequest] = useState(0);
  const [pendingDraft, setPendingDraft] = useState<string | undefined>();
  const [attentionNudge, setAttentionNudge] = useState(false);
  const [nextChatCursor, setNextChatCursor] = useState<string | null>(null);
  const [loadingMoreChats, setLoadingMoreChats] = useState(false);
  const [nextRunCursor, setNextRunCursor] = useState<string | null>(null);
  const [loadingMoreRuns, setLoadingMoreRuns] = useState(false);
  const [chatPresenceOverride, setChatPresenceOverride] = useState<{
    chatId: string;
    state: AskAgentAvatarState;
  } | null>(null);
  const [chatRunOverride, setChatRunOverride] = useState<{
    chatId: string;
    runId: string | null;
  } | null>(null);
  const [draftChat, setDraftChat] = useState(false);
  const [supportChatError, setSupportChatError] = useState<{ associationKey: string; message: string } | null>(null);
  const [supportChatRetry, setSupportChatRetry] = useState(0);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const askTriggerRef = useRef<HTMLButtonElement | null>(null);
  const returnFocusRef = useRef<HTMLElement | null>(null);
  const focusTargetRef = useRef<DockFocusTarget>('header');
  const loadingMoreChatsRef = useRef(false);
  const loadingMoreRunsRef = useRef(false);
  const ensuredSupportConversationRef = useRef<string | null>(null);
  const draftStoreKey = activeChatId
    ? `chat:${activeChatId}`
    : embedded
      ? `support:${associatedSupportConversationId ?? 'unknown'}:draft`
      : 'global:draft';
  const supportAssociationKey = workspaceId && associatedSupportConversationId
    ? `${workspaceId}:${associatedSupportConversationId}`
    : null;

  const orderedRuns = useMemo(() => [...runs].sort((left, right) => {
    const leftPresentation = presentDockRun(left.run.status, left.run.pause_reason, left.attention_kind);
    const rightPresentation = presentDockRun(right.run.status, right.run.pause_reason, right.attention_kind);
    const groupOrder = { needs_you: 0, running: 1, recent: 2 } as const;
    const groupDelta = groupOrder[leftPresentation.group] - groupOrder[rightPresentation.group];
    return groupDelta || Date.parse(right.last_activity_at) - Date.parse(left.last_activity_at);
  }), [runs]);
  const attentionRuns = useMemo(() => orderedRuns.filter((summary) => (
    presentDockRun(summary.run.status, summary.run.pause_reason, summary.attention_kind).group === 'needs_you'
  )), [orderedRuns]);
  // The minimized bar is a live-status surface, not run history. Keep only
  // queued/running work and paused runs that require the user's attention;
  // terminal and passively paused runs remain available in the expanded roster.
  const triggerRuns = useMemo(() => orderedRuns.filter((summary) => {
    const presentation = presentDockRun(summary.run.status, summary.run.pause_reason, summary.attention_kind);
    return presentation.group === 'needs_you'
      || summary.run.status === 'queued'
      || summary.run.status === 'running';
  }), [orderedRuns]);
  const activeRun = orderedRuns.find((summary) => summary.run.id === activeRunId) ?? null;
  // Keep untouched drafts out of the global roster. An empty support-linked
  // row can still be selected from its support conversation, but it should not
  // appear as an "Untitled chat" in the user's general Ask history.
  const visibleChats = useMemo(() => chats.filter((chat) => (
    chat.id === activeChatId || chat.title.trim() !== '' || chat.last_message_at != null
  )), [activeChatId, chats]);
  const activeChat = visibleChats.find((chat) => chat.id === activeChatId) ?? null;
  const chatViewKey = draftChat
    ? embedded
      ? `support:${associatedSupportConversationId ?? 'unknown'}:draft`
      : 'global:draft'
    : activeChat?.id ?? draftStoreKey;
  const activeChatMatchesSupportConversation = Boolean(
    activeChat && activeChat.support_conversation_id === associatedSupportConversationId,
  );
  const currentSupportChatError = supportChatError?.associationKey === supportAssociationKey
    ? supportChatError.message
    : null;
  const resolvingSupportChat = Boolean(
    embedded && active && supportAssociationKey && !activeChatMatchesSupportConversation && !draftChat && !currentSupportChatError,
  );
  const selectedChatId = activeChat?.id ?? null;
  const activeChatRunId = chatRunOverride?.chatId === selectedChatId
    ? chatRunOverride.runId
    : activeChat?.active_run_id ?? null;
  const chatStreamFetchers = useMemo<AgentRunStreamFetchers>(() => ({
    getSnapshot: (ws) => selectedChatId
      ? dockChatService.getChatRun(ws, selectedChatId)
      : Promise.resolve({ data: null, error: null }),
    listEvents: (ws, _runId, after) => selectedChatId
      ? dockChatService.listChatRunEvents(ws, selectedChatId, after)
      : Promise.resolve({ data: null, error: null }),
  }), [selectedChatId]);
  const chatStreamController = useAgentRunStream(
    workspaceId,
    activeChatRunId,
    !!activeChatRunId,
    5_000,
    chatStreamFetchers,
  );
  const askAgentState = chatPresenceOverride?.chatId === selectedChatId
    ? chatPresenceOverride.state
    : deriveAskAgentAvatarState({
    run: chatStreamController.session,
    stream: chatStreamController.streamState,
  });
  const handleChatPresenceChange = useCallback((state: AskAgentAvatarState | null) => {
    setChatPresenceOverride(state && selectedChatId ? { chatId: selectedChatId, state } : null);
  }, [selectedChatId]);
  const handleChatRunIdChange = useCallback((runId: string | null) => {
    if (selectedChatId) setChatRunOverride({ chatId: selectedChatId, runId });
  }, [selectedChatId]);

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
    setNextRunCursor(result.data.next_cursor ?? null);
    setRunsLoading(false);
    return result.data.runs ?? [];
  }, [workspaceId]);

  const loadMoreRuns = useCallback(async () => {
    if (!workspaceId || !nextRunCursor || loadingMoreRunsRef.current) return;
    loadingMoreRunsRef.current = true;
    setLoadingMoreRuns(true);
    const cursor = nextRunCursor;
    const result = await dockChatService.listRuns(workspaceId, cursor);
    if (useWorkspaceStore.getState().currentWorkspace?.id === workspaceId) {
      if (result.error || !result.data) {
        toast.error(result.error ?? 'Unable to load more agent runs');
      } else {
        setRuns((current) => {
          const known = new Set(current.map((summary) => summary.run.id));
          return [...current, ...(result.data?.runs ?? []).filter((summary) => !known.has(summary.run.id))];
        });
        setNextRunCursor(result.data.next_cursor ?? null);
      }
    }
    loadingMoreRunsRef.current = false;
    setLoadingMoreRuns(false);
  }, [nextRunCursor, workspaceId]);

  const refreshChats = useCallback(async (preserveLoaded = false) => {
    if (!workspaceId) return;
    setChatsError(null);
    const result = await dockChatService.listChats(workspaceId);
    if (useWorkspaceStore.getState().currentWorkspace?.id !== workspaceId) return;
    if (result.error || !result.data) {
      setChatsError(result.error ?? 'Unable to load conversations');
      setChatsLoading(false);
      return;
    }
    const firstPage = result.data.chats ?? [];
    const currentChats = useDockStore.getState().chats;
    const currentById = new Map(currentChats.map((chat) => [chat.id, chat]));
    const mergedFirstPage = firstPage.map((chat) => {
      const current = currentById.get(chat.id);
      if (!chat.active_run_status && current
        && current.active_run_id === chat.active_run_id && current.active_run_status) {
        return { ...chat, active_run_status: current.active_run_status };
      }
      return chat;
    });
    if (preserveLoaded) {
      const firstPageIds = new Set(firstPage.map((chat) => chat.id));
      const existing = currentChats.filter((chat) => !firstPageIds.has(chat.id));
      setChats([...mergedFirstPage, ...existing]);
    } else {
      setChats(mergedFirstPage);
    }
    setNextChatCursor(result.data.next_cursor ?? null);
    setChatsLoading(false);
  }, [setChats, workspaceId]);

  const updateChatRunStatus = useCallback((runId: string | null, status: DockChat['active_run_status']) => {
    if (!runId || !status) return;
    const current = useDockStore.getState().chats;
    let changed = false;
    const next = current.map((chat) => {
      if (chat.active_run_id !== runId || chat.active_run_status === status) return chat;
      changed = true;
      return { ...chat, active_run_status: status };
    });
    if (changed) setChats(next);
  }, [setChats]);

  const loadMoreChats = useCallback(async () => {
    if (!workspaceId || !nextChatCursor || loadingMoreChatsRef.current) return;
    loadingMoreChatsRef.current = true;
    setLoadingMoreChats(true);
    const cursor = nextChatCursor;
    const result = await dockChatService.listChats(workspaceId, cursor);
    if (useWorkspaceStore.getState().currentWorkspace?.id === workspaceId) {
      if (result.error || !result.data) {
        toast.error(result.error ?? 'Unable to load more conversations');
      } else {
        const existing = useDockStore.getState().chats;
        const knownIds = new Set(existing.map((chat) => chat.id));
        setChats([...existing, ...(result.data.chats ?? []).filter((chat) => !knownIds.has(chat.id))]);
        setNextChatCursor(result.data.next_cursor ?? null);
      }
    }
    loadingMoreChatsRef.current = false;
    setLoadingMoreChats(false);
  }, [nextChatCursor, setChats, workspaceId]);

  const renameChat = useCallback(async (chatId: string, title: string) => {
    if (!workspaceId) return false;
    const result = await dockChatService.updateChat(workspaceId, chatId, { title });
    if (result.error || !result.data) {
      toast.error(result.error ?? 'Failed to rename conversation');
      return false;
    }
    setChats(useDockStore.getState().chats.map((chat) => chat.id === chatId ? result.data! : chat));
    return true;
  }, [setChats, workspaceId]);

  const archiveChat = useCallback(async (chatId: string) => {
    if (!workspaceId) return false;
    const result = await dockChatService.updateChat(workspaceId, chatId, { archived: true });
    if (result.error || !result.data) {
      toast.error(result.error ?? 'Failed to archive conversation');
      return false;
    }
    const remaining = useDockStore.getState().chats.filter((chat) => chat.id !== chatId);
    setChats(remaining);
    if (useDockStore.getState().activeChatId === chatId) setActiveChatId(remaining[0]?.id ?? null);
    return true;
  }, [setActiveChatId, setChats, workspaceId]);

  const updateChatVisibility = useCallback(async (chatId: string, visibility: DockChatVisibility) => {
    if (!workspaceId) return false;
    const result = await dockChatService.updateChat(workspaceId, chatId, { visibility });
    if (result.error || !result.data) {
      toast.error(result.error ?? 'Failed to update who can see this chat');
      return false;
    }
    setChats(useDockStore.getState().chats.map((chat) => chat.id === chatId ? result.data! : chat));
    return true;
  }, [setChats, workspaceId]);

  useEffect(() => {
    if (!workspaceId) return;
    activateWorkspace(workspaceId);
    let cancelled = false;
    queueMicrotask(() => {
      if (cancelled) return;
      setRuns([]);
      setRunsLoading(true);
      setNextRunCursor(null);
      loadingMoreRunsRef.current = false;
      setLoadingMoreRuns(false);
      setChatsLoading(true);
      setNextChatCursor(null);
      loadingMoreChatsRef.current = false;
      setLoadingMoreChats(false);
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
    if (embedded || chatsLoading || draftChat) return;
    const current = useDockStore.getState().activeChatId;
    if (current && visibleChats.some((chat) => chat.id === current)) return;
    setActiveChatId(visibleChats[0]?.id ?? null);
  }, [chatsLoading, draftChat, embedded, setActiveChatId, visibleChats]);

  useEffect(() => {
    if (!embedded) return;
    ensuredSupportConversationRef.current = null;
    setEmbeddedActiveChatId(null);
    setDraftChat(false);
    setSupportChatError(null);
  }, [associatedSupportConversationId, embedded]);

  useEffect(() => {
    if (!active || !workspaceId || !associatedSupportConversationId || chatsLoading) return;
    const associationKey = `${workspaceId}:${associatedSupportConversationId}`;
    const associatedChat = chats.find(
      (chat) => chat.support_conversation_id === associatedSupportConversationId,
    );
    if (associatedChat) {
      setDraftChat(false);
      setSupportChatError(null);
      ensuredSupportConversationRef.current = associationKey;
      if (activeChatId !== associatedChat.id) {
        setActiveChatId(associatedChat.id);
      }
      setTab('chats');
      return;
    }
    if (ensuredSupportConversationRef.current === associationKey) return;
    ensuredSupportConversationRef.current = associationKey;
    setSupportChatError(null);
    void dockChatService.findSupportConversationChat(workspaceId, associatedSupportConversationId).then((result) => {
      if (ensuredSupportConversationRef.current !== associationKey) return;
      if (result.error) {
        ensuredSupportConversationRef.current = null;
        const message = result.error;
        setSupportChatError({ associationKey, message });
        toast.error(message);
        return;
      }
      if (!result.data) {
        setDraftChat(true);
        setEmbeddedActiveChatId(null);
        setSupportChatError(null);
        focusTargetRef.current = 'composer';
        return;
      }
      upsertChat(result.data);
      setDraftChat(false);
      setSupportChatError(null);
      setActiveChatId(result.data.id);
      setTab('chats');
      focusTargetRef.current = 'composer';
    });
  }, [active, activeChatId, associatedSupportConversationId, chats, chatsLoading, setActiveChatId, setTab, supportChatRetry, upsertChat, workspaceId]);

  const retrySupportChat = useCallback(() => {
    if (!supportAssociationKey) return;
    ensuredSupportConversationRef.current = null;
    setSupportChatError(null);
    setSupportChatRetry((current) => current + 1);
  }, [supportAssociationKey]);

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
    const refresh = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string; status?: DockChat['active_run_status'] }>).detail;
      updateChatRunStatus(detail?.entity_id ?? null, detail?.status ?? null);
      if (timer) return;
      timer = setTimeout(() => {
        timer = null;
        void Promise.all([refreshRuns(), refreshChats(true)]);
      }, 180);
    };
    window.addEventListener('agent_run-updated', refresh);
    window.addEventListener('coding_session-updated', refresh);
    return () => {
      if (timer) clearTimeout(timer);
      window.removeEventListener('agent_run-updated', refresh);
      window.removeEventListener('coding_session-updated', refresh);
    };
  }, [refreshChats, refreshRuns, updateChatRunStatus, workspaceId]);

  const rememberFocusSource = useCallback((source?: HTMLElement | null) => {
    const active = document.activeElement;
    returnFocusRef.current = source ?? (active instanceof HTMLElement ? active : askTriggerRef.current);
  }, []);

  const openDock = useCallback((
    targetTab?: 'agents' | 'chats',
    focusTarget: DockFocusTarget = targetTab === 'chats' ? 'composer' : 'selection',
    source?: HTMLElement | null,
  ) => {
    rememberFocusSource(source);
    focusTargetRef.current = focusTarget;
    if (targetTab) setTab(targetTab);
    setCollapsed(false);
  }, [rememberFocusSource, setCollapsed, setTab]);

  const closeDock = useCallback(() => {
    if (embedded) {
      onClose?.();
      return;
    }
    setMaximized(false);
    setCollapsed(true);
    window.setTimeout(() => {
      const target = returnFocusRef.current;
      if (target?.isConnected) target.focus();
      else askTriggerRef.current?.focus();
    }, 0);
  }, [embedded, onClose, setCollapsed]);

  const newChat = useCallback(() => {
    if (embedded) return;
    clearDraft('global:draft');
    setDraftChat(true);
    setActiveChatId(null);
    setTab('chats');
    focusTargetRef.current = 'composer';
    setCollapsed(false);
  }, [clearDraft, embedded, setActiveChatId, setCollapsed, setTab]);

  const createDraftChat = useCallback(async () => {
    if (!workspaceId) return null;
    const supportConversationId = embedded ? associatedSupportConversationId : undefined;
    const moduleId = embedded ? 'support' : creationModule;
    const result = await dockChatService.createChat(workspaceId, '', supportConversationId, moduleId);
    if (result.error || !result.data) {
      toast.error(result.error ?? 'Failed to create chat');
      return null;
    }
    upsertChat(result.data);
    setActiveChatId(result.data.id);
    setTab('chats');
    return result.data;
  }, [associatedSupportConversationId, creationModule, embedded, setActiveChatId, setTab, upsertChat, workspaceId]);

  useLayoutEffect(() => {
    if (collapsed && !embedded) return;
    if (!panelRef.current) return;
    const target = focusTargetRef.current;
    if (target === 'composer' && textareaRef.current) {
      textareaRef.current.focus();
      return;
    }
    if (target === 'selection') {
      const selected = panelRef.current.querySelector<HTMLElement>('[aria-current="true"]');
      if (selected) {
        selected.focus();
        return;
      }
    }
    panelRef.current.querySelector<HTMLElement>('[data-dock-header]')?.focus();
  }, [activeChatId, activeRunId, collapsed, embedded, tab]);

  useEffect(() => {
    if (embedded) return;
    const onKey = (event: KeyboardEvent) => {
      const target = event.target;
      const editable = target instanceof Element
        && target.matches('input, textarea, select, [contenteditable="true"]');
      if (event.key === '/' && !editable && !event.metaKey && !event.ctrlKey && !event.altKey) {
        event.preventDefault();
        openDock('chats', 'composer');
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
  }, [closeDock, collapsed, embedded, newChat, openDock]);

  useEffect(() => {
    if (embedded || collapsed) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Tab' || !panelRef.current) return;
      const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      )).filter((element) => element.offsetParent !== null);
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;
      if (!(active instanceof Node) || !panelRef.current.contains(active)) {
        event.preventDefault();
        (event.shiftKey ? last : first).focus();
      } else if (event.shiftKey && active === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [collapsed, embedded]);

  useEffect(() => {
    if (embedded || collapsed) return;
    const previousBodyOverflow = document.body.style.overflow;
    const previousRootOverflow = document.documentElement.style.overflow;
    document.body.style.overflow = 'hidden';
    document.documentElement.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previousBodyOverflow;
      document.documentElement.style.overflow = previousRootOverflow;
    };
  }, [collapsed, embedded]);

  useEffect(() => {
    if (embedded) return;
    const onAsk = (event: Event) => {
      const detail = (event as CustomEvent<AskAgentsEventDetail>).detail ?? {};
      const query = detail.query?.trim();
      if (detail.intent === 'new_chat') {
        newChat();
        if (query) setPendingDraft(query);
        return;
      }
      if (detail.runId) {
        setActiveRunId(detail.runId);
        openDock('agents', 'selection');
        return;
      }
      if (detail.chatId) {
        setActiveChatId(detail.chatId);
        openDock('chats', 'composer');
      } else {
        openDock(detail.mode === 'runs' ? 'agents' : 'chats', detail.mode === 'runs' ? 'selection' : 'composer');
      }
      if (query) setPendingDraft(query);
    };
    window.addEventListener('helpin:ask-agents', onAsk);
    return () => window.removeEventListener('helpin:ask-agents', onAsk);
  }, [embedded, newChat, openDock, setActiveChatId, setActiveRunId]);

  useEffect(() => {
    const compute = () => {
      const dialogs = document.querySelectorAll('[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]');
      let hidden = false;
      dialogs.forEach((dialog) => {
        const dockOwned = dialog.closest('[data-helpin-dock], [data-helpin-dock-overlay]');
        if (!dockOwned) hidden = true;
      });
      setHiddenByModal(hidden);
    };
    compute();
    const observer = new MutationObserver(compute);
    observer.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-state'] });
    return () => observer.disconnect();
  }, []);

  if (!workspaceId || typeof document === 'undefined') return null;

  if (embedded) {
    return (
      <div
        ref={panelRef}
        id="support-agent-sidebar"
        data-helpin-dock="true"
        data-helpin-dock-presentation="embedded"
        className="flex h-full min-h-0 w-full flex-col overflow-hidden bg-[#fffefa] dark:bg-[#242320]"
      >
        <DockPaneHeader
          key={`embedded:${activeChat?.id ?? 'empty'}`}
          tab="chats"
          run={null}
          chat={activeChat}
          workspaceSlug={workspace.slug}
          workspaceName={workspace.name}
          currentUserId={currentUserId}
          askAgentState={askAgentState}
          onClose={closeDock}
          closeLabel="Back to details"
          maximized={false}
          onToggleMaximized={() => {}}
          allowMaximize={false}
          onRenameChat={renameChat}
          onArchiveChat={archiveChat}
          onUpdateVisibility={updateChatVisibility}
          chatPlaceholder={resolvingSupportChat
            ? { title: 'Opening chat…', subtitle: 'Loading conversation context' }
            : currentSupportChatError
              ? { title: 'Conversation chat', subtitle: 'Unable to open' }
              : undefined}
        />
        {resolvingSupportChat ? (
          <SupportChatLoadingPane />
        ) : currentSupportChatError ? (
          <SupportChatErrorPane message={currentSupportChatError} onRetry={retrySupportChat} />
        ) : activeChat || draftChat ? (
          <ChatView
            // Creating the first chat must preserve the pending send and run state.
            key={supportAssociationKey ?? chatViewKey}
            workspaceId={workspaceId}
            chatId={activeChat?.id}
            onCreateChat={createDraftChat}
            textareaRef={textareaRef}
            draftValue={drafts[draftStoreKey] ?? ''}
            onDraftChange={(value) => setDraft(draftStoreKey, value)}
            onChatChanged={() => {
              setDraftChat(false);
              void refreshChats(true);
            }}
            onRunStatusChange={updateChatRunStatus}
            streamController={chatStreamController}
            onPresenceChange={handleChatPresenceChange}
            onRunIdChange={handleChatRunIdChange}
            requiredPageContext={requiredPageContext}
            showComposerShortcutHint={requiredPageContext?.entity_type !== 'support_conversation'}
            scrollToLatestRequest={chatScrollRequest}
          />
        ) : (
          <EmptyChatPane onNewChat={() => void newChat()} />
        )}
      </div>
    );
  }

  return createPortal(
    <div
      data-helpin-dock="true"
      aria-hidden={hiddenByModal || undefined}
      className={cn('agent-dock-root fixed inset-0 z-[60]', hiddenByModal && 'pointer-events-none opacity-0')}
    >
      {!collapsed ? (
        <button type="button" aria-label="Close agent dock" tabIndex={-1} onClick={closeDock} className="agent-dock-scrim absolute inset-0 bg-[rgba(28,27,25,.10)] backdrop-blur-[1.5px] dark:bg-black/35" />
      ) : null}

      <div className={cn(
        'agent-dock-anchor pointer-events-none absolute inset-0 flex flex-col items-center justify-end motion-safe:transition-[padding] motion-safe:duration-300 motion-safe:ease-[cubic-bezier(0.22,1,0.36,1)]',
        maximized
          ? 'gap-0 p-0'
          : 'gap-2.5 px-3 pb-[calc(22px+env(safe-area-inset-bottom))]',
      )}>
        {!collapsed ? (
          <div
            ref={panelRef}
            id="agent-dock-panel"
            role="dialog"
            aria-modal="true"
            aria-label="Agent runs and chats"
            data-maximized={maximized || undefined}
            tabIndex={-1}
            className={cn(
              'agent-dock-panel pointer-events-auto flex origin-bottom overflow-hidden bg-[#fffefa] will-change-[width,height] motion-safe:transition-[width,height,min-height,border-radius,box-shadow] motion-safe:duration-300 motion-safe:ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none dark:bg-[#242320]',
              maximized
                ? 'h-full w-full min-h-0'
                : 'h-[min(600px,calc(100dvh-104px))] w-[min(900px,92vw)] min-h-[360px] rounded-[18px] border border-[#e6e3dd] shadow-[0_30px_70px_-26px_rgba(28,27,25,.5)] dark:border-[#37352f]',
            )}
          >
            <DockRoster
              workspaceId={workspaceId}
              tab={tab}
              runs={orderedRuns}
              chats={visibleChats}
              selectedRunId={activeRunId}
              selectedChatId={activeChatId}
              loadingRuns={runsLoading}
              loadingChats={chatsLoading}
              runsError={runsError}
              chatsError={chatsError}
              onTabChange={(next) => {
                focusTargetRef.current = 'selection';
                setTab(next);
                if (next === 'agents' && !activeRunId) setActiveRunId(orderedRuns[0]?.run.id ?? null);
                if (next === 'chats') {
                  if (!activeChatId) setActiveChatId(visibleChats[0]?.id ?? null);
                  setChatScrollRequest((request) => request + 1);
                }
              }}
              onSelectRun={(runId) => { focusTargetRef.current = 'selection'; setActiveRunId(runId); setTab('agents'); }}
              onSelectChat={(chatId) => {
                focusTargetRef.current = 'selection';
                setDraftChat(false);
                setActiveChatId(chatId);
                setTab('chats');
                setChatScrollRequest((request) => request + 1);
              }}
              onNewChat={() => void newChat()}
              onRenameChat={renameChat}
              onArchiveChat={archiveChat}
              hasMoreChats={!!nextChatCursor}
              loadingMoreChats={loadingMoreChats}
              onLoadMoreChats={() => void loadMoreChats()}
              hasMoreRuns={!!nextRunCursor}
              loadingMoreRuns={loadingMoreRuns}
              onLoadMoreRuns={() => void loadMoreRuns()}
              onRetryRuns={() => void refreshRuns()}
              onRetryChats={() => void refreshChats()}
            />
            <section className="flex min-w-0 flex-1 flex-col bg-[#fffefa] dark:bg-[#242320]">
              <DockPaneHeader
                key={tab === 'agents' ? `agents:${activeRun?.run.id ?? 'empty'}` : `chats:${activeChat?.id ?? 'empty'}`}
                tab={tab}
                run={activeRun}
                chat={activeChat}
                workspaceSlug={workspace.slug}
                workspaceName={workspace.name}
                currentUserId={currentUserId}
                askAgentState={askAgentState}
                maximized={maximized}
                onToggleMaximized={() => setMaximized((value) => !value)}
                onClose={closeDock}
                onRenameChat={renameChat}
                onArchiveChat={archiveChat}
                onUpdateVisibility={updateChatVisibility}
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
              ) : activeChat || draftChat ? (
                <ChatView
                  key={chatViewKey}
                  workspaceId={workspaceId}
                  chatId={activeChat?.id}
                  onCreateChat={createDraftChat}
                  scrollToLatestRequest={chatScrollRequest}
                  textareaRef={textareaRef}
                  initialDraft={pendingDraft}
                  onDraftConsumed={() => setPendingDraft(undefined)}
                  draftValue={drafts[draftStoreKey] ?? ''}
                  onDraftChange={(value) => setDraft(draftStoreKey, value)}
                  onChatChanged={() => {
                    setDraftChat(false);
                    void refreshChats(true);
                  }}
                  onRunStatusChange={updateChatRunStatus}
                  streamController={chatStreamController}
                  onPresenceChange={handleChatPresenceChange}
                  onRunIdChange={handleChatRunIdChange}
                />
              ) : (
                <EmptyChatPane onNewChat={() => void newChat()} />
              )}
            </section>
          </div>
        ) : null}

        {!maximized && !(hideCollapsedTrigger && collapsed) ? <DockTrigger
          askTriggerRef={askTriggerRef}
          open={!collapsed}
          runs={triggerRuns}
          attentionCount={attentionRuns.length}
          nudge={attentionNudge}
          askAgentState={askAgentState}
          onAsk={(source) => openDock('chats', 'composer', source)}
          onRun={(runId, source) => {
            setActiveRunId(runId);
            openDock('agents', 'selection', source);
          }}
          onAttention={(source) => {
            const firstAttention = attentionRuns[0] ?? orderedRuns[0];
            if (firstAttention) setActiveRunId(firstAttention.run.id);
            openDock('agents', 'selection', source);
          }}
          onToggle={(source) => {
            if (collapsed) openDock(tab, tab === 'chats' ? 'composer' : 'selection', source);
            else closeDock();
          }}
        /> : null}
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
  workspaceName,
  currentUserId,
  askAgentState,
  maximized,
  onToggleMaximized,
  onClose,
  onRenameChat,
  onArchiveChat,
  onUpdateVisibility,
  closeLabel = 'Minimize',
  allowMaximize = true,
  chatPlaceholder,
}: {
  tab: 'agents' | 'chats';
  run: DockRunSummary | null;
  chat: DockChat | null;
  workspaceSlug?: string;
  workspaceName: string;
  currentUserId?: string;
  askAgentState: AskAgentAvatarState;
  maximized: boolean;
  onToggleMaximized: () => void;
  onClose: () => void;
  onRenameChat: (chatId: string, title: string) => Promise<boolean>;
  onArchiveChat: (chatId: string) => Promise<boolean>;
  onUpdateVisibility: (chatId: string, visibility: DockChatVisibility) => Promise<boolean>;
  closeLabel?: string;
  allowMaximize?: boolean;
  chatPlaceholder?: { title: string; subtitle: string };
}) {
  const [editingTitle, setEditingTitle] = useState(false);
  const [title, setTitle] = useState(chat?.title ?? '');
  const headerRun = tab === 'agents' ? run : null;
  const presentation = headerRun ? presentDockRun(headerRun.run.status, headerRun.run.pause_reason, headerRun.attention_kind) : null;
  const fullPath = headerRun ? buildCodingSessionPath(workspaceSlug, headerRun.run.id) : null;

  const commitTitle = async () => {
    if (!chat) return;
    const nextTitle = title.trim();
    setEditingTitle(false);
    if (!nextTitle || nextTitle === chat.title) return;
    const updated = await onRenameChat(chat.id, nextTitle);
    if (!updated) setTitle(chat.title);
  };

  return (
    <header data-dock-header tabIndex={-1} className="flex min-h-[51px] items-center gap-2.5 border-b border-[#f1efea] px-3.5 py-2.5 outline-none dark:border-[#302f2b]">
      {run && tab === 'agents' ? (
        <AgentAvatar name={run.agent.name} presetKey={run.agent.preset_key} iconKey={run.agent.icon_key} className="h-[26px] w-[26px] rounded-[8px] border-0 shadow-none" />
      ) : (
        <AskAgentAvatar state={askAgentState} plateStyle="feather" className="h-[34px] w-[34px]" />
      )}
      <span className="min-w-0 flex-1">
        {editingTitle && chat ? (
          <input
            autoFocus
            aria-label="Conversation title"
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            onBlur={() => void commitTitle()}
            onKeyDown={(event) => {
              if (event.key === 'Enter') void commitTitle();
              if (event.key === 'Escape') { setTitle(chat.title); setEditingTitle(false); }
            }}
            maxLength={120}
            className="block w-full rounded-md border border-[#d8d3c9] bg-[#fffefa] px-1.5 py-1 text-[13.5px] font-semibold text-[#1c1b19] outline-none focus:border-[#a5a29b] dark:bg-[#242320] dark:text-[#eeeae1]"
          />
        ) : (
          <span className="block truncate text-[13.5px] font-semibold text-[#1c1b19] dark:text-[#eeeae1]">
            {tab === 'agents'
              ? (run ? dockRunTitle(run) : 'Agent runs')
              : <AnimatedDockChatTitle title={chat?.title.trim() || chatPlaceholder?.title || 'New chat'} />}
          </span>
        )}
        {tab === 'agents' && run ? (
          <span className="block truncate font-mono text-[10.5px] text-[#a5a29b]">{dockRunContext(run)}</span>
        ) : chat ? (
          <DockChatVisibilityControl
            chat={chat}
            workspaceName={workspaceName}
            editable={chat.user_id === currentUserId}
            onChange={(visibility) => onUpdateVisibility(chat.id, visibility)}
          />
        ) : (
          <span className="block truncate text-[10.5px] text-[#a5a29b]">{chatPlaceholder?.subtitle || 'Only you can see this'}</span>
        )}
      </span>
      {presentation ? (
        <span className="shrink-0 rounded-full px-2 py-[3px] text-[11px] font-semibold" style={{ backgroundColor: presentation.chipBackground, color: presentation.chipForeground }}>
          {presentation.label}
        </span>
      ) : null}
      <div data-dock-actions className="flex items-center gap-0.5">
      {fullPath ? (
        <a href={fullPath} aria-label="Open full agent session" title="Open full session" className="grid h-8 w-8 place-items-center rounded-md text-[#a5a29b] transition hover:bg-[#f4f2ee] hover:text-[#4b4945] dark:hover:bg-[#302f2b]">
          <LinkSquare01Icon className="h-3.5 w-3.5" />
        </a>
      ) : null}
      {(tab === 'chats' && chat) || (tab === 'agents' && run) ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button type="button" aria-label={tab === 'agents' ? 'Agent run actions' : 'Conversation actions'} className="agent-dock-header-action grid h-8 w-8 shrink-0 place-items-center rounded-md text-[#a5a29b] transition hover:bg-[#f4f2ee] hover:text-[#4b4945] dark:hover:bg-[#302f2b]">
              <MoreVerticalIcon className="h-3.5 w-3.5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="z-[70]">
			<PublicShareMenuActions
				workspaceId={run?.run.workspace_id ?? chat?.workspace_id ?? ''}
				resourceType={tab === 'agents' ? 'agent_run' : 'dock_chat'}
				resourceId={run?.run.id ?? chat?.id ?? ''}
			/>
			{tab === 'chats' && chat?.user_id === currentUserId ? <>
				<DropdownMenuSeparator />
				<DropdownMenuItem onSelect={() => { setTitle(chat?.title ?? ''); setEditingTitle(true); }}>Rename</DropdownMenuItem>
				<DropdownMenuItem className="text-destructive" onSelect={() => chat && void onArchiveChat(chat.id)}>Archive</DropdownMenuItem>
			</> : null}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
      {allowMaximize ? <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            onClick={onToggleMaximized}
            aria-label={maximized ? 'Restore agent dock' : 'Maximize agent dock'}
            className="agent-dock-header-action grid h-8 w-8 shrink-0 place-items-center rounded-md text-[#a5a29b] transition hover:bg-[#f4f2ee] hover:text-[#4b4945] dark:hover:bg-[#302f2b]"
          >
            {maximized
              ? <CollapseIcon className="h-3.5 w-3.5" />
              : <ExpandIcon className="h-3.5 w-3.5" />}
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="z-[70]">{maximized ? 'Restore' : 'Maximize'}</TooltipContent>
      </Tooltip> : null}
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button" onClick={onClose} aria-label={closeLabel} className="agent-dock-header-action grid h-8 w-8 shrink-0 place-items-center rounded-md text-[#a5a29b] transition hover:bg-[#f4f2ee] hover:text-[#4b4945] dark:hover:bg-[#302f2b]">
            <Cancel01Icon className="h-3.5 w-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="z-[70]">{closeLabel}</TooltipContent>
      </Tooltip>
      </div>
    </header>
  );
}

const DOCK_CHAT_MODULE_LABELS: Record<NonNullable<DockChat['module_id']>, string> = {
  support: 'Support',
  crm: 'CRM',
  pm: 'Projects',
  docs: 'Docs',
};

function DockChatVisibilityControl({
  chat,
  workspaceName,
  editable,
  onChange,
}: {
  chat: DockChat;
  workspaceName: string;
  editable: boolean;
  onChange: (visibility: DockChatVisibility) => Promise<boolean>;
}) {
  const [saving, setSaving] = useState(false);
  const confirm = useConfirm();
  const visibility = chat.visibility || 'private';
  const moduleLabel = chat.module_id ? DOCK_CHAT_MODULE_LABELS[chat.module_id] : null;
  const label = visibility === 'workspace'
    ? `Visible to everyone at ${workspaceName || 'this workspace'}`
    : visibility === 'module' && moduleLabel
      ? `Visible to teammates in ${moduleLabel}`
      : 'Only you can see this';
  const Icon = visibility === 'workspace' ? GlobeIcon : visibility === 'module' ? UserGroupIcon : LockIcon;

  if (!editable) {
    return (
      <span className="flex min-w-0 items-center gap-1 text-[10.5px] text-[#8a8781]" title={label}>
        <Icon className="h-3 w-3 shrink-0" />
        <span className="truncate">{label}</span>
      </span>
    );
  }

  const selectVisibility = async (next: DockChatVisibility) => {
    if (next === visibility || saving) return;
    if (next === 'private' && visibility !== 'private') {
      const confirmed = await confirm({
        title: 'Make this chat private?',
        description: 'Teammates who can see it now will lose access to the complete chat history.',
        confirmText: 'Make private',
        variant: 'destructive',
      });
      if (!confirmed) return;
    }
    setSaving(true);
    await onChange(next);
    setSaving(false);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          disabled={saving}
          className="flex max-w-full items-center gap-1 text-[10.5px] text-[#8a8781] transition hover:text-[#4b4945] disabled:opacity-60 dark:hover:text-[#d4d0c7]"
          aria-label={`${label}. Change who can see this chat`}
        >
          <Icon className="h-3 w-3 shrink-0" />
          <span className="truncate">{saving ? 'Updating visibility…' : label}</span>
          <ArrowDown01Icon className="h-3 w-3 shrink-0" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="z-[70] w-[290px] p-1.5">
        <div className="px-2 pb-2 pt-1">
          <p className="text-xs font-semibold">Who can see this chat?</p>
        </div>
        <DockChatVisibilityItem
          icon={<LockIcon className="h-4 w-4" />}
          label="Only me"
          description="Keep this chat personal."
          selected={visibility === 'private'}
          onSelect={() => void selectVisibility('private')}
        />
        {moduleLabel ? (
          <DockChatVisibilityItem
            icon={<UserGroupIcon className="h-4 w-4" />}
            label={`Teammates in ${moduleLabel}`}
            description={`Anyone with access to ${moduleLabel} can open this chat.`}
            selected={visibility === 'module'}
            onSelect={() => void selectVisibility('module')}
          />
        ) : null}
        <DockChatVisibilityItem
          icon={<GlobeIcon className="h-4 w-4" />}
          label={`Everyone at ${workspaceName || 'this workspace'}`}
          description="All workspace members can open this chat."
          selected={visibility === 'workspace'}
          onSelect={() => void selectVisibility('workspace')}
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function DockChatVisibilityItem({
  icon,
  label,
  description,
  selected,
  onSelect,
}: {
  icon: React.ReactNode;
  label: string;
  description: string;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <DropdownMenuItem onSelect={onSelect} className="items-start gap-2.5 rounded-lg px-2 py-2">
      <span className="mt-0.5 text-[#8a8781]">{icon}</span>
      <span className="min-w-0 flex-1">
        <span className="block text-xs font-medium">{label}</span>
        <span className="block text-[10px] leading-4 text-[#8a8781]">{description}</span>
      </span>
      {selected ? <CheckmarkCircle02Icon className="mt-0.5 h-4 w-4 shrink-0" /> : null}
    </DropdownMenuItem>
  );
}

function DockTrigger({
  askTriggerRef,
  open,
  runs,
  attentionCount,
  nudge,
  askAgentState,
  onAsk,
  onRun,
  onAttention,
  onToggle,
}: {
  askTriggerRef: React.RefObject<HTMLButtonElement | null>;
  open: boolean;
  runs: DockRunSummary[];
  attentionCount: number;
  nudge: boolean;
  askAgentState: AskAgentAvatarState;
  onAsk: (source: HTMLButtonElement) => void;
  onRun: (runId: string, source: HTMLButtonElement) => void;
  onAttention: (source: HTMLButtonElement) => void;
  onToggle: (source: HTMLButtonElement) => void;
}) {
  const visible = runs.slice(0, 4);
  return (
    <div
      role="group"
      aria-label="Agent dock"
      className={cn(
        'agent-dock-trigger pointer-events-auto flex min-h-[42px] max-w-[calc(100vw-24px)] items-center rounded-[26px] border border-[#e6e3dd] bg-[#fffefa] py-[4px] pe-1 ps-1 text-[#1c1b19] shadow-[0_12px_30px_-14px_rgba(28,27,25,.45)] transition-[transform,border-color] duration-200 hover:border-[#d2cec5] dark:border-[#37352f] dark:bg-[#242320] dark:text-[#eeeae1]',
        nudge && 'agent-dock-attention-nudge',
      )}
    >
      <button
        ref={askTriggerRef}
        type="button"
        aria-expanded={open}
        aria-controls="agent-dock-panel"
        aria-label="Ask Agent"
        onClick={(event) => onAsk(event.currentTarget)}
        className="agent-dock-trigger-segment flex min-h-8 min-w-0 items-center gap-2 rounded-full px-3 outline-none transition hover:bg-[#f4f2ee] focus-visible:ring-2 focus-visible:ring-[#a855f7]/45 dark:hover:bg-[#302f2b]"
      >
        <AskAgentAvatar state={askAgentState} plateStyle="feather" className="h-8 w-8" />
        <span className="truncate text-[13.5px] font-medium">Ask Agent</span>
        <kbd className="agent-dock-shortcut rounded-[5px] border border-[#eae7e0] px-[5px] py-px font-mono text-[11px] text-[#a5a29b] dark:border-[#3a3832]">/</kbd>
      </button>
      {runs.length > 0 ? (
        <>
          <span className="mx-1 h-[22px] w-px shrink-0 bg-[#eeece7] dark:bg-[#3a3832]" />
          <span className="flex shrink-0 items-center">
            {visible.map((summary) => {
              const presentation = presentDockRun(summary.run.status, summary.run.pause_reason, summary.attention_kind);
              return (
                <button
                  key={summary.run.id}
                  type="button"
                  aria-label={`Open ${dockRunTitle(summary)}, ${presentation.label}`}
                  title={`${dockRunTitle(summary)} — ${presentation.label}`}
                  onClick={(event) => onRun(summary.run.id, event.currentTarget)}
                  className="agent-dock-stack-item relative -ms-1.5 flex h-8 w-8 items-center justify-center rounded-[10px] leading-none outline-none first:ms-0 hover:z-[1] focus-visible:z-[2] focus-visible:ring-2 focus-visible:ring-[#a855f7]/45"
                >
                  <span className="relative flex h-[26px] w-[26px] shrink-0 leading-none">
                    <AgentAvatar name={summary.agent.name} presetKey={summary.agent.preset_key} iconKey={summary.agent.icon_key} className="h-[26px] w-[26px] rounded-[9px] border-0 shadow-[0_0_0_2px_#fffefa] dark:shadow-[0_0_0_2px_#242320]" />
                    <span
                      className="absolute -bottom-0.5 -end-0.5 h-2.5 w-2.5 rounded-full border-2 border-[#fffefa] dark:border-[#242320]"
                      style={{ backgroundColor: presentation.dot }}
                      data-agent-dock-trigger-status-dot
                    />
                  </span>
                </button>
              );
            })}
            {runs.length > 4 ? (
              <button type="button" aria-label={`Open agents, ${runs.length - 4} more`} onClick={(event) => onRun(runs[4].run.id, event.currentTarget)} className="agent-dock-stack-more -ms-1.5 grid h-8 min-w-8 place-items-center rounded-[10px] bg-[#f0eee9] px-1 text-[10px] font-semibold text-[#6b6862] shadow-[0_0_0_2px_#fffefa] outline-none hover:z-[1] focus-visible:z-[2] focus-visible:ring-2 focus-visible:ring-[#a855f7]/45 dark:bg-[#37352f] dark:text-[#c4c0b7] dark:shadow-[0_0_0_2px_#242320]">+{runs.length - 4}</button>
            ) : null}
          </span>
          {attentionCount > 0 ? (
            <button type="button" aria-label={`${attentionCount} agent${attentionCount === 1 ? '' : 's'} need your attention`} onClick={(event) => onAttention(event.currentTarget)} className="agent-dock-attention-badge ms-1 flex h-8 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border border-[#f5dcb3] bg-[#fff7ea] pe-[11px] ps-[9px] text-[12px] font-semibold text-[#b45309] outline-none hover:bg-[#fff0d7] focus-visible:ring-2 focus-visible:ring-amber-500/45 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
              <span className="agent-dock-attention-dot h-1.5 w-1.5 rounded-full bg-[#d97706]" />
              <span className="agent-dock-attention-copy">{attentionCount} need you</span>
            </button>
          ) : null}
        </>
      ) : null}
      <button type="button" aria-label={open ? 'Close agent dock' : 'Open agent dock'} aria-expanded={open} aria-controls="agent-dock-panel" onClick={(event) => onToggle(event.currentTarget)} className="agent-dock-toggle grid h-8 w-7 shrink-0 place-items-center rounded-full text-[12px] text-[#a5a29b] outline-none hover:bg-[#f4f2ee] focus-visible:ring-2 focus-visible:ring-[#a855f7]/45 dark:hover:bg-[#302f2b]">
        <span aria-hidden>{open ? '⌄' : '⌃'}</span>
      </button>
    </div>
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
        <AskAgentAvatar plateStyle="feather" className="mx-auto mb-2 h-[72px] w-[72px]" />
        <p className="text-[13px] text-[#8a8781]">No conversations yet.</p>
        <button type="button" onClick={onNewChat} className="mt-3 rounded-[9px] bg-[#1c1b19] px-3 py-2 text-[12.5px] font-semibold text-white hover:bg-[#34322e] dark:bg-[#eeeae1] dark:text-[#1c1b19]">Start a conversation</button>
      </div>
    </div>
  );
}

function SupportChatLoadingPane() {
  return (
    <div className="relative min-h-0 flex-1 px-4 py-5" role="status" aria-label="Opening conversation chat">
      <span className="sr-only">Opening conversation chat…</span>
      <div className="space-y-4">
        <div className="flex items-start gap-3">
          <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
          <div className="w-full space-y-2 pt-1">
            <Skeleton className="h-3 w-2/3 rounded" />
            <Skeleton className="h-3 w-5/6 rounded" />
          </div>
        </div>
        <div className="flex justify-end">
          <Skeleton className="h-12 w-4/5 rounded-xl" />
        </div>
        <div className="flex items-start gap-3">
          <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
          <Skeleton className="h-16 w-3/4 rounded-xl" />
        </div>
      </div>
      <Skeleton className="absolute inset-x-4 bottom-4 h-20 rounded-xl" />
    </div>
  );
}

function SupportChatErrorPane({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="grid min-h-0 flex-1 place-items-center px-6 text-center">
      <div>
        <p className="text-[13px] font-medium text-[#4b4945] dark:text-[#ddd8ce]">Unable to open this conversation chat.</p>
        <p className="mt-1 text-[11.5px] text-[#8a8781]">{message}</p>
        <button type="button" onClick={onRetry} className="mt-3 rounded-[9px] border border-[#dedad1] px-3 py-2 text-[12.5px] font-semibold text-[#4b4945] hover:bg-[#f4f2ee] dark:border-[#45423d] dark:text-[#ddd8ce] dark:hover:bg-[#302f2b]">Retry</button>
      </div>
    </div>
  );
}
