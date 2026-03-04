import { createFileRoute } from '@tanstack/react-router';
import { IterationsPage } from '@/pages/pm/Iterations';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/iterations/')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <IterationsPage />
    </div>
  ),
});
