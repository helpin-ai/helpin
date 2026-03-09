import { createFileRoute } from '@tanstack/react-router';
import { InsightsPage } from '@/pages/crm/Insights';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/insights')({
  component: InsightsRoute,
});

function InsightsRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <InsightsPage />
    </div>
  );
}
