import { createFileRoute } from '@tanstack/react-router';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { TaskTemplatesSettingsPage } from '@/pages/settings/TaskTemplatesSettingsPage';

type TaskTemplatesSettingsSearch = {
  team?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/task-templates')({
  component: TaskTemplatesSettingsRoute,
  validateSearch: (search: Record<string, unknown>): TaskTemplatesSettingsSearch => ({
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
});

function TaskTemplatesSettingsRoute() {
  const { team } = Route.useSearch();

  return (
    <SettingsRouteViewport>
      <TaskTemplatesSettingsPage initialTeamId={team} />
    </SettingsRouteViewport>
  );
}
