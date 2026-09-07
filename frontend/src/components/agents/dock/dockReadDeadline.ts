/** Bound a browser read even if a transport ignores AbortSignal. */
export function withDockReadDeadline<T>(read: (signal: AbortSignal) => Promise<T>, controller: AbortController): Promise<T> {
  const { signal } = controller;
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => controller.abort(new Error('Dock request timed out')), 30_000);
    const cleanup = () => { clearTimeout(timer); signal.removeEventListener('abort', abort); };
    const abort = () => { cleanup(); reject(signal.reason ?? new DOMException('Read cancelled', 'AbortError')); };
    signal.addEventListener('abort', abort, { once: true });
    if (signal.aborted) { abort(); return; }
    Promise.resolve().then(() => {
      if (signal.aborted) throw signal.reason;
      return read(signal);
    }).then(
      (value) => { cleanup(); resolve(value); },
      (error: unknown) => { cleanup(); reject(error); },
    );
  });
}
