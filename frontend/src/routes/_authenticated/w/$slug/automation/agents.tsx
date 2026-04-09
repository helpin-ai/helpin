import { createFileRoute } from '@tanstack/react-router';
import { AgentsPage } from '@/pages/automation/Agents';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/agents')({
  component: AgentsRoute,
});

function AgentsRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <AgentsPage />
    </div>
  );
}
