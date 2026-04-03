import { useCallback, useRef, useState } from 'react';

/**
 * Hook for copying text to the clipboard with visual feedback state.
 * Prefers navigator.clipboard API; falls back to execCommand for insecure contexts.
 */
export function useCopyToClipboard(resetMs = 2000) {
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleResult = useCallback(
    (success: boolean, err?: Error) => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      setCopied(success);
      setError(err ?? null);
      if (success) {
        timeoutRef.current = setTimeout(() => setCopied(false), resetMs);
      }
    },
    [resetMs],
  );

  const copy = useCallback(
    (text: string) => {
      // Prefer the modern async Clipboard API (works in secure contexts).
      if (
        typeof navigator !== 'undefined' &&
        typeof navigator.clipboard?.writeText === 'function'
      ) {
        navigator.clipboard
          .writeText(text)
          .then(() => handleResult(true))
          .catch((err) => {
            // Async API failed (permissions, etc.) — try fallback.
            if (fallbackCopy(text)) {
              handleResult(true);
            } else {
              handleResult(false, err instanceof Error ? err : new Error(String(err)));
            }
          });
        return;
      }

      // No async API available — use execCommand fallback.
      if (fallbackCopy(text)) {
        handleResult(true);
      } else {
        handleResult(false, new Error('Clipboard API not available'));
      }
    },
    [handleResult],
  );

  const reset = useCallback(() => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    setCopied(false);
    setError(null);
  }, []);

  return { copied, copy, error, reset };
}

/** Fallback for insecure contexts using the legacy execCommand API. */
function fallbackCopy(text: string): boolean {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  ta.setAttribute('aria-hidden', 'true');
  ta.style.position = 'fixed';
  ta.style.top = '0';
  ta.style.left = '-9999px';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  ta.setSelectionRange(0, ta.value.length);

  try {
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    document.body.removeChild(ta);
  }
}
