import { createFileRoute } from '@tanstack/react-router';
import { TasksPage } from '@/pages/pm/Tasks';

type TasksSearch = { team?: string; task?: string; run?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/tasks/')({
  component: TasksRoute,
  validateSearch: (search: Record<string, unknown>): TasksSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
    task: search.task != null ? String(search.task) : undefined,
    run: search.run != null ? String(search.run) : undefined,
  }),
});

function TasksRoute() {
  const { team } = Route.useSearch();
  return (
    <div className="h-full overflow-hidden">
      <TasksPage teamId={team} />
    </div>
  );
}
