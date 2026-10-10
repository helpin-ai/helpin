import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { unwrap } from '@/lib/queryUtils';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import type { SupportMessage } from '@/lib/pmTypes';
import { Clock01Icon, Loading03Icon } from '@/lib/icons';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

export function PendingSendStatus({ message }: { message: SupportMessage }) {
  const client = useQueryClient();
  const draft = useSupportInboxStore(s => s.drafts[message.conversation_id]);
  const attachmentCount = useSupportInboxStore(s => s.draftAttachmentCounts[message.conversation_id] ?? 0);
  const hasDraft = !!draft?.trim() || attachmentCount > 0;
  const update = useMutation({
    mutationFn: (action: string) => api.post(`/support/inbox/conversations/${message.conversation_id}/translation/sends/${message.pending_send_id}?workspace_id=${encodeURIComponent(message.workspace_id)}`, { action }).then(unwrap),
    onSuccess: () => client.invalidateQueries({ queryKey: ['support', message.workspace_id, 'pending-sends', message.conversation_id] }),
  });
  const edit = () => {
    if (hasDraft) return;
    const detail = {
      conversationId: message.conversation_id,
      markdown: message.content,
      attachments: message.attachments || [],
      deliveryMode: message.pending_request?.delivery_mode,
      pendingSend: true,
      restored: false,
    };
    window.dispatchEvent(new CustomEvent('support:restore-draft', { detail }));
    if (detail.restored) update.mutate('dismiss');
  };
  const actionClass = 'underline-offset-2 hover:underline disabled:opacity-50';
  const pendingLabel = message.pending_send === 'translating' ? 'Translating…' : message.pending_send === 'queued' ? 'Queued' : 'Sending…';
  return <div className="mt-1 flex flex-wrap items-center justify-end gap-2 pr-9 text-xs text-muted-foreground" aria-live="polite">
    {message.pending_send !== 'failed' ? (
      <Tooltip>
        <TooltipTrigger asChild>
          <span role="status" aria-label={pendingLabel} tabIndex={0} className="inline-flex min-h-6 items-center rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <span aria-hidden="true">
              {message.pending_send === 'queued' ? <Clock01Icon className="h-3.5 w-3.5" /> : <Loading03Icon className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" />}
            </span>
          </span>
        </TooltipTrigger>
        <TooltipContent>{pendingLabel}</TooltipContent>
      </Tooltip>
    ) : <>
      <span className="text-destructive">Not sent</span>
      <button type="button" className={actionClass} disabled={update.isPending} onClick={() => update.mutate('retry')}>Retry</button>
      {message.pending_failure === 'translation' && <button type="button" className={actionClass} disabled={update.isPending} onClick={() => update.mutate('original')}>Send original</button>}
      <button
        type="button" className={actionClass}
        disabled={update.isPending || hasDraft}
        title={hasDraft ? 'Send or clear your current draft before editing this reply.' : undefined}
        onClick={edit}
      >Edit</button>
    </>}
    {update.isError && <span role="alert">Could not update. Try again.</span>}
  </div>;
}
