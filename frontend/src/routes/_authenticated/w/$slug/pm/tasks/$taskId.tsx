import { createFileRoute } from '@tanstack/react-router';
import { TaskRouteFallbackBackground } from '@/components/pm/task-detail/TaskRouteFallbackBackground';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/tasks/$taskId')({
  validateSearch: (search: Record<string, unknown>) => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
  component: TaskRouteComponent,
});

function TaskRouteComponent() {
  const { team } = Route.useSearch();
  return <TaskRouteFallbackBackground teamId={team} />;
}
