import { useLayoutEffect, useRef } from 'react';

/** Restore keyboard focus when a lazy cell replaces its Popover with a fresh trigger. */
export function useQuietDropdownFocusReturn(open: boolean, enabled = true) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const wasOpen = useRef(open);
  useLayoutEffect(() => {
    const closed = wasOpen.current && !open;
    wasOpen.current = open;
    if (!enabled || !closed) return;
    const frame = requestAnimationFrame(() => {
      // Respect focus placed by an outside click or a newly opened dialog.
      if (document.activeElement === document.body)
        triggerRef.current?.focus({ preventScroll: true });
    });
    return () => cancelAnimationFrame(frame);
  }, [open, enabled]);
  return triggerRef;
}
