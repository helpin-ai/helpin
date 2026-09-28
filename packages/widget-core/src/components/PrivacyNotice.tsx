import { useState } from 'preact/hooks';
import { DEFAULT_PRIVACY_NOTICE_TEXT } from '@helpin-ai/shared';
import { XIcon } from './icons';

interface PrivacyNoticeProps {
  policyUrl: string;
  text: string;
  workspaceId: string;
  conversationId?: string;
  onDismiss?: () => void;
}

export function PrivacyNotice({ policyUrl, text, workspaceId, conversationId, onDismiss }: PrivacyNoticeProps) {
  const storageKey = conversationId ? `helpin:privacy-dismissed:${workspaceId}:${conversationId}` : undefined;
  const [dismissed, setDismissed] = useState(() => {
    try { return storageKey ? localStorage.getItem(storageKey) === '1' : false; }
    catch { return false; }
  });
  if (dismissed) return null;

  return (
    <aside className="helpin-privacy-notice" aria-label="Chat privacy notice">
      <p>{text.trim() || DEFAULT_PRIVACY_NOTICE_TEXT}{' '}
        <a href={policyUrl} target="_blank" rel="noopener noreferrer">Privacy Policy</a>.
      </p>
      <button type="button" className="helpin-privacy-dismiss" aria-label="Dismiss privacy notice" onClick={() => {
        setDismissed(true);
        onDismiss?.();
        try { if (storageKey) localStorage.setItem(storageKey, '1'); }
        catch { /* Dismissal still works when browser storage is unavailable. */ }
      }}><XIcon size={14} /></button>
    </aside>
  );
}
