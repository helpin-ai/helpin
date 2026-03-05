import { createFileRoute } from '@tanstack/react-router';
import { StoriesPage } from '@/pages/pm/Stories';

type StoriesSearch = { team?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/stories/')({
  component: StoriesRoute,
  validateSearch: (search: Record<string, unknown>): StoriesSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
});

function StoriesRoute() {
  const { team } = Route.useSearch();
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <StoriesPage teamId={team} />
    </div>
  );
}
