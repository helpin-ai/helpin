import { createFileRoute } from '@tanstack/react-router';
import { MembersSettingsPage } from '@/pages/settings/MembersSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/members')({
  component: MembersSettingsRoute,
});

function MembersSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <MembersSettingsPage />
    </SettingsRouteViewport>
  );
}
