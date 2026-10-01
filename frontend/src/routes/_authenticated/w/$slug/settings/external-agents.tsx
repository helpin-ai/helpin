import { createFileRoute } from '@tanstack/react-router';
import { ExternalAgentsSettingsPage } from '@/pages/settings/ExternalAgentsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/external-agents')({
  component: ExternalAgentsSettingsRoute,
});

function ExternalAgentsSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <ExternalAgentsSettingsPage />
    </SettingsRouteViewport>
  );
}
