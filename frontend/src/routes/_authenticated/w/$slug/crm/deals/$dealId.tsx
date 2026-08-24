import { createFileRoute } from '@tanstack/react-router';
import { DealRouteFallbackBackground } from '@/components/crm/deal-detail/DealRouteFallbackBackground';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/deals/$dealId')({
  component: DealDetailRoute,
});

function DealDetailRoute() {
  return <DealRouteFallbackBackground />;
}
