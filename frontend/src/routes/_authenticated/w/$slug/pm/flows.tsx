import { createFileRoute } from '@tanstack/react-router';
import { FlowsPage } from '@/pages/pm/Flows';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/flows')({
  component: FlowsRoute,
});

function FlowsRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <FlowsPage />
    </div>
  );
}
