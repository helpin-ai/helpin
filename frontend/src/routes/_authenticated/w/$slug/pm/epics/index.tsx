import { createFileRoute } from '@tanstack/react-router';
import { EpicsPage } from '@/pages/pm/Epics';

type EpicsSearch = { team?: string; task?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/epics/')({
  component: EpicsRoute,
  validateSearch: (search: Record<string, unknown>): EpicsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
    task: search.task != null ? String(search.task) : undefined,
  }),
});

function EpicsRoute() {
  const { team } = Route.useSearch();
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <EpicsPage teamId={team} />
    </div>
  );
}
