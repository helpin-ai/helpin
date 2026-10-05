import { useEffect, useMemo, useRef, useSyncExternalStore } from 'react';
import { toast } from 'sonner';
import type { AgentRunEventDetail } from '@/lib/agentRunRealtime';
import type { DockChat, DockRunSummary } from '@/lib/dockTypes';
import { dockChatService } from '@/lib/services/dockChatService';
import { useAuthStore } from '@/stores/authStore';
import { useDockStore } from '@/stores/dockStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { withDockReadDeadline } from './dockReadDeadline';
import { isDockNetworkAvailable } from './useDockNetworkActivity';

type ListKind = 'chats' | 'runs';
type Activity = { active: boolean; open: boolean };
type RosterState = {
  runs: DockRunSummary[];
  runsLoading: boolean;
  chatsLoading: boolean;
  runsError: string | null;
  chatsError: string | null;
  nextChatCursor: string | null;
  nextRunCursor: string | null;
  loadingMoreChats: boolean;
  loadingMoreRuns: boolean;
};
const emptyState: RosterState = {
  runs: [], runsLoading: true, chatsLoading: true, runsError: null, chatsError: null,
  nextChatCursor: null, nextRunCursor: null, loadingMoreChats: false, loadingMoreRuns: false,
};
const owners = new Map<string, DockRosterOwner>();
let storeScope: string | null = null;

/** One request/listener owner for all dock surfaces in the authenticated workspace. */
class DockRosterOwner {
  private state: RosterState = emptyState;
  private listeners = new Set<() => void>();
  private activities = new Map<symbol, Activity>();
  private requests = new Map<string, Promise<void>>();
  private controllers = new Map<string, AbortController>();
  private archivedChatIds = new Set<string>();
  private pendingRunUpdates = new Map<string, AgentRunEventDetail>();
  private revisions = { chats: 0, runs: 0 };
  private loadedPages = { chats: false, runs: false };
  private started = false;
  private activated = false;
  private disposeEvents?: () => void;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private pending = new Set<ListKind>();
  private followups: Partial<Record<ListKind, { promise: Promise<void>; automatic: boolean; preserveLoaded: boolean }>> = {};

  readonly key: string;
  private workspaceId: string;
  private userId: string;
  private lifetime = 0;
  constructor(key: string, workspaceId: string, userId: string) {
    this.key = key;
    this.workspaceId = workspaceId;
    this.userId = userId;
  }

  getSnapshot = () => this.state;
  private publish(patch: Partial<RosterState>) {
    this.state = { ...this.state, ...patch };
    this.listeners.forEach((listener) => listener());
  }
  private current() {
    return owners.get(this.key) === this
      && useWorkspaceStore.getState().currentWorkspace?.id === this.workspaceId
      && useAuthStore.getState().user?.id === this.userId;
  }
  private canRefresh() {
    return this.current() && isDockNetworkAvailable()
      && [...this.activities.values()].some((activity) => activity.active);
  }

  subscribe = (listener: () => void) => {
    owners.set(this.key, this);
    this.listeners.add(listener);
    if (!this.disposeEvents) this.attachEvents();
    return () => {
      this.listeners.delete(listener);
      if (this.listeners.size) return;
      this.disposeEvents?.();
      this.disposeEvents = undefined;
      if (this.timer) clearTimeout(this.timer);
      this.timer = null;
      this.pending.clear();
      this.pendingRunUpdates.clear();
      this.controllers.forEach((controller) => controller.abort());
      this.controllers.clear();
      this.lifetime += 1;
      this.requests.clear();
      this.followups = {};
      this.started = false;
      this.revisions.chats += 1;
      this.revisions.runs += 1;
      if (owners.get(this.key) === this) owners.delete(this.key);
    };
  };

  setActivity(token: symbol, activity: Activity) {
    const previous = this.activities.get(token);
    this.activities.set(token, activity);
    if (!this.activated && this.current()) {
      this.activated = true;
      // Activating a second dock must not reset global selection/transcripts.
      if (storeScope && storeScope !== this.key) {
        useDockStore.setState({ workspaceId: null });
      }
      storeScope = this.key;
      useDockStore.getState().activateWorkspace(this.workspaceId);
    }
    if (!this.canRefresh()) return;
    if (!this.started) {
      this.started = true;
      void this.read('runs');
      void this.read('chats');
    } else if (previous && ((!previous.active && activity.active) || (!previous.open && activity.open))) {
      this.schedule('runs', 'chats');
    }
  }
  removeActivity(token: symbol) { this.activities.delete(token); }

