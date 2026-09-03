import { useEffect, useState } from 'react';

/**
 * Returns `value` after it has been stable for `delayMs`.
 *
 * Use this instead of a hand-rolled `useEffect` + `setTimeout` in search
 * boxes; feed the debounced value into a query key so TanStack Query
 * handles the fetch, cancellation and caching.
 */
export function useDebounce<T>(value: T, delayMs = 300): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delayMs);
    return () => window.clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}
