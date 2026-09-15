import { createFileRoute } from '@tanstack/react-router';
import { AISettingsPage } from '@/pages/settings/AISettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/ai-connections')({
  component: PersonalAIConnectionsRoute,
});

function PersonalAIConnectionsRoute() {
  return (
    <SettingsRouteViewport>
      <AISettingsPage scope="personal" />
    </SettingsRouteViewport>
  );
}
