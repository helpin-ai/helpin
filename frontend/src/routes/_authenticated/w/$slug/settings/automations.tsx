import { createFileRoute } from '@tanstack/react-router';
import { AutomationsSettingsPage } from '@/pages/settings/AutomationsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/automations')({
  component: AutomationsSettingsRoute,
});

function AutomationsSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <AutomationsSettingsPage />
    </SettingsRouteViewport>
  );
}
