import { createFileRoute, Navigate, Outlet } from '@tanstack/react-router';
import { useWorkspaceAccess } from '@/hooks/queries';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export const Route = createFileRoute('/_authenticated/w/$slug/support')({
  component: SupportModuleRoute,
});

function SupportModuleRoute() {
  const { slug } = Route.useParams();
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspace?.id ?? '');
  const { data: access } = useWorkspaceAccess(workspaceId);

  if (workspaceId && access && !access.modules.includes('support')) {
    return <Navigate to="/w/$slug/pm/my-work" params={{ slug }} replace />;
  }

  return <Outlet />;
}
