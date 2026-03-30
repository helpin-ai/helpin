import { createFileRoute } from '@tanstack/react-router';
import { CRMEmailAccountsSettingsPage } from '@/pages/settings/CRMEmailAccountsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/crm-email')({
  component: CRMEmailSettingsRoute,
});

function CRMEmailSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <CRMEmailAccountsSettingsPage />
    </SettingsRouteViewport>
  );
}
