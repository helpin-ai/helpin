type Read<T> = (after: number, signal: AbortSignal) => Promise<T>;
type Waiter<T> = { resolve: (value: T) => void; reject: (error: unknown) => void };
type Batch<T> = {
  after: number;
  read: Read<T>;
  controller: AbortController;
  waiters: Set<Waiter<T>>;
  started: boolean;
  version: number;
  done: boolean;
  timeout?: ReturnType<typeof setTimeout>;
};
type Entry<T> = { active: Batch<T>; queued: Batch<T> | null };

// Entries exist only while reads are in flight. Keys include user, workspace,
// run and endpoint identity; completed responses are never cached here.
const reads = new Map<string, Entry<unknown>>();
let readVersion = 0;

/** Capture this when an action invalidates data, before waiting on another read. */
export function currentAgentRunReadVersion() {
  return readVersion;
}

function batch<T>(after: number, read: Read<T>): Batch<T> {
  return { after, read, controller: new AbortController(), waiters: new Set(), started: false, version: 0, done: false };
}

/** Share identical reads and serialize requests that need an earlier history cursor. */
export function readSharedAgentRun<T>(key: string, after: number, read: Read<T>, signal: AbortSignal, afterRead?: number): Promise<T> {
  if (signal.aborted) return Promise.reject(new DOMException('Read cancelled', 'AbortError'));
  let entry = reads.get(key) as Entry<T> | undefined;
  if (!entry) {
    entry = { active: batch(after, read), queued: null };
    reads.set(key, entry as Entry<unknown>);
  }
  let target = entry.active;
  if (target.started && ((afterRead !== undefined && target.version <= afterRead) || target.after > after)) {
    entry.queued ??= batch(after, read);
    entry.queued.after = Math.min(entry.queued.after, after);
    target = entry.queued;
  }
  const current = entry;
  const finish = (completed: Batch<T>, result: { value: T } | { error: unknown }) => {
    if (completed.done) return;
    completed.done = true;
    clearTimeout(completed.timeout);
    if (current.active === completed) {
      if (current.queued) {
        current.active = current.queued;
        current.queued = null;
        start(current.active);
      } else if (reads.get(key) === current) {
        reads.delete(key);
      }
    } else if (current.queued === completed) {
      current.queued = null;
    }
    for (const waiter of completed.waiters) {
      if ('error' in result) waiter.reject(result.error);
      else waiter.resolve(result.value);
    }
    completed.waiters.clear();
  };
  const start = (next: Batch<T>) => {
    if (next.started || next.done) return;
    next.started = true;
    next.version = ++readVersion;
    // Bound the underlying request lifetime, even when subscribers arrive at
    // different times or retry while another subscriber is still waiting.
    next.timeout = setTimeout(() => {
      next.controller.abort();
      finish(next, { error: new DOMException('Run read timed out', 'TimeoutError') });
    }, 30_000);
    try {
      void next.read(next.after, next.controller.signal).then(
        (value) => finish(next, { value }),
        (error: unknown) => finish(next, { error }),
      );
    } catch (error) {
      finish(next, { error });
    }
  };
  return new Promise<T>((resolve, reject) => {
    const cleanup = () => signal.removeEventListener('abort', abort);
    const waiter: Waiter<T> = {
      resolve: (value) => { cleanup(); resolve(value); },
      reject: (error) => { cleanup(); reject(error); },
    };
    const abort = () => {
      target.waiters.delete(waiter);
      waiter.reject(new DOMException('Read cancelled', 'AbortError'));
      if (target.waiters.size === 0) {
        target.controller.abort();
        finish(target, { error: new DOMException('Read cancelled', 'AbortError') });
      }
    };
    target.waiters.add(waiter);
    signal.addEventListener('abort', abort, { once: true });
    if (current.active === target) start(target);
  });
}
