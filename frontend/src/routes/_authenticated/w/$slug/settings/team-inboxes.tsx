import { createFileRoute } from '@tanstack/react-router';
import { TeamInboxesSettingsPage } from '@/pages/settings/TeamInboxesSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/team-inboxes')({
  component: TeamInboxesSettingsRoute,
});

function TeamInboxesSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <TeamInboxesSettingsPage />
    </SettingsRouteViewport>
  );
}
