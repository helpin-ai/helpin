import { withDockReadDeadline } from './dockReadDeadline';

type Batch<T> = {
  automatic: boolean;
  controller: AbortController;
  epoch: number;
  resolve: (value: T | undefined) => void;
  promise: Promise<T | undefined>;
};

/** One active read and one coalesced follow-up; callers await only their batch. */
export function createDockReadQueue<T>(
  fetch: (signal: AbortSignal) => Promise<T>,
  canRefresh: () => boolean,
  onError: (message: string) => void,
) {
  let active: Batch<T> | null = null;
  let queued: Batch<T> | null = null;
  let epoch = 0;
  let disposed = false;
  const batch = (automatic: boolean): Batch<T> => {
    let resolve!: Batch<T>['resolve'];
    const promise = new Promise<T | undefined>((done) => { resolve = done; });
    return { automatic, controller: new AbortController(), epoch: 0, resolve, promise };
  };
  const start = (current: Batch<T>) => {
    if (disposed || (current.automatic && !canRefresh())) { current.resolve(undefined); return; }
    active = current;
    current.epoch = epoch;
    void withDockReadDeadline(fetch, current.controller).then((value) => {
      finish(current, current.epoch === epoch && !disposed ? value : undefined);
    }, (error: unknown) => {
      if (!disposed && current.epoch === epoch && !(error instanceof DOMException && error.name === 'AbortError')) {
        onError(error instanceof Error ? error.message : 'Unable to refresh conversation');
      }
      finish(current, undefined);
    });
  };
  const finish = (current: Batch<T>, value: T | undefined) => {
    if (active === current) {
      active = null;
      const next = queued;
      queued = null;
      if (next) start(next);
    }
    current.resolve(value);
  };
  return {
    get pending() { return !!active || !!queued; },
    activate() { disposed = false; },
    read(automatic = false) {
      if (disposed || (automatic && !canRefresh())) return Promise.resolve(undefined);
      if (active) {
        if (!automatic) epoch += 1;
        queued ??= batch(automatic);
        queued.automatic &&= automatic;
        return queued.promise;
      }
      const current = batch(automatic);
      start(current);
      return current.promise;
    },
    pause() {
      if (queued?.automatic) { queued.resolve(undefined); queued = null; }
      if (active?.automatic) { epoch += 1; active.controller.abort(); }
    },
    dispose() {
      disposed = true;
      epoch += 1;
      queued?.resolve(undefined);
      queued = null;
      active?.controller.abort();
      active = null;
    },
  };
}
