import { Navigate, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/library')({
  component: AutomationLibraryRoute,
});

function AutomationLibraryRoute() {
  const { slug } = Route.useParams();
  return (
    <Navigate to="/w/$slug/automation/triggers" params={{ slug }} replace />
  );
}
