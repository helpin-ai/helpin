import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { BotIcon, Loading01Icon, PauseIcon } from '@/lib/icons';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { useChatSettings } from '@/hooks/queries/useSupport';
import { supportService } from '@/lib/services/supportService';
import { unwrap } from '@/lib/queryUtils';
import type { SupportConversation } from '@/lib/pmTypes';

export function useSupportAIControl(conversation: SupportConversation) {
  const [confirmReturnFor, setConfirmReturnFor] = useState<string | null>(null);
  const queryClient = useQueryClient();
  const { data: access } = useWorkspaceAccess(conversation.workspace_id);
  const { has } = usePermissions(access);
  const { data: installation } = useChatSettings(conversation.workspace_id);
  const { data: members } = useWorkspaceMembers(conversation.workspace_id);
  const portal = (conversation.channel ?? conversation.source) === 'portal';
  const paused = Boolean(
    conversation.human_takeover ||
    conversation.assigned_user_id ||
    conversation.opened_by_user_id ||
    conversation.customer_requested_human_at ||
    conversation.ai_state === 'escalated' ||
    (portal && conversation.flow_state !== 'ai_handling' && !conversation.assigned_agent_id),
  );
  const portalMode = installation?.settings.portal_ai_mode ?? 'off';
  const portalAgent = installation?.settings.portal_ai_agent_id || installation?.settings.ai_agent_id;
  const enabled = portal ? Boolean(
    installation?.active && installation.settings.portal_enabled && portalAgent &&
    ['ai_first', 'internal_note'].includes(portalMode),
  ) : Boolean(
    installation?.active &&
    installation.settings.ai_enabled &&
    installation.settings.ai_agent_id &&
    ['ai_first', 'internal_note'].includes(
      installation.settings.ai_response_mode,
    ),
  );
  const channel =
    conversation.source === 'email' || conversation.channel === 'email'
      ? 'email'
      : 'chat';
  const channels = installation?.settings.ai_reply_channels ?? 'chat';
  const eligible =
    !conversation.anonymized_at &&
    !['resolved', 'spam'].includes(conversation.status) &&
    (portal ? Boolean(conversation.portal_visible) :
      ['widget', 'email'].includes(conversation.channel ?? conversation.source) &&
      (channels === 'both' || channels === channel));
  const pausedBy = members?.find(
    (member) => member.user_id === conversation.ai_paused_by_user_id,
  )?.full_name;
  const mutation = useMutation({
    mutationFn: ({
      action,
      confirmed,
    }: {
      action: 'pause' | 'return' | 'run_now';
      confirmed: boolean;
    }) =>
      supportService
        .changeConversationAIControl(
          conversation.workspace_id,
          conversation.id,
          {
            action,
            expected_version: conversation.ai_control_version ?? 0,
            confirm_human_request: confirmed,
          },
        )
        .then(unwrap),
    onSuccess: () => {
      setConfirmReturnFor(null);
    },
    onError: (error: Error) =>
      toast.error('Could not change AI control', {
        description: error.message,
      }),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ['support'] });
    },
  });
  if (
    !has('support.edit') ||
    conversation.anonymized_at ||
    ['resolved', 'spam'].includes(conversation.status) ||
    (!enabled && !conversation.ai_state && !conversation.ai_paused_at)
  )
    return { item: null, confirmation: null };
  const waiting =
    !paused &&
    conversation.ai_resumed_at &&
    (!conversation.last_customer_message_at ||
      new Date(conversation.last_customer_message_at) <=
        new Date(conversation.ai_resumed_at));
  const unavailable = paused && (!enabled || !eligible || (portal && portalMode !== 'ai_first'));
  const disabled = mutation.isPending || unavailable;
  const label = paused ? (portal ? 'Ask Echo to handle' : 'Resume AI') : 'Pause AI';
  const status = paused
    ? pausedBy
      ? `AI paused by ${pausedBy}.`
      : 'AI paused. Humans are handling this conversation.'
    : enabled
      ? 'AI handling.'
      : 'AI disabled.';
  const explanation = paused
    ? unavailable
      ? portal && portalMode === 'internal_note'
        ? 'Private suggestions are generated for new unassigned requests. Enable customer replies to ask Echo to take over this request.'
        : 'Enable AI for this channel to return the conversation.'
      : portal
        ? 'Echo will answer the latest unanswered customer message. This releases human assignment.'
        : 'AI will respond to the next customer message. Resuming releases human assignment.'
    : `${waiting ? 'Waiting for the next customer message. ' : ''}Stop AI replies and follow-ups.`;
  const Icon = mutation.isPending
    ? Loading01Icon
    : paused
      ? BotIcon
      : PauseIcon;
  const item = (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuItem
            aria-label={label}
            aria-busy={mutation.isPending}
            disabled={disabled}
            className="data-disabled:pointer-events-auto"
            onSelect={() => {
              if (disabled) return;
              if (paused && conversation.customer_requested_human_at)
                setConfirmReturnFor(conversation.id);
              else
                mutation.mutate({
                  action: paused ? (portal ? 'run_now' : 'return') : 'pause',
                  confirmed: false,
                });
            }}
          >
            <Icon
              aria-hidden="true"
              className={`size-3.5 shrink-0${mutation.isPending ? ' animate-spin' : ''}`}
            />
            <span>{mutation.isPending ? 'Updating…' : label}</span>
          </DropdownMenuItem>
        </TooltipTrigger>
        <TooltipContent side="left" sideOffset={8}>
          {status} {explanation}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
  const confirmation = (
    <AlertDialog
      open={confirmReturnFor === conversation.id && paused}
      onOpenChange={(open) =>
        setConfirmReturnFor(open ? conversation.id : null)
      }
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{portal ? 'Ask Echo to handle this request?' : 'Resume AI for this conversation?'}</AlertDialogTitle>
          <AlertDialogDescription>
            The customer requested a human. Resuming releases human assignment
            and allows AI to respond to {portal ? 'their latest unanswered message' : 'their next message'}. The request remains
            in the conversation history.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={mutation.isPending}>
            Keep with humans
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={mutation.isPending}
            onClick={(event) => {
              event.preventDefault();
              mutation.mutate({ action: portal ? 'run_now' : 'return', confirmed: true });
            }}
          >
            {portal ? 'Ask Echo to handle' : 'Resume AI'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
  return { item, confirmation };
}