  private schedule(...kinds: ListKind[]) {
    kinds.forEach((kind) => this.pending.add(kind));
    if (this.timer || !this.canRefresh()) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      if (!this.canRefresh()) return;
      for (const detail of this.pendingRunUpdates.values()) {
        const id = detail.entity_id;
        if (detail.update_kind === 'duplicate') continue;
        if (detail.update_kind === 'progress') {
          if (useDockStore.getState().chats.some((chat) => chat.active_run_id === id)) this.pending.add('chats');
          if (this.state.runs.some((summary) => summary.run.id === id)) this.pending.add('runs');
        } else if (detail.data?.change_kind) {
          const knownChat = useDockStore.getState().chats.some((chat) => chat.active_run_id === id);
          const knownRun = this.state.runs.some((summary) => summary.run.id === id);
          if (knownChat) this.pending.add('chats');
          if (knownRun || (detail.update_kind === 'lifecycle' && (knownChat || detail.dock_chat_id || detail.data.dock_chat_id))) this.pending.add('runs');
          if (!knownChat && !knownRun) this.pending.add(detail.dock_chat_id || detail.data.dock_chat_id ? 'chats' : 'runs');
        } else { this.pending.add('runs'); this.pending.add('chats'); }
      }
      this.pendingRunUpdates.clear();
      const pending = [...this.pending];
      this.pending.clear();
      pending.forEach((kind) => { void this.refreshFresh(kind, true, true); });
    }, 180);
  }

  private attachEvents() {
    const onRun = (event: Event) => {
      if (!this.current()) return;
      const detail = (event as CustomEvent<AgentRunEventDetail>).detail;
      const id = detail?.entity_id;
      const status = detail?.status ?? detail?.data?.status;
      const chats = useDockStore.getState().chats;
      const knownChat = !!id && chats.some((chat) => chat.active_run_id === id);
      if (knownChat && typeof status === 'string') {
        this.updateChatRunStatus(id, status as DockChat['active_run_status']);
      }
      // A raw legacy alias has no update_kind. Retain the canonical kind for
      // the same run/status notification, regardless of which alias arrives first.
      const pause = detail?.pause_reason ?? detail?.data?.pause_reason;
      const approval = detail?.approval_state ?? detail?.data?.approval_state;
      const key = JSON.stringify([id, status, pause === 'none' ? '' : pause ?? '', approval ?? '']);
      const previous = this.pendingRunUpdates.get(key);
      const priority = { duplicate: 1, progress: 2, content: 3, lifecycle: 4 };
      if (!previous?.update_kind || (detail?.update_kind && priority[detail.update_kind] >= priority[previous.update_kind])) {
        this.pendingRunUpdates.set(key, detail ?? {});
      }
      this.schedule();
    };
    const onSession = (event: Event) => {
      const detail = (event as CustomEvent<{ parent_id?: string; data?: { type?: string } }>).detail;
      const type = detail?.data?.type;
      // Deltas and tool progress do not change roster rows. Persisted messages,
      // child results and interaction/lifecycle events may change ordering/title.
      if (!type || !/(?:message\.(?:created|completed)|child|interaction|session\.(?:completed|failed|paused))/.test(type)) return;
      const id = detail?.parent_id;
      const knownChat = useDockStore.getState().chats.some((chat) => chat.active_run_id === id);
      const knownRun = this.state.runs.some((summary) => summary.run.id === id);
      if (knownChat) this.schedule('chats');
      if (knownRun || (knownChat && /interaction|session\./.test(type))) this.schedule('runs');
      if (!knownChat && !knownRun) this.schedule('runs', 'chats');
    };
    const recover = () => {
      if (!this.canRefresh()) return;
      if (!this.started) {
        const entry = this.activities.entries().next().value;
        if (entry) this.setActivity(...entry);
      } else this.schedule('runs', 'chats');
    };
    const unsubscribeConnection = useSupportPresenceStore.subscribe((state, previous) => {
      if (state.wsConnected && !previous.wsConnected) recover();
    });
    window.addEventListener('agent_run-updated', onRun);
    window.addEventListener('coding_session-updated', onRun);
    window.addEventListener('coding_session_event-created', onSession);
    window.addEventListener('focus', recover);
    window.addEventListener('online', recover);
    document.addEventListener('visibilitychange', recover);
    this.disposeEvents = () => {
      unsubscribeConnection();
      window.removeEventListener('agent_run-updated', onRun);
      window.removeEventListener('coding_session-updated', onRun);
      window.removeEventListener('coding_session_event-created', onSession);
      window.removeEventListener('focus', recover);
      window.removeEventListener('online', recover);
      document.removeEventListener('visibilitychange', recover);
    };
  }

  updateChatRunStatus = (runId: string | null, status: DockChat['active_run_status']) => {
    if (!runId || !status || !this.current()) return;
    const chats = useDockStore.getState().chats;
    let changed = false;
    const next = chats.map((chat) => {
      if (chat.active_run_id !== runId || chat.active_run_status === status) return chat;
      changed = true;
      return { ...chat, active_run_status: status };
    });
    if (changed) useDockStore.getState().setChats(next);
  };

  private read(kind: ListKind, cursor?: string, preserveLoaded = true): Promise<void> {
    if (!this.current()) return Promise.resolve();
    const requestKey = `${kind}:${cursor ?? ''}`;
    const existing = this.requests.get(requestKey);
    if (existing) return existing;
    if (!cursor) this.revisions[kind] += 1;
    const revision = this.revisions[kind];
    const controller = new AbortController();
    this.controllers.set(requestKey, controller);
    this.publish(kind === 'chats'
      ? { chatsError: null, ...(cursor ? { loadingMoreChats: true } : {}) }
      : { runsError: null, ...(cursor ? { loadingMoreRuns: true } : {}) });
    const request = Promise.resolve().then(async () => {
      try {
        if (!this.current() || revision !== this.revisions[kind]) return;
        if (kind === 'chats') {
          const result = await withDockReadDeadline((signal) => dockChatService.listChats(this.workspaceId, cursor, 30, signal), controller);
          if (!this.current() || revision !== this.revisions[kind]) return;
          if (result.error || !result.data) {
            this.publish({ chatsError: result.error ?? 'Unable to load conversations' });
            if (cursor) toast.error(result.error ?? 'Unable to load more conversations');
            return;
          }
          const current = useDockStore.getState().chats;
          const byId = new Map(current.map((chat) => [chat.id, chat]));
          const incoming = result.data.chats.filter((chat) => !this.archivedChatIds.has(chat.id)).map((chat) => {
            const previous = byId.get(chat.id);
            return !chat.active_run_status && previous && previous.active_run_id === chat.active_run_id && previous.active_run_status
              ? { ...chat, active_run_status: previous.active_run_status } : chat;
          });
          const incomingIds = new Set(incoming.map((chat) => chat.id));
          const merged = cursor
            ? [...current, ...incoming.filter((chat) => !byId.has(chat.id))]
            : [...incoming, ...(preserveLoaded ? current.filter((chat) => !incomingIds.has(chat.id)) : [])];
          useDockStore.getState().setChats(merged);
          // Refreshing page one must not rewind an already advanced cursor.
          if (cursor || !preserveLoaded || !this.loadedPages.chats) this.publish({ nextChatCursor: result.data.next_cursor ?? null });
          if (cursor) this.loadedPages.chats = true;
          else if (!preserveLoaded) this.loadedPages.chats = false;
        } else {
          const result = await withDockReadDeadline((signal) => dockChatService.listRuns(this.workspaceId, cursor, 30, signal), controller);
          if (!this.current() || revision !== this.revisions[kind]) return;
          if (result.error || !result.data) {
            this.publish({ runsError: result.error ?? 'Unable to load agent runs' });
            if (cursor) toast.error(result.error ?? 'Unable to load more agent runs');
            return;
          }
          const current = this.state.runs;
          const incoming = (result.data.runs ?? []).filter((summary) => !this.archivedChatIds.has(summary.run.dock_chat_id ?? ''));
          const known = new Set(current.map((summary) => summary.run.id));
          const incomingIds = new Set(incoming.map((summary) => summary.run.id));
          this.publish({
            runs: cursor ? [...current, ...incoming.filter((summary) => !known.has(summary.run.id))]
              : [...incoming, ...(preserveLoaded ? current.filter((summary) =>
                !incomingIds.has(summary.run.id)
                && !['queued', 'running', 'paused'].includes(summary.run.status)
              ) : [])],
            ...(cursor || !preserveLoaded || !this.loadedPages.runs ? { nextRunCursor: result.data.next_cursor ?? null } : {}),
          });
          if (cursor) this.loadedPages.runs = true;
          else if (!preserveLoaded) this.loadedPages.runs = false;
        }
      } catch (error) {
        if (this.current() && revision === this.revisions[kind]) {
          const message = error instanceof Error ? error.message : 'Unable to refresh dock';
          this.publish(kind === 'chats' ? { chatsError: message } : { runsError: message });
        }
      } finally {
        if (this.requests.get(requestKey) === request) this.requests.delete(requestKey);
        if (this.controllers.get(requestKey) === controller) this.controllers.delete(requestKey);
        if (this.current() && revision === this.revisions[kind]) this.publish(kind === 'chats'
          ? { chatsLoading: false, loadingMoreChats: false }
          : { runsLoading: false, loadingMoreRuns: false });
      }
    });
    this.requests.set(requestKey, request);
    return request;
  }

  // Explicit mutation refreshes must fetch after the mutation, even when an
  // older automatic read is still in flight. Multiple such triggers coalesce.
  private refreshFresh(kind: ListKind, preserveLoaded = true, automatic = false): Promise<void> {
    if (!automatic) this.revisions[kind] += 1;
    const queued = this.followups[kind];
    if (queued) {
      queued.automatic &&= automatic;
      queued.preserveLoaded &&= preserveLoaded;
      return queued.promise;
    }
    const inFlight = [...this.requests.entries()].filter(([key]) => key.startsWith(`${kind}:`)).map(([, request]) => request);
    if (!inFlight.length) return this.read(kind, undefined, preserveLoaded);
    const lifetime = this.lifetime;
    const batch = { automatic, preserveLoaded, promise: Promise.resolve() };
    const next = Promise.all(inFlight).then(() => {
      if (lifetime !== this.lifetime) return;
      delete this.followups[kind];
      if (batch.automatic && !this.canRefresh()) { this.pending.add(kind); return; }
      return this.read(kind, undefined, batch.preserveLoaded);
    });
    batch.promise = next;
    this.followups[kind] = batch;
    return next;
  }
  archiveChat = async (chatId: string): Promise<boolean> => {
    if (!this.current() || this.archivedChatIds.has(chatId)) return false;
    const chats = useDockStore.getState().chats;
    const index = chats.findIndex((chat) => chat.id === chatId);
    const archivedChat = chats[index];
    const archivedRuns = this.state.runs.filter((summary) => summary.run.dock_chat_id === chatId);
    // Keep a tombstone for this roster's lifetime so stale reads cannot restore
    // a chat or its collapsed-bar links, including across multiple dock surfaces.
    this.archivedChatIds.add(chatId);
    useDockStore.getState().setChats(chats.filter((chat) => chat.id !== chatId));
    this.publish({ runs: this.state.runs.filter((summary) => summary.run.dock_chat_id !== chatId) });
    try {
      const result = await dockChatService.updateChat(this.workspaceId, chatId, { archived: true });
      if (result.error || !result.data) throw new Error(result.error ?? 'Failed to archive conversation');
      return true;
    } catch (error) {
      this.archivedChatIds.delete(chatId);
      if (this.current()) {
        const currentChats = [...useDockStore.getState().chats];
        if (archivedChat && !currentChats.some((chat) => chat.id === chatId)) {
          currentChats.splice(Math.min(index, currentChats.length), 0, archivedChat);
          useDockStore.getState().setChats(currentChats);
        }
        const currentRunIds = new Set(this.state.runs.map((summary) => summary.run.id));
        this.publish({ runs: [...this.state.runs, ...archivedRuns.filter((summary) => !currentRunIds.has(summary.run.id))] });
        toast.error(error instanceof Error ? error.message : 'Failed to archive conversation');
      }
      return false;
    }
  };

  invalidateChats = () => { this.revisions.chats += 1; };
  refreshChats = (preserveLoaded = false) => this.refreshFresh('chats', preserveLoaded);
  refreshRuns = async () => {
    await this.refreshFresh('runs');
    return this.state.runs;
  };
  loadMoreChats = () => this.state.nextChatCursor ? this.read('chats', this.state.nextChatCursor) : Promise.resolve();
  loadMoreRuns = () => this.state.nextRunCursor ? this.read('runs', this.state.nextRunCursor) : Promise.resolve();
}

