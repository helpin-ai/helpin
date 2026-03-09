import { createFileRoute } from '@tanstack/react-router';
import { DealsPage } from '@/pages/crm/Deals';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/deals/')({
  component: DealsRoute,
});

function DealsRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <DealsPage />
    </div>
  );
}
