import { createFileRoute } from '@tanstack/react-router';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { TeamsSettingsPage } from '@/pages/settings/TeamsSettingsPage';

type TeamsSettingsSearch = {
  team?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/teams')({
  component: TeamsSettingsRoute,
  validateSearch: (search: Record<string, unknown>): TeamsSettingsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
});

function TeamsSettingsRoute() {
  const { team } = Route.useSearch();

  return (
    <SettingsRouteViewport>
      <TeamsSettingsPage initialTeamId={team} />
    </SettingsRouteViewport>
  );
}
