import { Copy01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useMessageInfo } from '@/hooks/queries/useSupport';
import type { SupportMessageInfo } from '@/lib/pmTypes';
import { formatTimestamp } from './helpers';

interface MessageInfoDialogProps {
  workspaceId: string;
  conversationId: string;
  messageId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onViewEmail?: () => void;
}

function formatValue(value: string | boolean | null | undefined) {
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  return value?.trim() || 'No';
}

function formatEmailStatus(info: SupportMessageInfo) {
  if (!info.email_delivery_status_label) return '';
  const parts = [info.email_delivery_status_label];
  if (info.delivered?.delivered_at) {
    parts.push(formatTimestamp(info.delivered.delivered_at));
  }
  if (info.not_delivered_reason) {
    parts.push(info.not_delivered_reason);
  }
  return parts.join(' · ');
}

function formatEmailList(values: string[] | null | undefined) {
  return values?.map((value) => value.trim()).filter(Boolean).join(', ') ?? '';
}

export function buildMessageInfoRows(info: SupportMessageInfo) {
  const rows: Array<readonly [string, string | boolean | null | undefined]> = [
    ['Identifier', info.id],
    [info.email_delivery_status === 'received' ? 'Received at' : info.external_email ? 'Sent at' : 'Created', formatTimestamp(info.sent_at)],
    [info.external_email ? 'Sent by' : 'Sender', info.sender.name],
    ['Origin', info.origin],
  ];

  if (info.from?.includes('@')) {
    rows.push(['From', info.from]);
  }
  const address = (value: string) => (value.match(/<([^>]+)>/)?.[1] ?? value).trim().toLowerCase();
  if (info.reply_to?.includes('@') && address(info.reply_to) !== address(info.from)) {
    rows.push(['Reply-To', info.reply_to]);
  }
  if (info.to_email?.includes('@')) {
    rows.push(['To', info.to_email]);
  }
  const cc = formatEmailList(info.cc_emails);
  if (cc) {
    rows.push(['Cc', cc]);
  }
  const bcc = formatEmailList(info.bcc_emails);
  if (bcc) {
    rows.push(['Bcc', bcc]);
  }
  if (info.external_email && info.captured_via) {
    rows.push(['Added to Helpin', `Via the ${info.captured_via.toLowerCase()}`]);
  }
  if (info.type && info.type !== 'text') {
    rows.push(['Type', info.type]);
  }

  const emailStatus = formatEmailStatus(info);
  if (emailStatus) {
    rows.push(['Email status', emailStatus]);
  }
  if (info.read_status_label || info.read || info.read_at) {
    rows.push(['Read', info.read_status_label || (info.read_at ? formatTimestamp(info.read_at) : info.read)]);
  }
  if (info.edited) {
    rows.push(['Edited', info.edited]);
  }
  if (info.translated) {
    rows.push(['Translated', info.translation_language || info.translated]);
  }
  if (info.automated) {
    rows.push(['Automated', info.automated]);
  }

  return rows;
}

export function MessageInfoDialog({ workspaceId, conversationId, messageId, open, onOpenChange, onViewEmail }: MessageInfoDialogProps) {
  const { data: info, isLoading } = useMessageInfo(workspaceId, conversationId, messageId, open);
  const rows = info ? buildMessageInfoRows(info) : [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Message info</DialogTitle>
          <DialogDescription className="sr-only">Delivery and author details for this support message.</DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <div className="py-6 text-sm text-muted-foreground">Loading...</div>
        ) : info ? (
          <div className="divide-y divide-border rounded-lg border">
            {rows.map(([label, value]) => (
              <div key={label} className="grid grid-cols-[6.5rem_minmax(0,1fr)] items-start gap-3 sm:grid-cols-[8rem_minmax(0,1fr)] px-3 py-2 text-sm">
                <div className="text-muted-foreground">{label}</div>
                <div className="flex min-w-0 items-start justify-between gap-2">
                  <span className="min-w-0 whitespace-pre-wrap [overflow-wrap:anywhere]">{formatValue(value)}</span>
                  {label === 'Identifier' && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      className="shrink-0"
                      onClick={() => void navigator.clipboard?.writeText(info.id)}
                      aria-label="Copy message identifier"
                    >
                      <Copy01Icon className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="py-6 text-sm text-muted-foreground">Message details are unavailable.</div>
        )}
        {info?.email_direction && onViewEmail && (
          <Button variant="link" className="h-auto justify-self-start p-0" onClick={onViewEmail}>
            View original email
          </Button>
        )}
        {info?.original_text && <div className="space-y-2 text-sm"><p className="text-muted-foreground">Original reply</p><p dir="auto" className="max-h-48 overflow-auto whitespace-pre-wrap break-words">{info.original_text}</p></div>}
      </DialogContent>
    </Dialog>
  );
}
