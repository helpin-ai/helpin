import { createFileRoute } from '@tanstack/react-router';
import { CRMAutonomySettingsPage } from '@/pages/settings/CRMAutonomySettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/crm-autonomy')({
  component: CRMAutonomySettingsRoute,
});

function CRMAutonomySettingsRoute() {
  return (
    <SettingsRouteViewport>
      <CRMAutonomySettingsPage />
    </SettingsRouteViewport>
  );
}
