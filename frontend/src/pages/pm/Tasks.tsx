import { KanbanBoard } from '@/components/pm/KanbanBoard';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { BoardFilters } from '@/stores/pmBoardStore';

interface TasksPageProps {
  teamId?: string;
  initialFilters?: BoardFilters;
}

export function TasksPage({ teamId, initialFilters }: TasksPageProps) {
  useTitle('Tasks');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return <KanbanBoard workspaceId={workspace.id} teamId={teamId} initialFilters={initialFilters} />;
}
