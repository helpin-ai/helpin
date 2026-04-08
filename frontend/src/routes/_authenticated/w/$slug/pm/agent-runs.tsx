import { createFileRoute, Navigate } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/agent-runs')({
  component: AgentRunsRoute,
});

function AgentRunsRoute() {
  const { slug } = Route.useParams();
  return <Navigate to="/w/$slug/automation/runs" params={{ slug }} replace />;
}
