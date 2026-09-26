const pending = new Map<string, Promise<unknown>>();
/** Preserve request order for a record without blocking unrelated records. */
export function serializeRecordMutation<T>(
  key: string,
  operation: () => Promise<T>,
): Promise<T> {
  const previous = pending.get(key) ?? Promise.resolve();
  const next = previous.catch(() => undefined).then(operation);
  pending.set(key, next);
  const release = () => {
    if (pending.get(key) === next) pending.delete(key);
  };
  void next.then(release, release);
  return next;
}
