import { createFileRoute, Navigate } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/')({
  component: AutomationIndexRoute,
});

function AutomationIndexRoute() {
  const { slug } = Route.useParams();
  return <Navigate to="/w/$slug/automation/flows" params={{ slug }} replace />;
}
