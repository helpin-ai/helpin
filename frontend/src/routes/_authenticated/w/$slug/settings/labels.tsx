import { createFileRoute } from '@tanstack/react-router';
import { LabelsSettingsPage } from '@/pages/settings/LabelsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

type LabelsSettingsSearch = {
  team?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/labels')({
  component: LabelsSettingsRoute,
  validateSearch: (search: Record<string, unknown>): LabelsSettingsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
});

function LabelsSettingsRoute() {
  const { team } = Route.useSearch();

  return (
    <SettingsRouteViewport>
      <LabelsSettingsPage initialTeamId={team} />
    </SettingsRouteViewport>
  );
}
