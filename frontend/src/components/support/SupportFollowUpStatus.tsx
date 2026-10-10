import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { QuietTextAction } from '@/components/design-system/quiet';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  AlertCircleIcon,
  Clock01Icon,
  InformationCircleIcon,
  SparklesIcon,
  StopIcon,
} from '@/lib/icons';
import { supportService } from '@/lib/services/supportService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrapRequired } from '@/lib/queryUtils';
import { cn } from '@/lib/utils';
import type { SupportConversation, SupportMessage } from '@/lib/pmTypes';
import { formatTimestamp } from './helpers';
import { formatFollowUpTime, getSupportFollowUpStage } from './supportFollowUpState';

const failureReasons: Record<string, string> = {
  assessment_launch_timeout: 'The conversation review could not start in time.',
  assessment_did_not_complete: 'The conversation review could not be completed.',
  assessment_timeout: 'The conversation review took too long to complete.',
  assessment_expired: 'The conversation review expired before it could finish.',
  closing_notice_missing: 'The final reminder could not be prepared.',
  follow_up_delivery_failed: 'The follow-up email could not be delivered.',
  follow_up_delivery_unconfirmed: 'Delivery of the follow-up email could not be confirmed.',
};

interface Props {
  conversation: SupportConversation;
  canEdit?: boolean;
  stoppedByName?: string;
  latestPublicMessage?: Pick<SupportMessage, 'id' | 'created_at'>;
}

export function SupportFollowUpStatus({
  conversation,
  canEdit = false,
  stoppedByName,
  latestPublicMessage,
}: Props) {
  const queryClient = useQueryClient();
  const [now, setNow] = useState(() => new Date());
  const [helpOpen, setHelpOpen] = useState(false);
  const followUp = conversation.ai_follow_up;
  const stage = getSupportFollowUpStage(conversation, latestPublicMessage);
  const followUpId = followUp?.id;
  useEffect(() => {
    if (!followUpId) return;
    const timer = window.setInterval(() => setNow(new Date()), 60_000);
    return () => window.clearInterval(timer);
  }, [followUpId]);
  const cancel = useMutation({
    mutationFn: async (target: {
      workspaceId: string;
      conversationId: string;
      followUpId: string;
      closing: boolean;
    }) =>
      unwrapRequired(
        await supportService.cancelConversationFollowUp(
          target.workspaceId,
          target.conversationId,
          target.followUpId,
        ),
        'Stop follow-up',
      ),
    onSuccess: (updated, target) => {
      queryClient.setQueryData(
        queryKeys.support.conversation(target.workspaceId, target.conversationId),
        updated,
      );
      void queryClient.invalidateQueries({ queryKey: ['support', target.workspaceId] });
      toast.success(target.closing ? 'Automatic closure stopped' : 'AI follow-ups stopped');
    },
    onError: (_error, target) => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.support.conversation(target.workspaceId, target.conversationId),
      });
      toast.error('Unable to stop this follow-up. Refresh the conversation and try again.');
    },
  });
  if (!stage || !followUp) return null;

  const closing = stage.kind === 'closing';
  const stoppedClosing =
    stage.kind === 'stopped' &&
    !!followUp.sent_at &&
    ((followUp.sequence_version ?? 1) < 2 || !!followUp.second_sent_at);
  const titles = {
    scheduled: 'AI follow-up scheduled',
    reviewing: 'AI is checking whether to follow up',
    final: 'Final AI follow-up scheduled',
    closing: 'Conversation will close if there’s no reply',
    stopped: stoppedClosing ? 'Automatic closure stopped' : 'AI follow-ups stopped',
    failed: 'AI follow-up couldn’t be completed',
  };
  const canStop = canEdit && ['scheduled', 'reviewing', 'final', 'closing'].includes(stage.kind);
  const Icon =
    stage.kind === 'failed'
      ? AlertCircleIcon
      : stage.kind === 'stopped'
        ? StopIcon
        : stage.kind === 'reviewing'
          ? SparklesIcon
          : Clock01Icon;
  let detail =
    'Visible only to your team. AI reviews the conversation before following up. A customer reply or teammate takeover cancels this sequence. Stopping cancels its remaining reminders and automatic closure without changing the assignee.';
  if (closing)
    detail =
      'Visible only to your team. Stops automatic closure for this follow-up sequence without changing the assignee. A customer reply or teammate takeover also cancels it.';
  if (stage.kind === 'stopped')
    detail = `Stopped by ${stoppedByName || 'a teammate'} on ${formatTimestamp(followUp.updated_at, now)}. Applies to this sequence; the assignee is unchanged.`;
  if (stage.kind === 'failed')
    detail = `${failureReasons[followUp.reason ?? ''] ?? 'A technical issue prevented this follow-up from continuing.'} This sequence has stopped and will not close automatically. The customer was not notified about this failure.`;

  return (
    <div
      data-support-follow-up={stage.kind}
      role="status"
      className={cn(
        'mx-0 my-5 grid grid-cols-[1rem_minmax(0,1fr)_auto] items-start gap-x-2.5 border-y border-quiet-divider py-3 sm:mx-8',
        stage.kind === 'stopped' && 'border-b-0',
      )}
    >
      <Icon
        className={cn(
          'mt-0.5 h-4 w-4 text-quiet-text-tertiary',
          stage.kind === 'failed' && 'text-amber-700 dark:text-amber-300',
        )}
      />
      <div className="min-w-0">
        <div
          className={cn(
            'flex items-center gap-1.5 text-xs font-medium text-quiet-text-primary',
            stage.kind === 'stopped' && 'font-normal text-quiet-text-secondary',
          )}
        >
          <span>{titles[stage.kind]}</span>
          <TooltipProvider>
            <Tooltip open={helpOpen} onOpenChange={setHelpOpen}>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  aria-label="About this AI follow-up"
                  className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded text-quiet-text-tertiary hover:text-quiet-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                  onClick={(event) => {
                    event.preventDefault();
                    setHelpOpen(true);
                  }}
                >
                  <InformationCircleIcon className="h-3 w-3" />
                </button>
              </TooltipTrigger>
              <TooltipContent side="top">
                <p>{detail}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
        <p className="mt-0.5 text-[11px] leading-relaxed text-quiet-text-secondary">
          {stage.deadline ? (
            <>
              <time dateTime={stage.deadline} title={formatTimestamp(stage.deadline, now)}>
                {formatFollowUpTime(stage.deadline, now)}
              </time>
              {!closing && ' · if the customer hasn’t replied'}
            </>
          ) : stage.kind === 'reviewing' ? (
            'Reviewing the conversation before sending a message.'
          ) : stage.kind === 'failed' ? (
            'A teammate should review this conversation.'
          ) : stoppedClosing ? (
            'This sequence will no longer close the conversation.'
          ) : (
            'Remaining reminders and automatic closure cancelled.'
          )}
        </p>
      </div>
      {canStop && (
        <QuietTextAction
          className="self-center whitespace-nowrap text-[11px] focus-visible:underline focus-visible:underline-offset-4"
          disabled={cancel.isPending}
          onClick={() =>
            cancel.mutate({
              workspaceId: conversation.workspace_id,
              conversationId: conversation.id,
              followUpId: followUp.id,
              closing,
            })
          }
        >
          {cancel.isPending ? 'Stopping…' : closing ? 'Stop auto-close' : 'Stop follow-ups'}
        </QuietTextAction>
      )}
    </div>
  );
}
