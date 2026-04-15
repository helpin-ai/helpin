import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { XIcon } from './icons';

interface SpecialNoticeBannerProps {
  text?: string;
  workspaceId?: string;
}

/**
 * Slim amber banner rendered above conversation surfaces when the backend
 * emits `availability.specialNoticeText`. Dismissal persists per session
 * (localStorage), keyed on workspace + stable hash of the text so a new
 * notice re-appears even if the last one was dismissed.
 */
export const SpecialNoticeBanner: FunctionComponent<SpecialNoticeBannerProps> = ({ text, workspaceId }) => {
  const [dismissed, setDismissed] = useState(true);
  const trimmed = (text ?? '').trim();

  useEffect(() => {
    if (!trimmed) {
      setDismissed(true);
      return;
    }
    const key = dismissalKey(workspaceId, trimmed);
    try {
      const stored = typeof window !== 'undefined' ? window.localStorage.getItem(key) : null;
      setDismissed(stored === '1');
    } catch {
      setDismissed(false);
    }
  }, [trimmed, workspaceId]);

  if (!trimmed || dismissed) return null;

  const handleDismiss = () => {
    const key = dismissalKey(workspaceId, trimmed);
    try {
      if (typeof window !== 'undefined') {
        window.localStorage.setItem(key, '1');
      }
    } catch {
      /* storage unavailable — dismissal still works for this session */
    }
    setDismissed(true);
  };

  return (
    <div className="helpin-special-notice" role="status" aria-live="polite">
      <svg
        className="helpin-special-notice-icon"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
        <line x1="12" y1="9" x2="12" y2="13" />
        <line x1="12" y1="17" x2="12.01" y2="17" />
      </svg>
      <span className="helpin-special-notice-text">{trimmed}</span>
      <button
        type="button"
        className="helpin-special-notice-dismiss"
        onClick={handleDismiss}
        aria-label="Dismiss notice"
      >
        <XIcon size={14} />
      </button>
    </div>
  );
};

// Small, stable hash keyed on text so dismissing notice A does not dismiss
// a later notice B with different content. Workspace scope prevents cross-
// tenant leaks when a single browser serves multiple widgets.
function dismissalKey(workspaceId: string | undefined, text: string): string {
  return `helpin:special-notice:${workspaceId ?? 'unknown'}:${stableHash(text)}`;
}

function stableHash(input: string): string {
  let h = 0;
  for (let i = 0; i < input.length; i += 1) {
    h = (h * 31 + input.charCodeAt(i)) | 0;
  }
  // Base-36 keeps the key short while still being deterministic.
  return Math.abs(h).toString(36);
}
