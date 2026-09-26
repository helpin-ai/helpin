import { createFileRoute } from '@tanstack/react-router';
import { AutomationRouteViewport } from '@/components/automation/AutomationRouteViewport';
import { AgentsPage } from '@/pages/automation/Agents';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/agents')({
  validateSearch: (search: Record<string, unknown>) => ({
    agent_id: typeof search.agent_id === 'string' && search.agent_id.trim() ? search.agent_id : undefined,
  }),
  component: AgentsRoute,
});

function AgentsRoute() {
  const { agent_id } = Route.useSearch();
  return (
    <AutomationRouteViewport>
      <AgentsPage requestedAgentId={agent_id} />
    </AutomationRouteViewport>
  );
}
