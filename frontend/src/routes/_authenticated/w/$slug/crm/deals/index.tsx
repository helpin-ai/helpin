import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/deals/')({
  component: DealsRoute,
});

const DealsPage = lazyRouteComponent(() => import('@/pages/crm/Deals'), 'DealsPage');

function DealsRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <DealsPage />
    </div>
  );
}
