import { createFileRoute } from '@tanstack/react-router';
import { SupportChannelsSettingsPage } from '@/pages/settings/SupportChannelsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/support-channels')({
  component: SupportChannelsSettingsRoute,
});

function SupportChannelsSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <SupportChannelsSettingsPage />
    </SettingsRouteViewport>
  );
}
