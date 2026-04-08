import { createFileRoute, Navigate } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/library')({
  component: AutomationLibraryRoute,
});

function AutomationLibraryRoute() {
  const { slug } = Route.useParams();
  return <Navigate to="/w/$slug/automation/library" params={{ slug }} replace />;
}
