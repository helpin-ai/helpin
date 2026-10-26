import { useConversationPortalAudience } from '@/hooks/queries/useSupport';
import type { SupportConversation } from '@/lib/pmTypes';

/**
 * SidebarPortalAudience tells teammates, before they reply, when people at
 * the customer's company can read this conversation in the customer portal.
 */
export function SidebarPortalAudience({ workspaceId, conversation }: {
  workspaceId: string;
  conversation: SupportConversation;
}) {
  // Only conversations linked to a company can be shared with its members.
  const audience = useConversationPortalAudience(workspaceId, conversation.id, !!conversation.crm_company_id);
  const company = audience.data;
  if (!conversation.crm_company_id || !company || company.members === 0) return null;

  const people = company.members === 1 ? '1 colleague' : `${company.members} colleagues`;
  return (
    <section aria-labelledby="portal-audience-heading" className="border-b border-border/60 px-4 py-3">
      <h3 id="portal-audience-heading" className="text-xs font-medium text-foreground">Customer portal</h3>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">
        Besides the requester, {people} at {company.company_name} can read and reply to this conversation in the portal.
      </p>
    </section>
  );
}
