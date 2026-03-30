import { createFileRoute } from '@tanstack/react-router';
import { ImportSettingsPage } from '@/pages/settings/ImportSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/import')({
  component: ImportSettingsRoute,
});

function ImportSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <ImportSettingsPage />
    </SettingsRouteViewport>
  );
}
