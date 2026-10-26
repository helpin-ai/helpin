import { createFileRoute } from '@tanstack/react-router';
import { SupportWebhooksSettingsPage } from '@/pages/settings/SupportWebhooksSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/support-webhooks')({
  component: SupportWebhooksSettingsRoute,
});

function SupportWebhooksSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <SupportWebhooksSettingsPage />
    </SettingsRouteViewport>
  );
}
