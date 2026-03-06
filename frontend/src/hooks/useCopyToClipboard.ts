import { useCallback, useRef, useState } from 'react';

/**
 * Hook for copying text to the clipboard with visual feedback state.
 * Uses navigator.clipboard with a textarea fallback for insecure contexts.
 */
export function useCopyToClipboard(resetMs = 2000) {
  const [copied, setCopied] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const copy = useCallback(
    (text: string) => {
      const onSuccess = () => {
        setCopied(true);
        if (timeoutRef.current) {
          clearTimeout(timeoutRef.current);
        }
        timeoutRef.current = setTimeout(() => setCopied(false), resetMs);
      };

      if (navigator.clipboard?.writeText) {
        navigator.clipboard.writeText(text).then(onSuccess, () => {
          fallbackCopy(text);
          onSuccess();
        });
      } else {
        fallbackCopy(text);
        onSuccess();
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
  document.execCommand('copy');
  document.body.removeChild(ta);
}
