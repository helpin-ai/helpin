import { createFileRoute } from '@tanstack/react-router';
import { IterationDetailPage } from '@/pages/pm/IterationDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/iterations/$iterationId')({
  component: () => (
    <div className="h-full overflow-hidden">
      <IterationDetailPage />
    </div>
  ),
});
