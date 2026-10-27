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
  const assistant = data.intake?.assistant ?? [];
  if (!company && opened.length === 0 && assistant.length === 0) return null;

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
      {assistant.length > 0 ? (
        <details className="mt-2 text-xs">
          <summary className="cursor-pointer leading-5 text-muted-foreground hover:text-foreground">
            Asked the help assistant first ({assistant.filter((turn) => turn.role === 'user').length} {assistant.filter((turn) => turn.role === 'user').length === 1 ? 'question' : 'questions'})
          </summary>
          <ol className="mt-2 space-y-2">
            {assistant.map((turn, index) => (
              <li key={index} className="min-w-0">
                <p className="font-medium text-foreground">{turn.role === 'user' ? 'Customer' : 'Help assistant'}</p>
                <p className="whitespace-pre-wrap break-words text-muted-foreground">
                  {turn.status === 'failed' ? 'Could not answer.' : turn.content}
                </p>
                {turn.citations.length > 0 ? (
                  <p className="mt-0.5 text-muted-foreground">Sources: {turn.citations.map((citation) => citation.title).join(', ')}</p>
                ) : null}
              </li>
            ))}
          </ol>
        </details>
      ) : null}
    </section>
  );
}
