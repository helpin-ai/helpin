import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/deals/$dealId')({
  component: DealDetailRoute,
});

const DealDetailPage = lazyRouteComponent(() => import('@/pages/crm/DealDetail'), 'DealDetailPage');

function DealDetailRoute() {
  const { dealId } = Route.useParams();
  return (
    <div className="h-full overflow-hidden">
      <DealDetailPage dealId={dealId} />
    </div>
  );
}
