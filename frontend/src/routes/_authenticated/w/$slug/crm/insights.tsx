import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/insights')({
  component: InsightsRoute,
});

const InsightsPage = lazyRouteComponent(() => import('@/pages/crm/Insights'), 'InsightsPage');

function InsightsRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <InsightsPage />
    </div>
  );
}
