import { createFileRoute, Navigate } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/agents')({
  component: AgentsRoute,
});

function AgentsRoute() {
  const { slug } = Route.useParams();
  return <Navigate to="/w/$slug/automation/agents" params={{ slug }} search={{ agent_id: undefined }} replace />;
}
