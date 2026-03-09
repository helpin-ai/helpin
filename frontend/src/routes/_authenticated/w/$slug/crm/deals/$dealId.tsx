import { createFileRoute } from '@tanstack/react-router';
import { DealDetailPage } from '@/pages/crm/DealDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/deals/$dealId')({
  component: DealDetailRoute,
});

function DealDetailRoute() {
  const { dealId } = Route.useParams();
  return (
    <div className="h-full overflow-auto pt-4 md:pt-6">
      <DealDetailPage dealId={dealId} />
    </div>
  );
}
