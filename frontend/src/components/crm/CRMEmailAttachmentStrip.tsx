import { AttachmentIcon, Cancel01Icon, Loading01Icon } from '@/lib/icons';
import type { PendingCRMEmailAttachment } from '@/hooks/useCRMEmailAttachments';

export function CRMEmailAttachmentStrip({ attachments, onRemove }: { attachments: PendingCRMEmailAttachment[]; onRemove: (id: string) => void }) {
  if (attachments.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1.5 px-4 pb-2">
      {attachments.map((attachment) => (
        <div key={attachment.id} className="flex min-w-0 items-center gap-1.5 rounded-md border border-border/60 bg-background/60 px-2 py-1 text-xs">
          {attachment.status === 'uploading' ? <Loading01Icon className="h-3.5 w-3.5 animate-spin text-muted-foreground" /> : <AttachmentIcon className="h-3.5 w-3.5 text-muted-foreground" />}
          <span className="max-w-44 truncate">{attachment.fileName}</span>
          {attachment.status === 'error' ? <span className="text-destructive">Failed</span> : attachment.status === 'uploading' ? <span className="text-muted-foreground">{attachment.progress}%</span> : null}
          <button type="button" aria-label={`Remove ${attachment.fileName}`} className="text-muted-foreground hover:text-foreground" onClick={() => onRemove(attachment.id)}><Cancel01Icon className="h-3 w-3" /></button>
        </div>
      ))}
    </div>
  );
}
