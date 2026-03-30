import { createFileRoute } from '@tanstack/react-router';
import { RedirectsSettingsPage } from '@/pages/settings/RedirectsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/redirects')({
  component: RedirectsSettingsRoute,
});

function RedirectsSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <RedirectsSettingsPage />
    </SettingsRouteViewport>
  );
}
