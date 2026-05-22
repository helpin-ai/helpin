import { createFileRoute } from '@tanstack/react-router';
import { RepositoriesSettingsPage } from '@/pages/settings/RepositoriesSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/repositories')({
  component: RepositoriesSettingsRoute,
});

function RepositoriesSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <RepositoriesSettingsPage />
    </SettingsRouteViewport>
  );
}
