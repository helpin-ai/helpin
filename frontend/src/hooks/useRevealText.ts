import { useEffect, useState } from 'react';

/**
 * useRevealText reveals append-only text a few characters at a time, like a
 * reply being typed, unless the reader prefers reduced motion. Text that does
 * not extend what is shown replaces it at once. It reports whether the reveal
 * is still running.
 */
export function useRevealText(text: string, animate: boolean): { visible: string; revealing: boolean } {
  const [motionAllowed, setMotionAllowed] = useState(() =>
    typeof window.matchMedia === 'function' && !window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  const [visible, setVisible] = useState(() => animate && motionAllowed ? '' : text);
  useEffect(() => {
    const media = window.matchMedia?.('(prefers-reduced-motion: reduce)');
    if (!media) return;
    const update = () => setMotionAllowed(!media.matches);
    media.addEventListener?.('change', update);
    return () => media.removeEventListener?.('change', update);
  }, []);
  if ((!animate || !motionAllowed || !text.startsWith(visible)) && visible !== text) setVisible(text);
  const revealing = animate && motionAllowed && text.startsWith(visible) && visible.length < text.length;
  useEffect(() => {
    if (!revealing) return;
    const timer = window.setTimeout(() => {
      let end = Math.min(text.length, visible.length + Math.max(5, Math.ceil(text.length / 60)));
      // Never cut a UTF-16 surrogate pair in half.
      if (end < text.length && /[\uD800-\uDBFF]/.test(text[end - 1])) end++;
      setVisible(text.slice(0, end));
    }, 32);
    return () => window.clearTimeout(timer);
  }, [text, revealing, visible]);
  return { visible: revealing ? visible : text, revealing };
}
