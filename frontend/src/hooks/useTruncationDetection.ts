import { useCallback, useState } from 'react';

/**
 * Detects whether elements identified by ID have truncated text (scrollWidth > clientWidth).
 * Returns a ref callback to attach to spans and a lookup to check truncation state.
 */
export function useTruncationDetection() {
  const [truncatedIds, setTruncatedIds] = useState<Set<string>>(new Set());

  const checkRef = useCallback((id: string, el: HTMLSpanElement | null) => {
    if (!el) return;
    const isTruncated = el.scrollWidth > el.clientWidth;
    setTruncatedIds((prev) => {
      if (isTruncated === prev.has(id)) return prev;
      const next = new Set(prev);
      if (isTruncated) next.add(id); else next.delete(id);
      return next;
    });
  }, []);

  const isTruncated = useCallback((id: string) => truncatedIds.has(id), [truncatedIds]);

  return { checkRef, isTruncated };
}
