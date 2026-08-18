import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/overview')({
  component: OverviewRoute,
});

const CRMOverviewPage = lazyRouteComponent(() => import('@/pages/crm/Overview'), 'CRMOverviewPage');

function OverviewRoute() {
  return <div className="h-full overflow-hidden"><CRMOverviewPage /></div>;
}
