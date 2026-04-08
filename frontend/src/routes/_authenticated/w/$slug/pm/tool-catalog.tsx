import { createFileRoute, Navigate } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/tool-catalog')({
  component: ToolCatalogRoute,
});

function ToolCatalogRoute() {
  const { slug } = Route.useParams();
  return <Navigate to="/w/$slug/automation/tools" params={{ slug }} replace />;
}
