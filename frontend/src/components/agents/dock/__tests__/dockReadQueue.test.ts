// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createDockReadQueue } from '../dockReadQueue';
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }
afterEach(() => vi.useRealTimers());

describe('dock read batches', () => {
  it('rejects obsolete data and settles an explicit caller without waiting for a later background batch', async () => {
    const first = deferred<string>(); const second = deferred<string>(); const third = deferred<string>();
    const fetch = vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise).mockReturnValueOnce(third.promise);
    const queue = createDockReadQueue(fetch, () => true, vi.fn());
    const old = queue.read(true);
    const explicit = queue.read(false);
    first.resolve('old');
    expect(await old).toBeUndefined();
    const background = queue.read(true);
    second.resolve('newer');
    expect(await explicit).toBe('newer');
    expect(fetch).toHaveBeenCalledTimes(3);
    third.resolve('latest');
    expect(await background).toBe('latest');
    queue.dispose();
  });

  it('promotes a queued background read to an explicit action that still runs while hidden', async () => {
    let visible = true;
    const first = deferred<string>();
    const fetch = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue('after-action');
    const queue = createDockReadQueue(fetch, () => visible, vi.fn());
    const old = queue.read(true);
    const automatic = queue.read(true);
    visible = false;
    const explicit = queue.read(false);
    first.resolve('old');
    expect(await old).toBeUndefined();
    expect(await explicit).toBe('after-action');
    expect(await automatic).toBe('after-action');
    queue.dispose();
  });

  it('publishes successful reads during continuous automatic notifications', async () => {
    const first = deferred<string>(); const second = deferred<string>();
    const fetch = vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise).mockResolvedValue('third');
    const queue = createDockReadQueue(fetch, () => true, vi.fn());
    const initial = queue.read(true);
    const followup = queue.read(true);
    first.resolve('first');
    expect(await initial).toBe('first');
    const later = queue.read(true);
    second.resolve('second');
    expect(await followup).toBe('second');
    expect(await later).toBe('third');
    queue.dispose();
  });

  it('does not publish a late response after disposal', async () => {
    const pending = deferred<string>();
    const queue = createDockReadQueue(() => pending.promise, () => true, vi.fn());
    const read = queue.read();
    queue.dispose();
    pending.resolve('late');
    expect(await read).toBeUndefined();
  });
});
