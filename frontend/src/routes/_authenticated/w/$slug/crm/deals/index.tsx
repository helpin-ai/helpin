import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/deals/')({
  component: DealsRoute,
  validateSearch: (search: Record<string, unknown>): { filters?: string } => ({filters: typeof search.filters === 'string' ? search.filters : undefined}),
});

const DealsPage = lazyRouteComponent(() => import('@/pages/crm/Deals'), 'DealsPage');

function DealsRoute() {
  return (
    <div className="h-full overflow-hidden">
      <DealsPage />
    </div>
  );
}
