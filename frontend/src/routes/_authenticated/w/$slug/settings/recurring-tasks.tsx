import { createFileRoute } from '@tanstack/react-router';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { RecurringTasksSettingsPage } from '@/pages/settings/RecurringTasksSettingsPage';

type RecurringTasksSettingsSearch = {
  team?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/recurring-tasks')({
  component: RecurringTasksSettingsRoute,
  validateSearch: (search: Record<string, unknown>): RecurringTasksSettingsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
});

function RecurringTasksSettingsRoute() {
  const { team } = Route.useSearch();

  return (
    <SettingsRouteViewport>
      <RecurringTasksSettingsPage initialTeamId={team} />
    </SettingsRouteViewport>
  );
}
