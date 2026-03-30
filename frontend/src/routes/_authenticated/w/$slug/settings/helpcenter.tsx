import { createFileRoute } from '@tanstack/react-router';
import { HelpCenterSettingsPage } from '@/pages/settings/HelpCenterSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/helpcenter')({
  component: HelpCenterSettingsRoute,
});

function HelpCenterSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <HelpCenterSettingsPage />
    </SettingsRouteViewport>
  );
}
