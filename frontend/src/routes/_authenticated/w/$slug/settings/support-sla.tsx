import { createFileRoute } from '@tanstack/react-router';
import { SupportSLASettingsPage } from '@/pages/settings/SupportSLASettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/support-sla')({
  component: SupportSLASettingsRoute,
});

function SupportSLASettingsRoute() {
  return (
    <SettingsRouteViewport>
      <SupportSLASettingsPage />
    </SettingsRouteViewport>
  );
}