const subscribeEmpty = () => () => {};
const getEmptySnapshot = () => emptyState;

export function useDockRoster(workspaceId: string | undefined, userId: string | undefined, active: boolean, open: boolean) {
  const token = useRef(Symbol('dock-roster-consumer'));
  const owner = useMemo(() => {
    if (!workspaceId || !userId) return null;
    const key = JSON.stringify([workspaceId, userId]);
    let entry = owners.get(key);
    if (!entry) { entry = new DockRosterOwner(key, workspaceId, userId); owners.set(key, entry); }
    return entry;
  }, [workspaceId, userId]);
  const state = useSyncExternalStore(owner?.subscribe ?? subscribeEmpty, owner?.getSnapshot ?? getEmptySnapshot);
  useEffect(() => {
    owner?.setActivity(token.current, { active, open });
  }, [owner, active, open]);
  useEffect(() => {
    const id = token.current;
    return () => owner?.removeActivity(id);
  }, [owner]);
  return {
    ...state,
    archiveChat: owner?.archiveChat ?? (async () => false),
    invalidateChats: owner?.invalidateChats ?? (() => {}),
    refreshChats: owner?.refreshChats ?? (async () => {}),
    refreshRuns: owner?.refreshRuns ?? (async () => []),
    loadMoreChats: owner?.loadMoreChats ?? (async () => {}),
    loadMoreRuns: owner?.loadMoreRuns ?? (async () => {}),
    updateChatRunStatus: owner?.updateChatRunStatus ?? (() => {}),
  };
}
