import { createFileRoute } from '@tanstack/react-router';
import { AgentRunsPage } from '@/pages/automation/AgentRuns';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/runs')({
  component: AgentRunsRoute,
});

function AgentRunsRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <AgentRunsPage />
    </div>
  );
}
