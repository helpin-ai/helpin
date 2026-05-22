import { createFileRoute } from '@tanstack/react-router';
import { TasksPage } from '@/pages/pm/Tasks';
import type { BoardFilters } from '@/stores/pmBoardStore';

type TasksSearch = { team?: string; task?: string; run?: string; epic?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/tasks/')({
  component: TasksRoute,
  validateSearch: (search: Record<string, unknown>): TasksSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
    task: search.task != null ? String(search.task) : undefined,
    run: search.run != null ? String(search.run) : undefined,
    epic: search.epic != null ? String(search.epic) : undefined,
  }),
});

function TasksRoute() {
  const { team, epic } = Route.useSearch();
  const initialFilters: BoardFilters | undefined = epic ? { epic_id: epic } : undefined;

  return (
    <div className="h-full overflow-hidden">
      <TasksPage teamId={team} initialFilters={initialFilters} />
    </div>
  );
}
