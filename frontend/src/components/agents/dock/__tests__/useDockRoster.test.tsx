// @vitest-environment jsdom
import { act, StrictMode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useDockRoster } from '../useDockRoster';
import { useDockStore } from '@/stores/dockStore';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import type { DockChat, DockRunSummary } from '@/lib/dockTypes';

const mocks = vi.hoisted(() => ({ listChats: vi.fn(), listRuns: vi.fn() }));
vi.mock('@/lib/services/dockChatService', () => ({ dockChatService: mocks }));
vi.mock('@/lib/helpin', () => ({ resetHelpinIdentity: vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const chat = (id: string): DockChat => ({ id, workspace_id: 'ws-1', user_id: 'user-1', title: id, visibility: 'private', created_at: '', updated_at: '' });
const response = (id: string, next_cursor: string | null = null) => ({ data: { chats: [chat(id)], next_cursor }, error: null });
const runSummary = (id: string, status: string, pauseReason: string, attentionKind?: DockRunSummary['attention_kind']) => ({
  run: { id, status, pause_reason: pauseReason },
  agent: { id: 'agent-1', name: 'Agent' },
  attention_kind: attentionKind,
  last_activity_at: '2026-09-24T00:00:00Z',
} as DockRunSummary);
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }
let root: Root;
let container: HTMLDivElement;
let rosters: Record<string, ReturnType<typeof useDockRoster>>;
function Harness({ id = 'first', userId = 'user-1', open = false }: { id?: string; userId?: string; open?: boolean }) {
  rosters[id] = useDockRoster('ws-1', userId, true, open);
  return null;
}
beforeEach(() => {
  vi.useFakeTimers();
  rosters = {};
  useDockStore.setState({ workspaceId: null, chats: [], transcripts: {}, drafts: {} });
  useAuthStore.setState({ user: { id: 'user-1' } as never });
  useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-1' } as never });
  useSupportPresenceStore.setState({ wsConnected: false });
  mocks.listChats.mockReset().mockResolvedValue(response('first'));
  mocks.listRuns.mockReset().mockResolvedValue({ data: { runs: [] }, error: null });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('shared roster request ordering and recovery', () => {
  it('removes answered chat attention when the active run leaves the roster', async () => {
    const awaiting = runSummary('ask-run', 'paused', 'human_input', 'input');
    mocks.listRuns.mockResolvedValueOnce({ data: { runs: [awaiting] }, error: null })
      .mockResolvedValue({ data: { runs: [] }, error: null });
    await act(async () => root.render(<Harness />));
    expect(rosters.first.runs.map((summary) => summary.run.id)).toEqual(['ask-run']);
    await act(async () => { await rosters.first.refreshRuns(); });
    expect(rosters.first.runs).toEqual([]);
  });

  it('refreshes the collapsed run indicators when a known chat gains attention', async () => {
    mocks.listChats.mockResolvedValue({ data: { chats: [{ ...chat('first'), active_run_id: 'ask-run' }] }, error: null });
    await act(async () => root.render(<Harness />));
    mocks.listRuns.mockClear();
    await act(async () => {
      window.dispatchEvent(new CustomEvent('coding_session_event-created', { detail: { parent_id: 'ask-run', data: { type: 'interaction.created' } } }));
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mocks.listRuns).toHaveBeenCalledTimes(1);
  });

  it('does not reset loaded transcripts or refetch lists when another dock mounts later', async () => {
    await act(async () => root.render(<Harness />));
    useDockStore.getState().setDraft('chat:first', 'Unsent text');
    await act(async () => root.render(<><Harness /><Harness id="second" open /></>));
    expect(mocks.listChats).toHaveBeenCalledTimes(1);
    expect(mocks.listRuns).toHaveBeenCalledTimes(1);
    expect(useDockStore.getState().drafts['chat:first']).toBe('Unsent text');
  });

  it('fetches after a mutation when a pre-mutation list read is still pending', async () => {
    const stale = deferred<ReturnType<typeof response>>();
    mocks.listChats.mockReturnValueOnce(stale.promise).mockResolvedValue(response('after-mutation'));
    await act(async () => root.render(<Harness />));
    let refreshed!: Promise<void>;
    await act(async () => { refreshed = rosters.first.refreshChats(true); });
    expect(mocks.listChats).toHaveBeenCalledTimes(1);
    await act(async () => { stale.resolve(response('stale')); await refreshed; });
    expect(mocks.listChats).toHaveBeenCalledTimes(2);
    expect(useDockStore.getState().chats.map((row) => row.id)).toEqual(['after-mutation']);
  });

  it('deduplicates pagination across docks and retains the advanced cursor after recovery', async () => {
    mocks.listChats.mockResolvedValueOnce(response('first', 'page-2')).mockResolvedValueOnce(response('older', 'page-3')).mockResolvedValue(response('newest', 'page-2'));
    await act(async () => root.render(<><Harness /><Harness id="second" /></>));
    await act(async () => { await Promise.all([rosters.first.loadMoreChats(), rosters.second.loadMoreChats()]); });
    expect(mocks.listChats).toHaveBeenCalledTimes(2);
    expect(rosters.first.nextChatCursor).toBe('page-3');
    await act(async () => { window.dispatchEvent(new Event('focus')); await vi.advanceTimersByTimeAsync(200); });
    expect(rosters.second.nextChatCursor).toBe('page-3');
    expect(useDockStore.getState().chats.map((row) => row.id)).toEqual(['newest', 'first', 'older']);
  });

  it('rejects a previous user response after the authenticated scope changes', async () => {
    const previous = deferred<ReturnType<typeof response>>();
    mocks.listChats.mockReturnValueOnce(previous.promise).mockResolvedValue(response('other-user'));
    await act(async () => root.render(<Harness />));
    await act(async () => {
      useAuthStore.setState({ user: { id: 'user-2' } as never });
      root.render(<Harness userId="user-2" />);
    });
    await act(async () => { previous.resolve(response('private-previous-user')); });
    expect(useDockStore.getState().chats.map((row) => row.id)).toEqual(['other-user']);
  });

  it('coalesces reconnect and focus recovery for both docks and skips hidden reads', async () => {
    let visibility: DocumentVisibilityState = 'visible';
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility);
    await act(async () => root.render(<><Harness /><Harness id="second" /></>));
    mocks.listChats.mockClear(); mocks.listRuns.mockClear();
    await act(async () => {
      visibility = 'hidden';
      document.dispatchEvent(new Event('visibilitychange'));
      useSupportPresenceStore.setState({ wsConnected: true });
      window.dispatchEvent(new Event('focus'));
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mocks.listChats).not.toHaveBeenCalled();
    await act(async () => {
      visibility = 'visible';
      document.dispatchEvent(new Event('visibilitychange'));
      window.dispatchEvent(new Event('focus'));
      window.dispatchEvent(new Event('online'));
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mocks.listChats).toHaveBeenCalledTimes(1);
    expect(mocks.listRuns).toHaveBeenCalledTimes(1);
  });

  it('queues one fresh recovery read after a slow pre-reconnect request', async () => {
    const old = deferred<ReturnType<typeof response>>();
    mocks.listChats.mockReturnValueOnce(old.promise).mockResolvedValue(response('after-reconnect'));
    await act(async () => root.render(<><Harness /><Harness id="second" /></>));
    await act(async () => {
      useSupportPresenceStore.setState({ wsConnected: true });
      window.dispatchEvent(new Event('focus'));
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(mocks.listChats).toHaveBeenCalledTimes(1);
    await act(async () => { old.resolve(response('before-reconnect')); });
    expect(mocks.listChats).toHaveBeenCalledTimes(2);
    expect(useDockStore.getState().chats[0]?.id).toBe('after-reconnect');
  });

  it.each(['canonical-first', 'legacy-first'])('routes new chat content without refreshing runs and ignores repeated states (%s)', async (order) => {
    await act(async () => root.render(<Harness />));
    mocks.listChats.mockClear(); mocks.listRuns.mockClear();
    const emit = (kind: string) => {
      const data = { change_kind: kind === 'content' ? 'message' : 'state', dock_chat_id: 'new-chat', status: 'paused', pause_reason: 'awaiting_user_message' };
      const canonical = new CustomEvent('agent_run-updated', { detail: { entity_id: 'new-run', data, update_kind: kind } });
      const legacy = new CustomEvent('coding_session-updated', { detail: { entity_id: 'new-run', data, update_kind: kind } });
      for (const event of order === 'canonical-first' ? [canonical, legacy] : [legacy, canonical]) window.dispatchEvent(event);
    };
    await act(async () => { emit('content'); vi.advanceTimersByTime(200); });
    expect(mocks.listChats).toHaveBeenCalledTimes(1);
    expect(mocks.listRuns).not.toHaveBeenCalled();
    mocks.listChats.mockClear();
    await act(async () => { emit('duplicate'); vi.advanceTimersByTime(200); });
    expect(mocks.listChats).not.toHaveBeenCalled();
    expect(mocks.listRuns).not.toHaveBeenCalled();
  });

  it.each(['canonical-first', 'legacy-first'])('normalizes raw compatibility aliases before selecting lists (%s)', async (order) => {
    mocks.listChats.mockResolvedValue({ data: { chats: [{ ...chat('first'), active_run_id: 'chat-run' }] }, error: null });
    await act(async () => root.render(<><Harness /><Harness id="second" /></>));
    mocks.listChats.mockClear(); mocks.listRuns.mockClear();
    const dispatch = (id: string) => {
      const canonical = new CustomEvent('agent_run-updated', { detail: { entity_id: id, status: 'running', pause_reason: 'none', update_kind: 'progress' } });
      const legacy = new CustomEvent('coding_session-updated', { detail: { entity_id: id, data: { status: 'running', pause_reason: 'none' } } });
      for (const event of order === 'canonical-first' ? [canonical, legacy] : [legacy, canonical]) window.dispatchEvent(event);
    };
    await act(async () => { dispatch('unrelated'); await vi.advanceTimersByTimeAsync(200); });
    expect(mocks.listRuns).not.toHaveBeenCalled();
    expect(mocks.listChats).not.toHaveBeenCalled();
    await act(async () => { dispatch('chat-run'); await vi.advanceTimersByTimeAsync(200); });
    expect(mocks.listRuns).not.toHaveBeenCalled();
    expect(mocks.listChats).toHaveBeenCalledTimes(1);
  });

  it('times out a hung list and allows a fresh recovery read', async () => {
    mocks.listChats.mockReturnValueOnce(new Promise(() => {})).mockResolvedValue(response('recovered'));
    await act(async () => root.render(<Harness />));
    const signal = mocks.listChats.mock.calls[0]?.[3] as AbortSignal;
    await act(async () => { await vi.advanceTimersByTimeAsync(30_001); });
    expect(rosters.first.chatsLoading).toBe(false);
    expect(rosters.first.chatsError).toMatch(/timed out/i);
    expect(signal.aborted).toBe(true);
    await act(async () => { window.dispatchEvent(new Event('focus')); await vi.advanceTimersByTimeAsync(200); });
    expect(mocks.listChats).toHaveBeenCalledTimes(2);
    expect(useDockStore.getState().chats.map((row) => row.id)).toEqual(['recovered']);
  });

  it('aborts pending reads when the last dock releases its owner', async () => {
    mocks.listChats.mockReturnValueOnce(new Promise(() => {}));
    await act(async () => root.render(<Harness />));
    const signal = mocks.listChats.mock.calls[0]?.[3] as AbortSignal;
    await act(async () => root.render(null));
    expect(signal?.aborted).toBe(true);
  });

  it('publishes slow roster responses while automatic notifications continue', async () => {
    const first = deferred<ReturnType<typeof response>>();
    const second = deferred<ReturnType<typeof response>>();
    mocks.listChats.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    await act(async () => root.render(<Harness />));
    await act(async () => {
      window.dispatchEvent(new CustomEvent('agent_run-updated', { detail: { entity_id: 'new-run', update_kind: 'lifecycle' } }));
      await vi.advanceTimersByTimeAsync(200);
    });
    await act(async () => { first.resolve(response('first-result')); });
    expect(useDockStore.getState().chats[0]?.id).toBe('first-result');
    await act(async () => { second.resolve(response('second-result')); });
  });

  it('restores the first-page cursor after an initial error and automatic recovery', async () => {
    mocks.listChats.mockResolvedValueOnce({ data: null, error: 'Temporary failure' }).mockResolvedValue(response('recovered', 'next-page'));
    await act(async () => root.render(<Harness />));
    expect(rosters.first.chatsError).toBe('Temporary failure');
    await act(async () => { window.dispatchEvent(new Event('focus')); await vi.advanceTimersByTimeAsync(200); });
    expect(rosters.first.nextChatCursor).toBe('next-page');
  });

  it('loads after StrictMode re-subscribes instead of joining an invalidated request', async () => {
    await act(async () => root.render(<StrictMode><Harness /></StrictMode>));
    expect(useDockStore.getState().chats.map((row) => row.id)).toEqual(['first']);
    expect(rosters.first.chatsLoading).toBe(false);
  });
});
