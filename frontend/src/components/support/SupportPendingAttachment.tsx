import { useState } from 'react';
import { toast } from 'sonner';
import { AttachmentIcon } from '@/lib/icons';
import type { SupportAttachmentPayload } from '@/lib/pmTypes';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

interface SupportPendingAttachmentProps {
  attachment: SupportAttachmentPayload;
  onRetry?: (id: string) => Promise<void>;
}

export function SupportPendingAttachment({ attachment, onRetry }: SupportPendingAttachmentProps) {
  const [retrying, setRetrying] = useState(false);
  const failed = attachment.processing_status === 'failed';

  async function retry() {
    if (!onRetry || retrying) return;
    setRetrying(true);
    try {
      await onRetry(attachment.id);
    } catch {
      toast.error('Could not retry attachment. Please try again.');
    } finally {
      setRetrying(false);
    }
  }

  return (
    <div className="flex min-w-0 items-center gap-2 rounded-md border border-border/70 px-3 py-2 text-xs">
      <AttachmentIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <span className="min-w-0 truncate font-medium" title={attachment.file_name}>{attachment.file_name}</span>
      <Tooltip>
        <TooltipTrigger asChild>
          <span tabIndex={0} role="status" className="ml-auto shrink-0 text-muted-foreground">
            {failed ? 'Unavailable' : 'Processing…'}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          {failed
            ? attachment.processing_error || 'The file could not be stored. The message is safe.'
            : 'The message has arrived. Its attachment is still being processed.'}
        </TooltipContent>
      </Tooltip>
      {failed && onRetry && (
        <button type="button" disabled={retrying} onClick={() => void retry()} className="font-medium text-primary hover:underline disabled:opacity-50">
          {retrying ? 'Retrying…' : 'Retry'}
        </button>
      )}
    </div>
  );
}
