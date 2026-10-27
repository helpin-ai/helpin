import { useConversationPortalContext } from '@/hooks/queries/useSupport';
import type { SupportConversation } from '@/lib/pmTypes';
import { useWorkspaceStore } from '@/stores/workspaceStore';

/**
 * SidebarPortalContext tells teammates, before they reply, what the customer
 * portal adds to a conversation: colleagues at the customer's company who can
 * read it, and help center articles the customer opened before writing in.
 */
export function SidebarPortalContext({ workspaceId, conversation }: {
  workspaceId: string;
  conversation: SupportConversation;
}) {
  const portal = (conversation.channel ?? conversation.source) === 'portal';
  // Only portal requests have an intake; only company-linked ones an audience.
  const relevant = portal || !!conversation.crm_company_id;
  const { data } = useConversationPortalContext(workspaceId, conversation.id, relevant);
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  if (!relevant || !data) return null;
  const company = data.company && data.company.members > 0 ? data.company : null;
  const opened = data.intake?.opened_articles ?? [];
  if (!company && opened.length === 0) return null;

  const people = company?.members === 1 ? '1 colleague' : `${company?.members} colleagues`;
  return (
    <section aria-labelledby="portal-context-heading" className="border-b border-border/60 px-4 py-3">
      <h3 id="portal-context-heading" className="text-xs font-medium text-foreground">Customer portal</h3>
      {company ? (
        <p className="mt-1 text-xs leading-5 text-muted-foreground">
          Besides the requester, {people} at {company.company_name} can read and reply to this conversation in the portal.
        </p>
      ) : null}
      {opened.length > 0 ? (
        <div className="mt-2">
          <p className="text-xs leading-5 text-muted-foreground">Before writing in, the customer opened:</p>
          <ul className="mt-1 space-y-1">
            {opened.map((article) => (
              <li key={article.id} className="min-w-0">
                {/* Articles for signed-in customers have no public address; open them in Docs. */}
                <a
                  href={article.url ?? `/w/${workspaceSlug}/docs/documents/${article.id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="block truncate text-xs text-foreground underline-offset-2 hover:underline"
                  title={article.portal_article_id ? `${article.title} (signed-in customers)` : article.title}
                >
                  {article.title}
                </a>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}
