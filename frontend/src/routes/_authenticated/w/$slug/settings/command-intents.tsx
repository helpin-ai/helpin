import { createFileRoute } from '@tanstack/react-router';
import { CommandIntentsSettingsPage } from '@/pages/settings/CommandIntentsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/command-intents')({
  component: CommandIntentsSettingsRoute,
});

function CommandIntentsSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <CommandIntentsSettingsPage />
    </SettingsRouteViewport>
  );
}
