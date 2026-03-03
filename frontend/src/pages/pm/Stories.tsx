import { KanbanBoard } from '@/components/pm/KanbanBoard';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export function StoriesPage() {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return <KanbanBoard workspaceId={workspace.id} />;
}
