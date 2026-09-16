import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { BotIcon, Loading01Icon, PauseIcon } from '@/lib/icons';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { useChatSettings } from '@/hooks/queries/useSupport';
import { supportService } from '@/lib/services/supportService';
import { unwrap } from '@/lib/queryUtils';
import type { SupportConversation } from '@/lib/pmTypes';

export function SupportAIControl({ conversation, compact = false }: { conversation: SupportConversation; compact?: boolean }) {
  const [confirmReturn, setConfirmReturn] = useState(false);
  const queryClient = useQueryClient();
  const { data: access } = useWorkspaceAccess(conversation.workspace_id);
  const { has } = usePermissions(access);
  const { data: installation } = useChatSettings(conversation.workspace_id);
  const { data: members } = useWorkspaceMembers(conversation.workspace_id);
  const paused = Boolean(conversation.human_takeover || conversation.assigned_user_id || conversation.opened_by_user_id || conversation.customer_requested_human_at || conversation.ai_state === 'escalated');
  const enabled = Boolean(installation?.active && installation.settings.ai_enabled && installation.settings.ai_agent_id && ['ai_first', 'internal_note'].includes(installation.settings.ai_response_mode));
  const channel = conversation.source === 'email' || conversation.channel === 'email' ? 'email' : 'chat';
  const channels = installation?.settings.ai_reply_channels ?? 'chat';
  const eligible = !conversation.anonymized_at && !['resolved', 'spam'].includes(conversation.status) && ['widget', 'email'].includes(conversation.channel ?? conversation.source) && (channels === 'both' || channels === channel);
  const pausedBy = members?.find(member => member.user_id === conversation.ai_paused_by_user_id)?.full_name;
  const mutation = useMutation({
    mutationFn: ({ action, confirmed }: { action: 'pause' | 'return'; confirmed: boolean }) => supportService.changeConversationAIControl(conversation.workspace_id, conversation.id, {
      action, expected_version: conversation.ai_control_version ?? 0, confirm_human_request: confirmed,
    }).then(unwrap),
    onSuccess: () => { setConfirmReturn(false); },
    onError: (error: Error) => toast.error('Could not change AI control', { description: error.message }),
    onSettled: () => { void queryClient.invalidateQueries({ queryKey: ['support'] }); },
  });
  if (!has('support.edit') || conversation.anonymized_at || ['resolved', 'spam'].includes(conversation.status) || (!enabled && !conversation.ai_state && !conversation.ai_paused_at)) return null;
  const waiting = !paused && conversation.ai_resumed_at && (!conversation.last_customer_message_at || new Date(conversation.last_customer_message_at) <= new Date(conversation.ai_resumed_at));
  const unavailable = paused && (!enabled || !eligible);
  const disabled = mutation.isPending || unavailable;
  const label = paused ? 'Return to AI' : 'Pause AI';
  const status = paused ? (pausedBy ? `AI paused by ${pausedBy}.` : 'AI paused. Humans are handling this conversation.') : enabled ? 'AI handling.' : 'AI disabled.';
  const explanation = paused
    ? unavailable ? 'Enable AI for this channel to return the conversation.' : 'Returning releases human assignment. AI responds to the next customer message.'
    : `${waiting ? 'Waiting for the next customer message. ' : ''}Stop AI replies and follow-ups.`;
  const Icon = mutation.isPending ? Loading01Icon : paused ? BotIcon : PauseIcon;
  return (
    <>
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              size={compact ? 'icon' : 'sm'}
              variant={compact ? 'ghost' : 'outline'}
              className={compact ? 'h-11 w-11 shrink-0 aria-disabled:opacity-50' : 'h-7 gap-1 text-xs aria-disabled:opacity-50'}
              aria-label={label}
              aria-busy={mutation.isPending}
              aria-disabled={disabled}
              onClick={() => {
                if (disabled) return;
                if (paused && conversation.customer_requested_human_at) setConfirmReturn(true);
                else mutation.mutate({ action: paused ? 'return' : 'pause', confirmed: false });
              }}
            >
              <Icon aria-hidden="true" className={`${compact ? 'h-4 w-4' : 'h-3.5 w-3.5'}${mutation.isPending ? ' animate-spin' : ''}`} />
              {!compact && (mutation.isPending ? 'Updating…' : label)}
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom" className="block max-w-64">
            <p className="font-medium">{status}</p>
            <p>{explanation}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <AlertDialog open={confirmReturn && paused} onOpenChange={setConfirmReturn}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Return this conversation to AI?</AlertDialogTitle>
            <AlertDialogDescription>The customer requested a human. Returning releases human assignment and allows AI to respond to their next message. The request remains in the conversation history.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={mutation.isPending}>Keep with humans</AlertDialogCancel>
            <AlertDialogAction disabled={mutation.isPending} onClick={event => { event.preventDefault(); mutation.mutate({ action: 'return', confirmed: true }); }}>Return to AI</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
