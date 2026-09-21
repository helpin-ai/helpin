import type { SupportMessage } from '@/lib/pmTypes';

// Per-message recipients, not the conversation's current outbound recipients.
export function SupportEmailRecipients({ message }: { message: SupportMessage }) {
  if (message.via_channel !== 'email' || !message.email_to) return null;
  const cc = message.email_cc ?? [];
  return <details className="mb-2 text-xs text-muted-foreground">
    <summary className="max-w-full cursor-pointer truncate hover:text-foreground">To: {message.email_to}{cc.length > 0 ? ` · CC (${cc.length})` : ''}</summary>
    <dl className="mt-1.5 grid grid-cols-[auto_1fr] gap-x-2 gap-y-1 break-all rounded-md bg-muted/40 p-2">
      {message.email_from && <><dt>From</dt><dd>{message.email_from}</dd></>}
      <dt>To</dt><dd>{message.email_to}</dd>
      {cc.length > 0 && <><dt>CC</dt><dd>{cc.join(', ')}</dd></>}
    </dl>
  </details>;
}
