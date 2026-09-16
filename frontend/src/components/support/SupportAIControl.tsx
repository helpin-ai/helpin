import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { useChatSettings } from '@/hooks/queries/useSupport';
import { supportService } from '@/lib/services/supportService';
import { unwrap } from '@/lib/queryUtils';
import type { SupportConversation } from '@/lib/pmTypes';

export function SupportAIControl({ conversation }: { conversation: SupportConversation }) {
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
  if (conversation.anonymized_at || ['resolved', 'spam'].includes(conversation.status) || (!enabled && !conversation.ai_state && !conversation.ai_paused_at)) return null;
  const waiting = !paused && conversation.ai_resumed_at && (!conversation.last_customer_message_at || new Date(conversation.last_customer_message_at) <= new Date(conversation.ai_resumed_at));
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2 text-xs">
      <div role="status" className="min-w-0 text-muted-foreground">
        <p className="font-medium text-foreground">{paused ? (pausedBy ? `AI paused by ${pausedBy}` : 'AI paused · Human handling') : enabled ? 'AI handling' : 'AI disabled'}</p>
        {paused && <p>AI will not reply or follow up until returned to AI.</p>}
        {waiting && <p>AI will respond to the next customer message.</p>}
      </div>
      {has('support.edit') && <Button size="sm" variant="outline" disabled={mutation.isPending || (paused && (!enabled || !eligible))} onClick={() => {
        if (paused && conversation.customer_requested_human_at) setConfirmReturn(true);
        else mutation.mutate({ action: paused ? 'return' : 'pause', confirmed: false });
      }}>{mutation.isPending ? 'Updating…' : paused ? 'Return to AI' : 'Pause AI'}</Button>}
      {paused && has('support.edit') && <p className="w-full text-muted-foreground">{enabled && eligible ? 'Returning releases human assignment. AI responds to the next customer message.' : 'Enable AI for this channel to return the conversation.'}</p>}
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
    </div>
  );
}
