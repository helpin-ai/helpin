import { createFileRoute, Navigate } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/')({
  component: CRMIndexRoute,
});

function CRMIndexRoute() {
  const { slug } = Route.useParams();
  return <Navigate to="/w/$slug/crm/overview" params={{ slug }} />;
}
