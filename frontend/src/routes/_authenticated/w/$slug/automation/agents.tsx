import { createFileRoute } from '@tanstack/react-router';
import { AutomationRouteViewport } from '@/components/automation/AutomationRouteViewport';
import { AgentsPage } from '@/pages/automation/Agents';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/agents')({
  component: AgentsRoute,
});

function AgentsRoute() {
  return (
    <AutomationRouteViewport>
      <AgentsPage />
    </AutomationRouteViewport>
  );
}
