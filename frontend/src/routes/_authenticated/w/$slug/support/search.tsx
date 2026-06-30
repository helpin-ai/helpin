import { createFileRoute } from '@tanstack/react-router';
import { SupportSearchPage, type SupportSearchRouteSearch } from '@/pages/pm/SupportSearch';

export const Route = createFileRoute('/_authenticated/w/$slug/support/search')({
  component: SupportSearchRoute,
  validateSearch: (search: Record<string, unknown>): SupportSearchRouteSearch => ({
    q: typeof search.q === 'string' ? search.q : undefined,
    sort: search.sort === 'newest' || search.sort === 'oldest' || search.sort === 'relevance' ? search.sort : undefined,
    assigned_to: typeof search.assigned_to === 'string' ? search.assigned_to : undefined,
    mailbox_ids: typeof search.mailbox_ids === 'string' ? search.mailbox_ids : undefined,
    tag_ids: typeof search.tag_ids === 'string' ? search.tag_ids : undefined,
    customer_email: typeof search.customer_email === 'string' ? search.customer_email : undefined,
    created_from: typeof search.created_from === 'string' ? search.created_from : undefined,
    created_to: typeof search.created_to === 'string' ? search.created_to : undefined,
    statuses: typeof search.statuses === 'string' ? search.statuses : undefined,
    priorities: typeof search.priorities === 'string' ? search.priorities : undefined,
    title: typeof search.title === 'string' ? search.title : undefined,
    ai: typeof search.ai === 'string' ? search.ai : undefined,
    page: typeof search.page === 'number' ? search.page : Number(search.page) || undefined,
    per_page: typeof search.per_page === 'number' ? search.per_page : Number(search.per_page) || undefined,
  }),
});

function SupportSearchRoute() {
  return (
    <div className="h-full overflow-hidden">
      <SupportSearchPage />
    </div>
  );
}
