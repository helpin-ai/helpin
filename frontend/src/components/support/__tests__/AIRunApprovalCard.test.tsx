// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AIRunApprovalCard } from '../AIRunApprovalCard';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';

const mocks = vi.hoisted(() => ({ listAIRunInteractions: vi.fn(), resolveAIRunInteraction: vi.fn() }));
vi.mock('@/lib/services/supportService', () => ({ supportService: mocks }));
vi.mock('@/lib/icons', () => ({ SecurityCheckIcon: () => null }));
vi.mock('@/components/agents/dock/PendingInteractionCard', () => ({
  PendingInteractionCard: ({ interaction }: { interaction: { interaction_id: string } }) => <span>{interaction.interaction_id}</span>,
}));
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

const key = ['support', 'ws-1', 'conversations', 'conv-1', 'ai-run-interactions'];
function response(runId = 'run-1', pending = false) {
  return { data: { run_id: runId, interactions: pending ? [{ id: 'approval-1', status: 'pending' }] : [] }, error: null, status: 200 };
}
function event(name: string, detail: Record<string, unknown>) {
  window.dispatchEvent(new CustomEvent(name, { detail: { workspace_id: 'ws-1', ...detail } }));
}
function interaction(runId = 'run-1', type = 'interaction.requested') {
  event('coding_session_event-created', { parent_id: runId, data: { type } });
}
function runEvent(data: Record<string, unknown> = {}, action = 'updated') {
  event(`agent_run-${action}`, { entity_id: 'run-1', parent_type: 'support_conversation', parent_id: 'conv-1', data });
}
let root: Root;
let container: HTMLDivElement;
let client: QueryClient;
let visible: boolean;
let online: boolean;
async function mount(enabled = true) {
  await act(async () => root.render(<QueryClientProvider client={client}><AIRunApprovalCard workspaceId="ws-1" conversationId="conv-1" enabled={enabled} /></QueryClientProvider>));
  await tick(1);
}
async function tick(ms = 250) {
  await act(async () => { await vi.advanceTimersByTimeAsync(ms); });
}
beforeEach(() => {
  vi.useFakeTimers();
  visible = true;
  online = true;
  vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visible ? 'visible' : 'hidden');
  vi.spyOn(navigator, 'onLine', 'get').mockImplementation(() => online);
  useSupportPresenceStore.setState({ wsConnected: true });
  mocks.listAIRunInteractions.mockReset().mockResolvedValue(response());
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

describe('AI run approval recovery', () => {
  it('does not keep polling a conversation without an active run', async () => {
    mocks.listAIRunInteractions.mockResolvedValue({ data: null, error: 'no run', status: 404 });
    await mount();
    await tick(240_000);
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(1);
  });

  it('uses a two-minute connected fallback for an active run', async () => {
    await mount();
    await tick(119_000);
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(1);
    await tick(1_000);
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
  });

  it('discovers a new conversation run after the initial empty result', async () => {
    mocks.listAIRunInteractions.mockResolvedValueOnce(response('')).mockResolvedValue(response('run-1', true));
    await mount();
    act(() => runEvent({ status: 'running', change_kind: 'state' }, 'created'));
    await tick();
    expect(container.textContent).toContain('approval-1');
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
  });

  it('keeps recovery for a known active run while its conversation link catches up', async () => {
    mocks.listAIRunInteractions.mockResolvedValue({ data: null, error: 'no run', status: 404 });
    await mount();
    act(() => runEvent({ status: 'running', change_kind: 'state' }, 'created'));
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
    mocks.listAIRunInteractions.mockResolvedValue(response('run-1', true));
    await tick(120_000);
    expect(container.textContent).toContain('approval-1');
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(3);
  });

  it('coalesces interaction events and removes approvals resolved without a run status change', async () => {
    await mount();
    mocks.listAIRunInteractions.mockResolvedValue(response('run-1', true));
    act(() => { interaction(); interaction(); runEvent({ status: 'running', change_kind: 'interaction' }); });
    await tick();
    expect(container.textContent).toContain('approval-1');
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
    mocks.listAIRunInteractions.mockResolvedValue(response());
    act(() => interaction('run-1', 'interaction.resolved'));
    await tick();
    expect(container.textContent).not.toContain('approval-1');
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(3);
  });

  it('ignores unrelated runs, workspaces, and message or artifact progress', async () => {
    await mount();
    act(() => {
      interaction('other-run');
      event('agent_run-created', { entity_id: 'other', parent_type: 'support_conversation', parent_id: 'other-conv' });
      event('coding_session_event-created', { workspace_id: 'other', parent_id: 'run-1', data: { type: 'interaction.requested' } });
      interaction('run-1', 'assistant.delta');
      runEvent({ status: 'running', change_kind: 'message' });
      runEvent({ status: 'running', change_kind: 'artifact' });
      event('agent_run-updated', { entity_id: 'run-1', update_kind: 'progress' });
    });
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(1);
  });

  it('fetches fresh approvals after an event arrives during an older request', async () => {
    let resolve!: (value: ReturnType<typeof response>) => void;
    mocks.listAIRunInteractions.mockReturnValueOnce(new Promise((done) => { resolve = done; })).mockResolvedValue(response('run-1', true));
    await mount();
    act(() => interaction());
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(1);
    await act(async () => resolve(response()));
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
    expect(container.textContent).toContain('approval-1');
  });

  it('stops fallback polling after a matching terminal event', async () => {
    await mount();
    act(() => runEvent({ status: 'completed', change_kind: 'state' }));
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
    await tick(240_000);
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
  });

  it('accepts interactions for a newer run discovered after an older terminal event', async () => {
    await mount();
    act(() => runEvent({ status: 'completed', change_kind: 'state' }));
    await tick();
    mocks.listAIRunInteractions.mockResolvedValue(response('run-2'));
    act(() => { visible = false; document.dispatchEvent(new Event('visibilitychange')); });
    act(() => { visible = true; document.dispatchEvent(new Event('visibilitychange')); });
    await tick();
    mocks.listAIRunInteractions.mockResolvedValue(response('run-2', true));
    act(() => interaction('run-2'));
    await tick();
    expect(container.textContent).toContain('approval-1');
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(4);
  });

  it('queues one followup when another interaction arrives during an event refresh', async () => {
    await mount();
    let resolve!: (value: ReturnType<typeof response>) => void;
    mocks.listAIRunInteractions.mockReturnValueOnce(new Promise((done) => { resolve = done; })).mockResolvedValue(response('run-1', true));
    act(() => interaction());
    await tick();
    act(() => { interaction(); interaction('run-1', 'interaction.updated'); });
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
    await act(async () => resolve(response()));
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(3);
    expect(container.textContent).toContain('approval-1');
  });

  it('retains pending approvals when recovery encounters a transient error', async () => {
    mocks.listAIRunInteractions.mockResolvedValue(response('run-1', true));
    await mount();
    mocks.listAIRunInteractions.mockResolvedValue({ data: null, error: 'temporarily unavailable', status: 503 });
    await act(async () => { await client.invalidateQueries({ queryKey: key }); });
    await tick();
    expect(container.textContent).toContain('approval-1');
  });

  it.each(['hidden', 'offline', 'disabled'])('does not read approvals while %s', async (state) => {
    visible = state !== 'hidden';
    online = state !== 'offline';
    await mount(state !== 'disabled');
    act(() => { interaction(); runEvent({ change_kind: 'interaction' }); });
    await tick(240_000);
    expect(mocks.listAIRunInteractions).not.toHaveBeenCalled();
  });

  it('rediscovers a run when returning from a hidden tab', async () => {
    mocks.listAIRunInteractions.mockResolvedValueOnce(response('')).mockResolvedValue(response('run-1', true));
    await mount();
    act(() => { visible = false; document.dispatchEvent(new Event('visibilitychange')); });
    act(() => { visible = true; document.dispatchEvent(new Event('visibilitychange')); });
    await tick();
    expect(mocks.listAIRunInteractions).toHaveBeenCalledTimes(2);
    expect(container.textContent).toContain('approval-1');
  });
});
