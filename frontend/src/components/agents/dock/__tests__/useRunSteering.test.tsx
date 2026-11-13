// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useRunSteering } from '../useRunSteering';
import { dockChatService } from '@/lib/services/dockChatService';
import { codingSessionService } from '@/lib/services/codingSessionService';

const run = { id: 'run-1', status: 'running', runtime_kind: 'native_sdk' };
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const roots: Root[] = [];
function renderHook<P = undefined>(hook: (props: P) => ReturnType<typeof useRunSteering>, options?: { initialProps: P }) {
  const root = createRoot(document.createElement('div'));
  roots.push(root);
  const result = { current: null as unknown as ReturnType<typeof useRunSteering> };
  function Harness({ props }: { props: P }) { result.current = hook(props); return null; }
  const rerender = (props: P) => act(() => root.render(<Harness props={props} />));
  rerender(options?.initialProps as P);
  return { result, rerender, unmount: () => { act(() => root.unmount()); roots.splice(roots.indexOf(root), 1); } };
}
async function waitFor(check: () => void) { await act(async () => {}); check(); }
afterEach(() => { act(() => roots.splice(0).forEach(root => root.unmount())); vi.restoreAllMocks(); vi.useRealTimers(); });

describe('run steering', () => {
  it('rechecks capability when the runtime has not attached yet', async () => {
    vi.useFakeTimers();
    const read = vi.spyOn(dockChatService, 'runSteering').mockResolvedValueOnce({ data: null, error: 'Not ready', status: 400 }).mockResolvedValue({ data: [], error: null });
    const { result, unmount } = renderHook(() => useRunSteering('ws', run, true));
    await act(async () => {});
    expect(result.current.canSteer).toBe(false);
    await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
    expect(result.current.canSteer).toBe(true);
    expect(read).toHaveBeenCalledTimes(2);
    unmount();
  });
  it('keeps the command ID across an ambiguous failure on the same chat run', async () => {
    vi.spyOn(dockChatService, 'chatRunSteering').mockResolvedValue({ data: [], error: null });
    const send = vi.spyOn(dockChatService, 'steerChatRun').mockResolvedValueOnce({ data: null, error: 'Connection lost', status: 502 }).mockImplementation(async (_ws, _chat, _run, command) => ({ data: { ...command, revision: 1, state: 'accepted' }, error: null }));
    const { result } = renderHook(() => useRunSteering('ws', run, true, 'dock', 'chat'));
    await waitFor(() => expect(result.current.canSteer).toBe(true));
    await act(async () => { await expect(result.current.send('Research only')).rejects.toThrow('Connection lost'); });
    await act(async () => { await result.current.send('Research only'); });
    expect(send.mock.calls[0]).toEqual(send.mock.calls[1]);
    expect(send.mock.calls[1].slice(0, 3)).toEqual(['ws', 'chat', 'run-1']);
    expect(result.current.receipt?.state).toBe('accepted');
  });
  it('uses the sheet endpoint and stops enabling steering after pause', async () => {
    vi.spyOn(codingSessionService, 'runSteering').mockResolvedValue({ data: [], error: null });
    const send = vi.spyOn(codingSessionService, 'steerRun').mockImplementation(async (_ws, _run, command) => ({ data: { ...command, revision: 1, state: 'applied' }, error: null }));
    const { result, rerender } = renderHook(({ status }) => useRunSteering('ws', { ...run, status }, true, 'pm'), { initialProps: { status: 'running' } });
    await waitFor(() => expect(result.current.canSteer).toBe(true));
    await act(async () => { await result.current.send('Use three examples'); });
    expect(send).toHaveBeenCalledWith('ws', 'run-1', expect.objectContaining({ content: 'Use three examples' }));
    await act(async () => { rerender({ status: 'paused' }); });
    expect(result.current.canSteer).toBe(false);
    await expect(result.current.send('More')).rejects.toThrow('no longer');
  });
  it('does not carry capability or receipts to a different run', async () => {
    vi.spyOn(dockChatService, 'runSteering').mockImplementation(async (_ws, id) => id === 'run-1' ? { data: [{ command_id: 'old', revision: 1, state: 'applied', content: 'Old' }], error: null } : { data: null, error: 'Unsupported' });
    const { result, rerender } = renderHook(({ id }) => useRunSteering('ws', { ...run, id }, true), { initialProps: { id: 'run-1' } });
    await waitFor(() => expect(result.current.canSteer).toBe(true));
    await act(async () => { rerender({ id: 'run-2' }); });
    expect(result.current.canSteer).toBe(false);
    expect(result.current.receipt).toBeNull();
  });
});

it('refreshes the revision after a conflict and inspects the applied receipt', async () => {
  vi.useFakeTimers();
  const history = vi.spyOn(dockChatService, 'runSteering').mockResolvedValue({ data: [], error: null });
  const send = vi.spyOn(dockChatService, 'steerRun').mockResolvedValueOnce({ data: null, error: 'Conflict', status: 409 }).mockImplementation(async (_ws, _run, command) => ({ data: { ...command, revision: 3, state: 'accepted' }, error: null }));
  const { result } = renderHook(() => useRunSteering('ws', run, true));
  await act(async () => {});
  await act(async () => { await expect(result.current.send('Use a table')).rejects.toThrow('Conflict'); });
  history.mockResolvedValue({ data: [{ command_id: 'other', revision: 2, content: 'Other', state: 'applied' }], error: null });
  await act(async () => { await result.current.send('Use a table'); });
  expect(send.mock.calls[1][2].expected_revision).toBe(2);
  expect(send.mock.calls[1][2].command_id).not.toBe(send.mock.calls[0][2].command_id);
  history.mockResolvedValue({ data: [{ ...send.mock.calls[1][2], revision: 3, state: 'applied' }], error: null });
  await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
  expect(result.current.receipt?.state).toBe('applied');
});

it('does not enable inactive or unsupported runs', async () => {
  const read = vi.spyOn(dockChatService, 'runSteering');
  const { result, rerender } = renderHook(({ active, runtime_kind }) => useRunSteering('ws', { ...run, runtime_kind }, active), { initialProps: { active: false, runtime_kind: 'native_sdk' } });
  await act(async () => {});
  expect(result.current.canSteer).toBe(false);
  await act(async () => { rerender({ active: true, runtime_kind: 'codex' }); });
  expect(result.current.canSteer).toBe(false);
  expect(read).not.toHaveBeenCalled();
});
