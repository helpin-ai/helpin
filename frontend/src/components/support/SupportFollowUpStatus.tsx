import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { QuietTextAction } from '@/components/design-system/quiet';
import { supportService } from '@/lib/services/supportService';
import { unwrap } from '@/lib/queryUtils';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import type { SupportConversation } from '@/lib/pmTypes';

const labels: Record<string, string> = {
  scheduled: 'Follow-up scheduled', assessing: 'Reviewing conversation for follow-up',
  waiting: 'Waiting for customer', resolved: 'Closed after no reply',
  skipped: 'Follow-up skipped', cancelled: 'Follow-up cancelled',
  failed: 'Follow-up stopped', handoff: 'Follow-up requires a teammate',
};

const failureReasons: Record<string, string> = {
  assessment_launch_timeout: 'The conversation review could not start in time.',
  assessment_did_not_complete: 'The conversation review could not be completed.',
  assessment_timeout: 'The conversation review took too long to complete.',
  assessment_expired: 'The conversation review expired before it could finish.',
  closing_notice_missing: 'The final reminder could not be prepared.',
  follow_up_delivery_failed: 'The follow-up email could not be delivered.',
  follow_up_delivery_unconfirmed: 'Delivery of the follow-up email could not be confirmed.',
};

export function SupportFollowUpStatus({ conversation }: { conversation: SupportConversation }) {
  const followUp = conversation.ai_follow_up;
  const queryClient = useQueryClient();
  const { data: access } = useWorkspaceAccess(conversation.workspace_id);
  const { has } = usePermissions(access);
  const cancel = useMutation({
    mutationFn: () => supportService.cancelConversationFollowUp(conversation.workspace_id, conversation.id).then(unwrap),
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['support'] }); },
    onError: () => toast.error('Unable to cancel follow-up. Please try again.'),
  });
  if (!followUp) {
    return conversation.ai_resolution_type === 'assumed' ? <p className="px-3 py-3 text-xs text-quiet-secondary">Closed after no reply</p> : null;
  }
  const waitingForSecond = followUp.status === 'waiting' && (followUp.sequence_version ?? 1) >= 2 && !followUp.second_sent_at;
  const deadline = waitingForSecond ? followUp.due_at : followUp.status === 'waiting' ? followUp.close_at : followUp.status === 'scheduled' ? followUp.due_at : undefined;
  return (
    <div className="border-b border-quiet-divider px-3 py-3 text-xs text-quiet-secondary" role="status">
      <p className="font-medium">{labels[followUp.status] ?? 'AI follow-up'}</p>
      {followUp.status === 'failed' && <p className="mt-1">{failureReasons[followUp.reason ?? ''] ?? 'A technical issue prevented this follow-up from continuing.'}</p>}
      {followUp.status === 'handoff' && followUp.reason && <p className="mt-1 break-words">{followUp.reason}</p>}
      {deadline && <p className="mt-1">{waitingForSecond ? 'Second follow-up scheduled for ' : followUp.status === 'waiting' ? 'Closes if no reply by ' : 'First follow-up scheduled for '}<time dateTime={deadline}>{new Date(deadline).toLocaleString()}</time></p>}
      {followUp.status === 'failed' && <p className="mt-1">Automatic follow-ups have stopped. This conversation will not close automatically. The customer was not notified about this failure.</p>}
      {followUp.status === 'handoff' && <p className="mt-1">A teammate should review this conversation. It will not close automatically.</p>}
      {has('support.edit') && ['scheduled', 'assessing', 'waiting'].includes(followUp.status) && <QuietTextAction className="mt-2" disabled={cancel.isPending} onClick={() => cancel.mutate()}>{cancel.isPending ? 'Cancelling…' : 'Cancel follow-up'}</QuietTextAction>}
      {followUp.status === 'waiting' && <p className="mt-1">A customer reply or teammate takeover cancels the remaining follow-ups and closure.</p>}
    </div>
  );
}
