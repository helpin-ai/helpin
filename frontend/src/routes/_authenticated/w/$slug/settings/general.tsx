import { createFileRoute } from '@tanstack/react-router';
import { GeneralSettingsPage } from '@/pages/settings/GeneralSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/general')({
  component: GeneralSettingsRoute,
});

function GeneralSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <GeneralSettingsPage />
    </SettingsRouteViewport>
  );
}
