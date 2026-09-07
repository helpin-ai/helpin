import { describe, expect, it, vi } from 'vitest';
import { currentAgentRunReadVersion, readSharedAgentRun } from '../sharedAgentRunRead';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

describe('shared agent run reads', () => {
  it('shares matching reads across consumers without caching completed responses', async () => {
    const pending = deferred<string>();
    const load = vi.fn(() => pending.promise);
    const first = readSharedAgentRun('same-run', 0, load, new AbortController().signal);
    const second = readSharedAgentRun('same-run', 0, load, new AbortController().signal);
    expect(load).toHaveBeenCalledTimes(1);
    pending.resolve('snapshot');
    expect(await Promise.all([first, second])).toEqual(['snapshot', 'snapshot']);
    await readSharedAgentRun('same-run', 0, load, new AbortController().signal);
    expect(load).toHaveBeenCalledTimes(2);
  });

  it('queues an explicit post-action read behind another consumer pre-action read', async () => {
    const pending = deferred<string>();
    const load = vi.fn().mockReturnValueOnce(pending.promise).mockResolvedValue('running');
    const first = readSharedAgentRun('post-action', 0, load, new AbortController().signal);
    const boundary = currentAgentRunReadVersion();
    const second = readSharedAgentRun('post-action', 0, load, new AbortController().signal, boundary);
    const third = readSharedAgentRun('post-action', 0, load, new AbortController().signal, boundary);
    expect(load).toHaveBeenCalledTimes(1);
    pending.resolve('paused');
    expect(await Promise.all([first, second, third])).toEqual(['paused', 'running', 'running']);
    expect(load).toHaveBeenCalledTimes(2);
  });

  it('queues a lower history cursor without overlapping the current read', async () => {
    const pending = deferred<string>();
    const load = vi.fn((after: number) => after === 20 ? pending.promise : Promise.resolve('full history'));
    const first = readSharedAgentRun('older-history', 20, load, new AbortController().signal);
    const second = readSharedAgentRun('older-history', 10, load, new AbortController().signal);
    const third = readSharedAgentRun('older-history', 0, load, new AbortController().signal);
    expect(load).toHaveBeenCalledTimes(1);
    pending.resolve('tail');
    expect(await Promise.all([first, second, third])).toEqual(['tail', 'full history', 'full history']);
    expect(load.mock.calls.map(([after]) => after)).toEqual([20, 0]);
  });

  it('does not abort a shared request when only one consumer leaves', async () => {
    const pending = deferred<string>();
    const load = vi.fn((_after: number, _signal: AbortSignal) => pending.promise);
    const controller = new AbortController();
    const first = readSharedAgentRun('shared-abort', 0, load, controller.signal).catch((error: Error) => error.name);
    const second = readSharedAgentRun('shared-abort', 0, load, new AbortController().signal);
    controller.abort();
    expect(await first).toBe('AbortError');
    expect(load.mock.calls[0][1].aborted).toBe(false);
    pending.resolve('still subscribed');
    expect(await second).toBe('still subscribed');
  });

  it('aborts abandoned work and allows another read even if its fetcher never settles', async () => {
    const load = vi.fn((_after: number, _signal: AbortSignal) => new Promise<string>(() => {}));
    const controller = new AbortController();
    const first = readSharedAgentRun('abandoned', 0, load, controller.signal).catch((error: Error) => error.name);
    controller.abort();
    expect(await first).toBe('AbortError');
    expect(load.mock.calls[0][1].aborted).toBe(true);
    expect(await readSharedAgentRun('abandoned', 0, async () => 'recovered', new AbortController().signal)).toBe('recovered');
  });

  it('retires a hung shared batch even while staggered consumers remain subscribed', async () => {
    vi.useFakeTimers();
    try {
      const load = vi.fn((_after: number, _signal: AbortSignal) => new Promise<string>(() => {}));
      const first = readSharedAgentRun('deadline', 0, load, new AbortController().signal).catch((error: Error) => error.name);
      await vi.advanceTimersByTimeAsync(5000);
      const second = readSharedAgentRun('deadline', 0, load, new AbortController().signal).catch((error: Error) => error.name);
      await vi.advanceTimersByTimeAsync(25000);
      expect(load.mock.calls[0][1].aborted).toBe(true);
      expect(await Promise.all([first, second])).toEqual(['TimeoutError', 'TimeoutError']);
      expect(await readSharedAgentRun('deadline', 0, async () => 'fresh', new AbortController().signal)).toBe('fresh');
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps different user, workspace, run and endpoint scopes separate', async () => {
    const load = vi.fn(async () => 'snapshot');
    await Promise.all(['user1:ws1:run1:chat1', 'user2:ws1:run1:chat1', 'user1:ws2:run1:chat1', 'user1:ws1:run2:chat1', 'user1:ws1:run1:chat2'].map((key) => readSharedAgentRun(key, 0, load, new AbortController().signal)));
    expect(load).toHaveBeenCalledTimes(5);
  });
});
