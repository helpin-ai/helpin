import { createFileRoute } from '@tanstack/react-router';
import { TasksPage } from '@/pages/pm/Stories';

type TasksSearch = { team?: string; task?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/tasks/')({
  component: TasksRoute,
  validateSearch: (search: Record<string, unknown>): TasksSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
    task: typeof search.task === 'string' ? search.task : undefined,
  }),
});

function TasksRoute() {
  const { team } = Route.useSearch();
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <TasksPage teamId={team} />
    </div>
  );
}
