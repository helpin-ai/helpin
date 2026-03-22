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

      if (fallbackCopy(text)) {
        onSuccess();
        return true;
      }

      const canUseAsyncClipboard =
        typeof window !== 'undefined' &&
        window.isSecureContext === true &&
        typeof navigator.clipboard?.writeText === 'function';

      if (!canUseAsyncClipboard) {
        onFailure();
        return false;
      }

      try {
        await navigator.clipboard.writeText(text);
        onSuccess();
        return true;
      } catch {
        onFailure();
        return false;
      }
    },
    [resetMs],
  );

  return { copied, copy };
}

function fallbackCopy(text: string) {
  const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const selection = document.getSelection();
  const ranges = selection
    ? Array.from({ length: selection.rangeCount }, (_, index) => selection.getRangeAt(index).cloneRange())
    : [];
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
    if (selection) {
      selection.removeAllRanges();
      for (const range of ranges) {
        selection.addRange(range);
      }
    }
    activeElement?.focus();
  }
}
