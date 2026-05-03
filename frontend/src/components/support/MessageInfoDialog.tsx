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

export function MessageInfoDialog({ workspaceId, conversationId, messageId, open, onOpenChange }: MessageInfoDialogProps) {
  const { data: info, isLoading } = useMessageInfo(workspaceId, conversationId, messageId, open);
  const rows = info ? [
    ['Identifier', info.id],
    ['Sent on', formatTimestamp(info.sent_at)],
    ['Sent by', info.sender.name],
    ['From', info.from],
    ['Origin', info.origin],
    ['Type', info.type],
    ['Delivered', info.delivered ? `${info.delivered.channel} · ${formatTimestamp(info.delivered.delivered_at)}` : 'No'],
    ['Not delivered', info.not_delivered_reason ?? 'No'],
    ['Read', info.read_at ? formatTimestamp(info.read_at) : info.read],
    ['Edited', info.edited],
    ['Translated', info.translated],
    ['Automated', info.automated],
  ] as const : [];

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
