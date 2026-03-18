import { useCallback, useRef, useState } from 'react';

/**
 * Hook for copying text to the clipboard with visual feedback state.
 * Uses navigator.clipboard with a textarea fallback for insecure contexts.
 */
export function useCopyToClipboard(resetMs = 2000) {
  const [copied, setCopied] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const copy = useCallback(
    async (text: string) => {
      const onSuccess = () => {
        setCopied(true);
        if (timeoutRef.current) {
          clearTimeout(timeoutRef.current);
        }
        timeoutRef.current = setTimeout(() => setCopied(false), resetMs);
      };

      const onFailure = () => {
        setCopied(false);
      };

      const canUseAsyncClipboard =
        typeof window !== 'undefined' &&
        window.isSecureContext === true &&
        typeof navigator.clipboard?.writeText === 'function';

      if (!canUseAsyncClipboard) {
        if (fallbackCopy(text)) {
          onSuccess();
          return true;
        }
        onFailure();
        return false;
      }

      try {
        await navigator.clipboard.writeText(text);
        onSuccess();
        return true;
      } catch {
        if (fallbackCopy(text)) {
          onSuccess();
          return true;
        }
        onFailure();
        return false;
      }
    },
    [resetMs],
  );

  return { copied, copy };
}

function fallbackCopy(text: string) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.position = 'fixed';
  ta.style.left = '-9999px';
  document.body.appendChild(ta);
  ta.focus();
  ta.select();

  try {
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    document.body.removeChild(ta);
  }
}
