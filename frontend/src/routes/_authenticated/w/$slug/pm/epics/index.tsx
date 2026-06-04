import { createFileRoute } from '@tanstack/react-router';
import { EpicsPage } from '@/pages/pm/Epics';

type EpicsSearch = { team?: string; task?: string; run?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/epics/')({
  component: EpicsRoute,
  validateSearch: (search: Record<string, unknown>): EpicsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
    task: search.task != null ? String(search.task) : undefined,
    run: search.run != null ? String(search.run) : undefined,
  }),
});

function EpicsRoute() {
  const { team } = Route.useSearch();
  return (
    <div className="h-full overflow-hidden">
      <EpicsPage teamId={team} />
    </div>
  );
}
