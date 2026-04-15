import { createFileRoute } from '@tanstack/react-router';
import { SprintsPage } from '@/pages/pm/Sprints';

type SprintsSearch = { team?: string; task?: string; run?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/sprints/')({
  component: SprintsRoute,
  validateSearch: (search: Record<string, unknown>): SprintsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
    task: search.task != null ? String(search.task) : undefined,
    run: search.run != null ? String(search.run) : undefined,
  }),
});

function SprintsRoute() {
  const { team } = Route.useSearch();
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <SprintsPage teamId={team} />
    </div>
  );
}
