import { KanbanBoard } from '@/components/pm/KanbanBoard';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';

interface StoriesPageProps {
  teamId?: string;
}

export function StoriesPage({ teamId }: StoriesPageProps) {
  useTitle('Stories');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return <KanbanBoard workspaceId={workspace.id} teamId={teamId} />;
}
