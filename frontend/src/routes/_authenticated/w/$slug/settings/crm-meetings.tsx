import { createFileRoute } from '@tanstack/react-router';
import { CRMMeetingSettingsPage } from '@/pages/settings/CRMMeetingSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/crm-meetings')({
  component: CRMMeetingSettingsRoute,
});

function CRMMeetingSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <CRMMeetingSettingsPage />
    </SettingsRouteViewport>
  );
}
