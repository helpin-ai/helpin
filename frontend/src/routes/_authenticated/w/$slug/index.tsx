import { createFileRoute, Navigate } from '@tanstack/react-router';
import { useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { workspaceHome } from '@/lib/workspaceSurface';

export const Route = createFileRoute('/_authenticated/w/$slug/')({ component: WorkspaceHome });
function WorkspaceHome() {
  const { slug } = Route.useParams();
  const workspace = useWorkspaceStore(s => s.currentWorkspace);
  const { data: access, isLoading } = useWorkspaceAccess(workspace?.id ?? '');
  if (isLoading) return null;
  const home = workspaceHome(slug, access?.modules ?? []);
  return home ? <Navigate to={home} replace /> : <p className="p-6">Ask your workspace administrator for module access.</p>;
}
