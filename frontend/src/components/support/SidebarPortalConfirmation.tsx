import { Button } from '@/components/ui/button';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useResendPortalConfirmation } from '@/hooks/queries/useSupport';
import type { SupportConversation } from '@/lib/pmTypes';
import { isHiddenPortalIntake } from './portalIntake';

/**
 * SidebarPortalConfirmation lets a support admin send a fresh confirmation
 * link for one anonymous portal request, after approving its submitter.
 * Using the link publishes only this request to the submitter's portal.
 */
export function SidebarPortalConfirmation({ workspaceId, conversation }: {
  workspaceId: string;
  conversation: SupportConversation;
}) {
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const resend = useResendPortalConfirmation(workspaceId);

  if (!isHiddenPortalIntake(conversation) || !permissions.has('support.admin')) return null;

  return (
    <section aria-labelledby="portal-confirmation-heading" className="border-b border-border/60 px-4 py-3">
      <h3 id="portal-confirmation-heading" className="text-xs font-medium text-foreground">Customer portal</h3>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">
        Sent without sign-in and not yet in the customer's portal. After approving the customer's contact, send a confirmation so they can view this request.
      </p>
      <Button
        variant="outline"
        size="sm"
        className="mt-2 h-7 text-xs"
        disabled={resend.isPending}
        onClick={() => resend.mutate(conversation.id)}
      >
        {resend.isPending ? 'Sending…' : 'Send portal confirmation'}
      </Button>
    </section>
  );
}
