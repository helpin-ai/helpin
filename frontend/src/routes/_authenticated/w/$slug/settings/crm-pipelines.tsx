import { createFileRoute } from '@tanstack/react-router';
import { CRMPipelinesSettingsPage } from '@/pages/settings/CRMPipelinesSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/crm-pipelines')({
  component: CRMPipelinesSettingsRoute,
});

function CRMPipelinesSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <CRMPipelinesSettingsPage />
    </SettingsRouteViewport>
  );
}
