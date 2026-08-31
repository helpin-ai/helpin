import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';
import { normalizeContactDetailTab, type ContactDetailTab } from '@/lib/contactDetailTabs';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/contacts/$contactId')({
  validateSearch: (search: Record<string, unknown>): { tab?: ContactDetailTab; thread?: string } => ({
    tab: typeof search.tab === 'string' ? normalizeContactDetailTab(search.tab) : undefined,
    thread: typeof search.thread === 'string' && search.thread.trim() ? search.thread : undefined,
  }),
  component: ContactDetailRoute,
});

const ContactDetailPage = lazyRouteComponent(() => import('@/pages/crm/ContactDetail'), 'ContactDetailPage');

function ContactDetailRoute() {
  const { contactId } = Route.useParams();
  const { tab, thread } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <div className="h-full overflow-hidden">
      <ContactDetailPage
        contactId={contactId}
        activeTab={normalizeContactDetailTab(tab)}
        emailThreadId={thread}
        onTabChange={(nextTab) => void navigate({ search: { tab: normalizeContactDetailTab(nextTab), thread: nextTab === 'emails' ? thread : undefined } })}
        onEmailThreadChange={(nextThread) => void navigate({ search: { tab: 'emails', thread: nextThread } })}
      />
    </div>
  );
}
