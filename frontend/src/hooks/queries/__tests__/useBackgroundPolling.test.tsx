// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useSupportRoutingUsage, useSupportTeammatePresence, useSupportUnreadByWorkspace } from '../useSupport';
import { useUnreadCount } from '../useNotifications';
import { useWorkspaceMemberPresenceMap } from '../useWorkspaces';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';

const mocks = vi.hoisted(() => ({
  getRoutingUsageStatus: vi.fn(), listTeammatePresence: vi.fn(), listWorkspaceUnread: vi.fn(),
  unreadCount: vi.fn(), listMemberPresence: vi.fn(),
}));
vi.mock('@/lib/services/supportService', () => ({ supportService: mocks }));
vi.mock('@/lib/services/notificationsService', () => ({ notificationsService: mocks }));
vi.mock('@/lib/services/workspacesService', () => ({ workspacesService: mocks }));
vi.mock('@/lib/helpin', () => ({ resetHelpinIdentity: vi.fn() }));
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

let root: Root;
let client: QueryClient;
let container: HTMLDivElement;
let visible: boolean;
let online: boolean;
function Harness({ enabled = true }: { enabled?: boolean }) {
  useSupportRoutingUsage(enabled ? 'ws-1' : '');
  useSupportTeammatePresence('ws-1', enabled);
  useSupportUnreadByWorkspace(enabled);
  useUnreadCount(enabled ? 'ws-1' : '');
  useWorkspaceMemberPresenceMap('ws-1', enabled);
  return null;
}
async function mount(enabled = true) {
  await act(async () => root.render(<QueryClientProvider client={client}><Harness enabled={enabled} /></QueryClientProvider>));
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
}
function expectRequests(count: number) {
  for (const request of Object.values(mocks)) expect(request).toHaveBeenCalledTimes(count);
}
beforeEach(() => {
  vi.useFakeTimers();
  visible = true;
  online = true;
  vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visible ? 'visible' : 'hidden');
  vi.spyOn(navigator, 'onLine', 'get').mockImplementation(() => online);
  useSupportPresenceStore.setState({ wsConnected: true });
  for (const request of Object.values(mocks)) request.mockReset().mockResolvedValue({ data: [], error: null });
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  client.clear();
  container.remove();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('background query recovery polling', () => {
  it('uses a two-minute recovery interval while realtime is connected', async () => {
    await mount();
    expectRequests(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(119_000); });
    expectRequests(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(1_000); });
    expectRequests(2);
  });

  it('uses a thirty-second fallback while realtime is disconnected', async () => {
    useSupportPresenceStore.setState({ wsConnected: false });
    await mount();
    await act(async () => { await vi.advanceTimersByTimeAsync(29_000); });
    expectRequests(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(1_000); });
    expectRequests(2);
  });

  it.each(['hidden', 'offline'])('pauses reads while %s and recovers stale data once available', async (state) => {
    await mount();
    await act(async () => {
      if (state === 'hidden') { visible = false; document.dispatchEvent(new Event('visibilitychange')); }
      else { online = false; window.dispatchEvent(new Event('offline')); }
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(240_000); });
    expectRequests(1);
    await act(async () => {
      visible = true; online = true;
      document.dispatchEvent(new Event('visibilitychange'));
      window.dispatchEvent(new Event('online'));
    });
    expectRequests(2);
  });

  it.each(['hidden', 'offline', 'disabled'])('skips initial reads when %s', async (state) => {
    visible = state !== 'hidden';
    online = state !== 'offline';
    await mount(state !== 'disabled');
    await act(async () => { await vi.advanceTimersByTimeAsync(240_000); });
    expectRequests(0);
  });
});
