import { createFileRoute } from '@tanstack/react-router';
import { CRMFieldsSettingsPage } from '@/pages/settings/CRMFieldsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/crm-fields')({
  component: CRMFieldsSettingsPageRoute,
});

function CRMFieldsSettingsPageRoute() {
  return (
    <SettingsRouteViewport>
      <CRMFieldsSettingsPage />
    </SettingsRouteViewport>
  );
}
