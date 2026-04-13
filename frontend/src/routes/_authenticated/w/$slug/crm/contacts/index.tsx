import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export type ContactsSearch = {
  search?: string;
  stage?: string;
  status?: string;
  owner?: string;
  sort?: string;
  dir?: 'asc' | 'desc';
  group?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/crm/contacts/')({
  component: ContactsRoute,
  validateSearch: (search: Record<string, unknown>): ContactsSearch => ({
    search: typeof search.search === 'string' ? search.search : undefined,
    stage: typeof search.stage === 'string' ? search.stage : undefined,
    status: typeof search.status === 'string' ? search.status : undefined,
    owner: typeof search.owner === 'string' ? search.owner : undefined,
    sort: typeof search.sort === 'string' ? search.sort : undefined,
    dir: search.dir === 'asc' || search.dir === 'desc' ? search.dir : undefined,
    group: typeof search.group === 'string' ? search.group : undefined,
  }),
});

const ContactsPage = lazyRouteComponent(() => import('@/pages/crm/Contacts'), 'ContactsPage');

function ContactsRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <ContactsPage />
    </div>
  );
}
