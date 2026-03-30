import { createFileRoute } from '@tanstack/react-router';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { StoryTemplatesSettingsPage } from '@/pages/settings/StoryTemplatesSettingsPage';

type StoryTemplatesSettingsSearch = {
  team?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/story-templates')({
  component: StoryTemplatesSettingsRoute,
  validateSearch: (search: Record<string, unknown>): StoryTemplatesSettingsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
});

function StoryTemplatesSettingsRoute() {
  const { team } = Route.useSearch();

  return (
    <SettingsRouteViewport>
      <StoryTemplatesSettingsPage initialTeamId={team} />
    </SettingsRouteViewport>
  );
}
