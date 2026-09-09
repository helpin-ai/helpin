// @vitest-environment jsdom
import { act, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useObjectiveAutosave } from '../useObjectiveAutosave';
import type { UpdateObjectiveRequest } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let queue: ReturnType<typeof useObjectiveAutosave>;
let uploads = 0;
let save: ReturnType<typeof vi.fn<(patch: UpdateObjectiveRequest) => Promise<void>>>;
function Harness({ save, uploads }: { save: (patch: UpdateObjectiveRequest) => Promise<void>; uploads: number }) { const value = useObjectiveAutosave({ save, pendingUploads: uploads }); useLayoutEffect(() => { queue = value; }); return null; }
beforeEach(() => {
  vi.useFakeTimers(); uploads = 0; save = vi.fn().mockResolvedValue(undefined);
  root = createRoot(document.createElement('div'));
  act(() => root.render(<Harness save={save} uploads={uploads} />));
});
afterEach(() => { act(() => root.unmount()); vi.useRealTimers(); });

describe('objective autosave', () => {
  it('combines edits and saves after the debounce', async () => {
    act(() => { queue.queuePatch({ name: 'First' }); queue.queuePatch({ name: 'Latest', health: 'at_risk' }); });
    expect(queue.dirty).toBe(true);
    await act(async () => { await vi.advanceTimersByTimeAsync(649); });
    expect(save).not.toHaveBeenCalled();
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenCalledWith({ name: 'Latest', health: 'at_risk' });
    expect(queue.dirty).toBe(false);
  });

  it('flushes immediately and drains newer edits without concurrent requests', async () => {
    let complete!: () => void;
    save.mockImplementationOnce(() => new Promise<void>(resolve => { complete = resolve; }));
    act(() => queue.queuePatch({ name: 'First' }));
    let flushing!: Promise<void>;
    act(() => { flushing = queue.flush(); });
    act(() => queue.queuePatch({ name: 'Latest' }));
    let navigation!: Promise<void>;
    act(() => { navigation = queue.flush(); });
    expect(save).toHaveBeenCalledTimes(1);
    await act(async () => { complete(); await Promise.all([flushing, navigation]); });
    expect(save.mock.calls).toEqual([[{ name: 'First' }], [{ name: 'Latest' }]]);
    expect(queue.dirty).toBe(false);
  });

  it('keeps the latest edits after failure and requires an explicit retry', async () => {
    let fail!: (error: Error) => void;
    save.mockImplementationOnce(() => new Promise<void>((_, reject) => { fail = reject; }));
    act(() => queue.queuePatch({ name: 'First', health: 'on_track' }));
    let flushing!: Promise<void>;
    act(() => { flushing = queue.flush(); });
    act(() => queue.queuePatch({ name: 'Latest' }));
    await act(async () => { fail(new Error('Offline')); await expect(flushing).rejects.toThrow('Offline'); });
    expect(queue.error).toBe('Offline');
    expect(queue.dirty).toBe(true);
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(save).toHaveBeenCalledTimes(1);
    await act(async () => { await queue.flush(); });
    expect(save).toHaveBeenLastCalledWith({ name: 'Latest', health: 'on_track' });
    expect(queue.error).toBeNull();
  });

  it('defers autosave during uploads and resumes when they finish', async () => {
    uploads = 1;
    act(() => { root.render(<Harness save={save} uploads={uploads} />); queue.queuePatch({ description: '<p>Uploading</p>' }); });
    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(save).not.toHaveBeenCalled();
    uploads = 0;
    act(() => root.render(<Harness save={save} uploads={uploads} />));
    await act(async () => { await vi.advanceTimersByTimeAsync(650); });
    expect(save).toHaveBeenCalledTimes(1);
  });

  it('blocks navigation during uploads even before a description patch arrives', async () => {
    uploads = 1;
    act(() => root.render(<Harness save={save} uploads={uploads} />));
    await act(async () => { await expect(queue.flush()).rejects.toThrow('Wait for uploads'); });
    expect(queue.dirty).toBe(true);
    expect(save).not.toHaveBeenCalled();
  });
});
