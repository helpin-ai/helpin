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
    [info.external_email ? 'Sent at' : 'Created', formatTimestamp(info.sent_at)],
    [info.external_email ? 'Sent by' : 'Sender', info.sender.name],
    ['Origin', info.origin],
  ];

  if (info.from?.includes('@')) {
    rows.push(['From', info.from]);
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
    rows.push(['Translated', info.translated]);
  }
  if (info.automated) {
    rows.push(['Automated', info.automated]);
  }

  return rows;
}

export function MessageInfoDialog({ workspaceId, conversationId, messageId, open, onOpenChange }: MessageInfoDialogProps) {
  const { data: info, isLoading } = useMessageInfo(workspaceId, conversationId, messageId, open);
  const rows = info ? buildMessageInfoRows(info) : [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Message info</DialogTitle>
          <DialogDescription>Delivery and author details for this support message.</DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <div className="py-6 text-sm text-muted-foreground">Loading...</div>
        ) : info ? (
          <div className="divide-y divide-border rounded-lg border">
            {rows.map(([label, value]) => (
              <div key={label} className="grid grid-cols-[8rem_1fr] items-center gap-3 px-3 py-2 text-sm">
                <div className="text-muted-foreground">{label}</div>
                <div className="flex min-w-0 items-center justify-between gap-2">
                  <span className="min-w-0 truncate">{formatValue(value)}</span>
                  {label === 'Identifier' && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
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
      </DialogContent>
    </Dialog>
  );
}
